package repo

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type ReportRepo struct {
	db *gorm.DB
}

func NewReportRepo(db *gorm.DB) *ReportRepo { return &ReportRepo{db: db} }

func periodStarts(nowUTC time.Time, loc *time.Location, period string) (time.Time, time.Time, error) {
	local := nowUTC.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch period {
	case "today":
		return today, today.AddDate(0, 0, 1), nil
	case "yesterday":
		return today.AddDate(0, 0, -1), today, nil
	case "week":
		offset := (int(local.Weekday()) + 6) % 7
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

func (r *ReportRepo) Dashboard(ctx context.Context, storeID, cashierID uint, cashierView bool, tz string) (any, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now()
	todayLocal := now.In(loc)
	dayStart := time.Date(todayLocal.Year(), todayLocal.Month(), todayLocal.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1)

	query := r.db.WithContext(ctx).Model(&model.Trx{}).Where("store_id = ? AND status = 'completed' AND created_at >= ? AND created_at < ?", storeID, dayStart.UTC(), dayEnd.UTC())
	if cashierView && cashierID != 0 {
		query = query.Where("cashier_id = ?", cashierID)
	}

	var omzet, cnt int64
	var totalOmzet struct {
		Sum int64
		Cnt int64
	}
	query.Select("COALESCE(SUM(total), 0) as sum, COUNT(*) as cnt").Scan(&totalOmzet)
	omzet = totalOmzet.Sum
	cnt = totalOmzet.Cnt

	// Items sold today
	itemsQuery := r.db.WithContext(ctx).Model(&model.TransactionItem{}).
		Joins("JOIN transactions ON transactions.id = transaction_items.trx_id").
		Where("transactions.store_id = ? AND transactions.status = 'completed' AND transactions.created_at >= ? AND transactions.created_at < ?", storeID, dayStart.UTC(), dayEnd.UTC())
	if cashierView && cashierID != 0 {
		itemsQuery = itemsQuery.Where("transactions.cashier_id = ?", cashierID)
	}
	var itemsSold int64
	itemsQuery.Select("COALESCE(SUM(transaction_items.qty), 0)").Scan(&itemsSold)

	lowStock := int64(0)
	if !cashierView {
		r.db.WithContext(ctx).Model(&model.Product{}).Where("store_id = ? AND active = ? AND stock <= 5", storeID, true).Count(&lowStock)
	}

	recent, err := r.recentTrx(ctx, storeID, cashierID, cashierView, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	todayDTO := model.DashboardToday{Omzet: omzet, TrxCount: cnt, ItemsSold: itemsSold, LowStock: lowStock}

	if cashierView {
		return &model.DashboardCashier{Role: "cashier", Today: todayDTO, Recent: recent}, nil
	}

	weekStart := dayStart.AddDate(0, 0, -6)
	var salesRows []struct {
		D time.Time
		V int64
	}
	if r.db.Dialector.Name() == "sqlite" {
		// sqlite has no AT TIME ZONE / ::date: aggregate in Go for local testing.
		var trxs []model.Trx
		if err := r.db.WithContext(ctx).
			Where("store_id = ? AND status = 'completed' AND created_at >= ? AND created_at < ?", storeID, weekStart.UTC(), dayEnd.UTC()).
			Find(&trxs).Error; err == nil {
			daySum := map[string]int64{}
			for _, t := range trxs {
				k := t.CreatedAt.In(loc).Format("2006-01-02")
				if k >= weekStart.Format("2006-01-02") && k <= todayLocal.Format("2006-01-02") {
					daySum[k] += t.Total
				}
			}
			for k, v := range daySum {
				parsed, _ := time.ParseInLocation("2006-01-02", k, loc)
				salesRows = append(salesRows, struct {
					D time.Time
					V int64
				}{D: parsed, V: v})
			}
		}
		if salesRows == nil {
			salesRows = []struct {
				D time.Time
				V int64
			}{}
		}
	} else {
		r.db.WithContext(ctx).Raw(`
			SELECT (created_at AT TIME ZONE ?)::date as d, COALESCE(SUM(total), 0) as v
			FROM transactions
			WHERE store_id = ? AND status = 'completed'
			  AND (created_at AT TIME ZONE ?)::date >= ?::date
			  AND (created_at AT TIME ZONE ?)::date <= ?::date
			GROUP BY (created_at AT TIME ZONE ?)::date
			ORDER BY d ASC
		`, tz, storeID, tz, weekStart.Format("2006-01-02"), tz, todayLocal.Format("2006-01-02"), tz).Scan(&salesRows)
	}

	byDate := map[string]int64{}
	for _, sr := range salesRows {
		byDate[sr.D.Format("2006-01-02")] = sr.V
	}

	sales7 := make([]model.DayPoint, 0, 7)
	for i := 0; i < 7; i++ {
		d := weekStart.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		sales7 = append(sales7, model.DayPoint{Date: key, Omzet: byDate[key]})
	}

	methods := []model.MethodPoint{}
	r.db.WithContext(ctx).Model(&model.Trx{}).
		Select("method, COALESCE(SUM(total), 0) as total").
		Where("store_id = ? AND status = 'completed' AND created_at >= ? AND created_at < ?", storeID, dayStart.UTC(), dayEnd.UTC()).
		Group("method").
		Order("total DESC").
		Scan(&methods)
	if methods == nil {
		methods = []model.MethodPoint{}
	}

	top := []model.TopProduct{}
	r.db.WithContext(ctx).Model(&model.TransactionItem{}).
		Select("transaction_items.product_id, transaction_items.name, SUM(transaction_items.qty) as qty, SUM(transaction_items.qty * transaction_items.price) as revenue").
		Joins("JOIN transactions ON transactions.id = transaction_items.trx_id").
		Where("transactions.store_id = ? AND transactions.status = 'completed' AND transactions.created_at >= ? AND transactions.created_at < ?", storeID, dayStart.UTC(), dayEnd.UTC()).
		Group("transaction_items.product_id, transaction_items.name").
		Order("qty DESC").
		Limit(5).
		Scan(&top)
	if top == nil {
		top = []model.TopProduct{}
	}

	return &model.DashboardAdmin{
		Role:        "admin",
		Today:       todayDTO,
		Sales7:      sales7,
		Methods:     methods,
		TopProducts: top,
		Recent:      recent,
	}, nil
}

func (r *ReportRepo) recentTrx(ctx context.Context, storeID, cashierID uint, cashierView bool, dayStart, dayEnd time.Time) ([]model.TrxBrief, error) {
	query := r.db.WithContext(ctx).Model(&model.Trx{}).
		Select("id, cashier_name, total, status, created_at as time").
		Where("store_id = ? AND status = 'completed' AND created_at >= ? AND created_at < ?", storeID, dayStart.UTC(), dayEnd.UTC())
	if cashierView && cashierID != 0 {
		query = query.Where("cashier_id = ?", cashierID)
	}
	recent := []model.TrxBrief{}
	err := query.Order("created_at DESC").Limit(5).Scan(&recent).Error
	if recent == nil {
		recent = []model.TrxBrief{}
	}
	return recent, err
}

func (r *ReportRepo) Report(ctx context.Context, storeID uint, period, tz string) (*model.ReportBundle, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	start, end, err := periodStarts(time.Now().UTC(), loc, period)
	if err != nil {
		return nil, err
	}

	query := r.db.WithContext(ctx).Model(&model.Trx{}).Where("store_id = ? AND status = 'completed'", storeID)
	if !start.IsZero() {
		query = query.Where("created_at >= ? AND created_at < ?", start.UTC(), end.UTC())
	}

	var list []*model.Trx
	if err := query.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	if len(list) > 0 {
		ids := make([]uint, len(list))
		for i, t := range list {
			ids[i] = t.ID
		}
		var items []model.TransactionItem
		if err := r.db.WithContext(ctx).Where("trx_id IN ?", ids).Order("id ASC").Find(&items).Error; err == nil {
			byID := map[uint]*model.Trx{}
			for _, t := range list {
				t.Items = []model.TrxItem{}
				byID[t.ID] = t
			}
			for _, it := range items {
				if t, ok := byID[it.TrxID]; ok {
					t.Items = append(t.Items, model.TrxItem{
						ProductID: it.ProductID,
						Name:      it.Name,
						BuyPrice:  it.BuyPrice,
						Price:     it.Price,
						Qty:       it.Qty,
					})
				}
			}
		}
	}

	bundle := &model.ReportBundle{
		Period:       period,
		ByMethod:     []model.MethodPoint{},
		ByStatus:     []model.StatusCount{},
		Products:     []model.ProductReportRow{},
		Transactions: []model.TrxProfitRow{},
		Stock:        []model.StockRow{},
	}

	prodAgg := map[uint]*model.ProductReportRow{}
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
			Date:    dateStr,
			ID:      t.ID,
			Cashier: t.CashierName,
			Method:  t.Method,
			Total:   t.Total,
			HPP:     hpp,
			Profit:  t.Total - hpp,
			Status:  t.Status,
		})
	}

	// Fill SKU via product lookup (TransactionItem has no SKU column).
	if len(prodAgg) > 0 {
		var prodsForSKU []model.Product
		_ = r.db.WithContext(ctx).Where("store_id = ?", storeID).Find(&prodsForSKU).Error
		skuByID := make(map[uint]string, len(prodsForSKU))
		for _, p := range prodsForSKU {
			skuByID[p.ID] = p.SKU
		}
		for _, pr := range prodAgg {
			if sku, ok := skuByID[pr.ProductID]; ok {
				pr.SKU = sku
			}
			bundle.Products = append(bundle.Products, *pr)
		}
	}

	sort.Slice(bundle.Products, func(i, j int) bool { return bundle.Products[i].Qty > bundle.Products[j].Qty })
	sort.Slice(bundle.ByMethod, func(i, j int) bool { return bundle.ByMethod[i].Total > bundle.ByMethod[j].Total })

	r.db.WithContext(ctx).Model(&model.Trx{}).
		Select("status, count(*) as count").
		Where("store_id = ?", storeID).
		Group("status").
		Scan(&bundle.ByStatus)
	if bundle.ByStatus == nil {
		bundle.ByStatus = []model.StatusCount{}
	}
	if bundle.Transactions == nil {
		bundle.Transactions = []model.TrxProfitRow{}
	}
	if bundle.Stock == nil {
		bundle.Stock = []model.StockRow{}
	}

	var prods []model.Product
	r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("name ASC").Find(&prods)
	for _, p := range prods {
		sr := model.StockRow{
			Name:       p.Name,
			SKU:        p.SKU,
			Stock:      p.Stock,
			BuyPrice:   p.BuyPrice,
			SellPrice:  p.SellPrice,
			StockValue: p.BuyPrice * int64(p.Stock),
		}
		bundle.Stock = append(bundle.Stock, sr)
	}

	return bundle, nil
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
