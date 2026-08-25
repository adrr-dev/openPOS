package repo

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

type ReportRepo struct {
	pool *pgxpool.Pool
}

func NewReportRepo(pool *pgxpool.Pool) *ReportRepo { return &ReportRepo{pool: pool} }

// periodStarts mengembalikan batas [mulai, selesai) absolut untuk periode
// dalam zona waktu toko. zero time = tanpa batas ("all").
func periodStarts(nowUTC time.Time, loc *time.Location, period string) (time.Time, time.Time, error) {
	local := nowUTC.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch period {
	case "today":
		return today, today.AddDate(0, 0, 1), nil
	case "yesterday":
		return today.AddDate(0, 0, -1), today, nil
	case "week":
		offset := (int(local.Weekday()) + 6) % 7 // Senin = awal minggu
		start := today.AddDate(0, 0, -offset)
		return start, today.AddDate(0, 0, 1), nil
	case "month":
		start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
		return start, start.AddDate(0, 1, 0), nil
	case "", "all":
		return time.Time{}, time.Time{}, nil
	default:
		return time.Time{}, time.Time{}, ErrNotFound
	}
}

// Dashboard menyusun ringkasan role-aware (FR-DASH-001..004).
func (r *ReportRepo) Dashboard(ctx context.Context, storeID, cashierID string, cashierView bool, tz string) (any, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now()
	todayLocal := now.In(loc)
	dayStart := time.Date(todayLocal.Year(), todayLocal.Month(), todayLocal.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1)

	outerScope := ` TRUE `
	innerScope := ` TRUE `
	args := []any{storeID}
	if cashierView {
		outerScope = ` t.cashier_id = $2 `
		innerScope = ` tt.cashier_id = $2 `
		args = append(args, cashierID)
	}
	startIdx := len(args) + 1
	endIdx := len(args) + 2
	args = append(args, dayStart.UTC(), dayEnd.UTC())

	var omzet, cnt, itemsSold int64
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.total),0), COUNT(*),
			COALESCE((SELECT SUM(ti.qty) FROM transaction_items ti
				JOIN transactions tt ON tt.id = ti.trx_id
				WHERE tt.store_id = $1 AND tt.status='completed' AND `+innerScope+`
					AND tt.created_at >= $`+strconv.Itoa(startIdx)+`
					AND tt.created_at < $`+strconv.Itoa(endIdx)+`), 0)
		FROM transactions t
		WHERE t.store_id = $1 AND t.status='completed' AND `+outerScope+`
			AND t.created_at >= $`+strconv.Itoa(startIdx)+`
			AND t.created_at < $`+strconv.Itoa(endIdx), args...,
	).Scan(&omzet, &cnt, &itemsSold)
	if err != nil {
		return nil, err
	}

	lowStock := int64(0)
	if !cashierView {
		if err := r.pool.QueryRow(ctx,
			`SELECT count(*) FROM products WHERE store_id=$1 AND active AND stock <= 5`, storeID,
		).Scan(&lowStock); err != nil {
			return nil, err
		}
	}

	recent, err := r.recentTrx(ctx, storeID, cashierID, cashierView, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	todayDTO := model.DashboardToday{Omzet: omzet, TrxCount: cnt, ItemsSold: itemsSold, LowStock: lowStock}

	if cashierView {
		return &model.DashboardCashier{Role: "cashier", Today: todayDTO, Recent: recent}, nil
	}

	// grafik 7 hari terakhir (zona waktu toko), hari kosong = 0
	weekStart := dayStart.AddDate(0, 0, -6)
	rows7, err := r.pool.Query(ctx, `
		SELECT (t.created_at AT TIME ZONE $2)::date AS d, COALESCE(SUM(t.total),0)
		FROM transactions t
		WHERE t.store_id=$1 AND t.status='completed'
			AND (t.created_at AT TIME ZONE $2)::date >= $3::date
			AND (t.created_at AT TIME ZONE $2)::date <= $4::date
		GROUP BY d ORDER BY d
	`, storeID, tz, weekStart.Format("2006-01-02"), todayLocal.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	byDate := map[string]int64{}
	for rows7.Next() {
		var d time.Time
		var v int64
		if err := rows7.Scan(&d, &v); err != nil {
			rows7.Close()
			return nil, err
		}
		byDate[d.Format("2006-01-02")] = v
	}
	rows7.Close()
	if err := rows7.Err(); err != nil {
		return nil, err
	}
	sales7 := make([]model.DayPoint, 0, 7)
	for i := 0; i < 7; i++ {
		d := weekStart.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		sales7 = append(sales7, model.DayPoint{Date: key, Omzet: byDate[key]})
	}

	methods := []model.MethodPoint{}
	mrows, err := r.pool.Query(ctx, `
		SELECT t.method, COALESCE(SUM(t.total),0) FROM transactions t
		WHERE t.store_id=$1 AND t.status='completed'
			AND t.created_at >= $2 AND t.created_at < $3
		GROUP BY t.method ORDER BY 2 DESC
	`, storeID, dayStart.UTC(), dayEnd.UTC())
	if err != nil {
		return nil, err
	}
	for mrows.Next() {
		var mp model.MethodPoint
		if err := mrows.Scan(&mp.Method, &mp.Total); err != nil {
			mrows.Close()
			return nil, err
		}
		methods = append(methods, mp)
	}
	mrows.Close()

	top := []model.TopProduct{}
	prows, err := r.pool.Query(ctx, `
		SELECT ti.product_id, ti.name, SUM(ti.qty) qty, SUM(ti.qty * ti.price) rev
		FROM transaction_items ti JOIN transactions t ON t.id = ti.trx_id
		WHERE t.store_id=$1 AND t.status='completed'
			AND t.created_at >= $2 AND t.created_at < $3
		GROUP BY 1,2 ORDER BY qty DESC LIMIT 5
	`, storeID, dayStart.UTC(), dayEnd.UTC())
	if err != nil {
		return nil, err
	}
	for prows.Next() {
		var tp model.TopProduct
		if err := prows.Scan(&tp.ProductID, &tp.Name, &tp.Qty, &tp.Revenue); err != nil {
			prows.Close()
			return nil, err
		}
		top = append(top, tp)
	}
	prows.Close()

	return &model.DashboardAdmin{
		Role: "admin", Today: todayDTO,
		Sales7: sales7, Methods: methods, TopProducts: top, Recent: recent,
	}, nil
}

func (r *ReportRepo) recentTrx(ctx context.Context, storeID, cashierID string, cashierView bool, dayStart, dayEnd time.Time) ([]model.TrxBrief, error) {
	q := `SELECT t.id, t.cashier_name, t.total, t.status, t.created_at FROM transactions t
		WHERE t.store_id=$1 AND t.status='completed' AND t.created_at >= $2 AND t.created_at < $3`
	args := []any{storeID, dayStart.UTC(), dayEnd.UTC()}
	if cashierView {
		args = append(args, cashierID)
		q += ` AND t.cashier_id = $4`
	}
	q += ` ORDER BY t.created_at DESC LIMIT 5`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TrxBrief{}
	for rows.Next() {
		var b model.TrxBrief
		if err := rows.Scan(&b.ID, &b.CashierName, &b.Total, &b.Status, &b.Time); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Report menyusun seluruh isi laporan untuk satu periode. HPP memakai snapshot
// buy_price pada transaction_items sehingga konsisten dengan histori (EC-008).
func (r *ReportRepo) Report(ctx context.Context, storeID, period, tz string) (*model.ReportBundle, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	start, end, err := periodStarts(time.Now().UTC(), loc, period)
	if err != nil {
		return nil, err
	}

	conds := []string{"t.store_id = $1", "t.status = 'completed'"}
	args := []any{storeID}
	if !start.IsZero() {
		args = append(args, start.UTC(), end.UTC())
		conds = append(conds, `t.created_at >= $2 AND t.created_at < $3`)
	}
	where := " WHERE " + joinAnd(conds)

	rows, err := r.pool.Query(ctx, `
		SELECT `+trxCols+` FROM transactions t`+where+`
		ORDER BY t.created_at DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	list := []*model.Trx{}
	for rows.Next() {
		t, err := scanTrx(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(list) > 0 {
		ids := make([]string, len(list))
		for i, t := range list {
			ids[i] = t.ID
		}
		if err := attachTrxItems(ctx, r.pool, list, ids); err != nil {
			return nil, err
		}
	}

	bundle := &model.ReportBundle{Period: period, ByMethod: []model.MethodPoint{}, ByStatus: []model.StatusCount{}, Products: []model.ProductReportRow{}, Transactions: []model.TrxProfitRow{}, Stock: []model.StockRow{}}

	prodAgg := map[string]*model.ProductReportRow{}
	for _, t := range list {
		var hpp int64
		for _, it := range t.Items {
			hpp += it.BuyPrice * int64(it.Qty)
			bundle.Summary.ItemsSold += int64(it.Qty)

			pr, ok := prodAgg[it.ProductID]
			if !ok {
				pr = &model.ProductReportRow{ProductID: it.ProductID, Name: it.Name}
				prodAgg[it.ProductID] = pr
			}
			pr.Qty += it.Qty
			pr.Revenue += it.Price * int64(it.Qty)
			pr.Profit += (it.Price - it.BuyPrice) * int64(it.Qty)
		}
		bundle.Summary.Omzet += t.Total
		bundle.Summary.TrxCount++
		bundle.Summary.GrossProfit += t.Total - hpp
		bundle.ByMethod = addMethod(bundle.ByMethod, t.Method, t.Total)

		dateStr := ""
		if !t.CreatedAt.IsZero() {
			dateStr = t.CreatedAt.In(loc).Format("2006-01-02")
		}
		bundle.Transactions = append(bundle.Transactions, model.TrxProfitRow{
			Date: dateStr, ID: t.ID, Cashier: t.CashierName, Method: t.Method,
			Total: t.Total, HPP: hpp, Profit: t.Total - hpp, Status: t.Status,
		})
	}
	sort.Slice(bundle.Products, func(i, j int) bool { return bundle.Products[i].Qty > bundle.Products[j].Qty })
	sort.Slice(bundle.ByMethod, func(i, j int) bool { return bundle.ByMethod[i].Total > bundle.ByMethod[j].Total })

	srows, err := r.pool.Query(ctx,
		`SELECT status, count(*) FROM transactions WHERE store_id=$1 GROUP BY status`, storeID)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var sc model.StatusCount
		if err := srows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, err
		}
		bundle.ByStatus = append(bundle.ByStatus, sc)
	}

	strows, err := r.pool.Query(ctx, `
		SELECT name, sku, stock, buy_price, sell_price FROM products
		WHERE store_id=$1 ORDER BY name ASC
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer strows.Close()
	for strows.Next() {
		var sr model.StockRow
		if err := strows.Scan(&sr.Name, &sr.SKU, &sr.Stock, &sr.BuyPrice, &sr.SellPrice); err != nil {
			return nil, err
		}
		sr.StockValue = sr.BuyPrice * int64(sr.Stock)
		bundle.Stock = append(bundle.Stock, sr)
	}
	return bundle, strows.Err()
}

func addMethod(list []model.MethodPoint, method string, total int64) []model.MethodPoint {
	for i := range list {
		if list[i].Method == method {
			list[i].Total += total
			return list
		}
	}
	return append(list, model.MethodPoint{Method: method, Total: total})
}

func joinAnd(conds []string) string {
	out := conds[0]
	for _, c := range conds[1:] {
		out += " AND " + c
	}
	return out
}
