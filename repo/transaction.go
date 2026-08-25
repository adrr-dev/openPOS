package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0xMinomus/openPOS/backend/model"
)

var (
	ErrPaidInsufficient = errors.New("jumlah bayar kurang dari total")
	ErrBadDiscount      = errors.New("diskon melebihi subtotal")
	ErrProductInactive  = errors.New("produk tidak aktif")
	ErrNotRefundable    = errors.New("transaksi ini tidak dapat direfund")
	ErrRefundTooMuch    = errors.New("qty refund melebihi jumlah terjual")
	ErrEmptyItems       = errors.New("keranjang kosong")
)

type CheckoutItem struct {
	ProductID string
	Qty       int
}

type CheckoutInput struct {
	StoreID     string
	CashierID   string
	CashierName string
	Items       []CheckoutItem
	Discount    int64
	Method      string
	Paid        int64
	Customer    string
}

const trxCols = `t.id, t.seq, t.cashier_id, t.cashier_name, t.subtotal, t.discount,
	t.tax, t.total, t.method, t.paid, t.change, t.status, t.customer, t.created_at`

func scanTrx(row interface{ Scan(...any) error }) (*model.Trx, error) {
	var t model.Trx
	if err := row.Scan(&t.ID, &t.Seq, &t.CashierID, &t.CashierName, &t.Subtotal, &t.Discount,
		&t.Tax, &t.Total, &t.Method, &t.Paid, &t.Change, &t.Status, &t.Customer, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

type TrxRepo struct {
	pool *pgxpool.Pool
}

func NewTrxRepo(pool *pgxpool.Pool) *TrxRepo { return &TrxRepo{pool: pool} }

// Checkout menjalankan seluruh alur penjualan dalam SATU transaksi DB:
// advisory-lock per toko (EC-001) → kunci produk (FOR UPDATE) → validasi stok &
// hitung ulang harga di server → seq per toko → insert trx+items → kurangi stok
// → movement 'sale'. (FR-POS-005..009)
func (r *TrxRepo) Checkout(ctx context.Context, in CheckoutInput) (*model.Trx, error) {
	if len(in.Items) == 0 {
		return nil, ErrEmptyItems
	}
	if !model.ValidPayMethod(in.Method) {
		return nil, fmt.Errorf("metode pembayaran tidak valid")
	}

	// Gabungkan baris dengan produk yang sama (klien yang buruk pun tak boleh
	// membuat duplikat baris item untuk satu produk).
	merged := make([]CheckoutItem, 0, len(in.Items))
	idx := map[string]int{}
	for _, it := range in.Items {
		if j, ok := idx[it.ProductID]; ok {
			merged[j].Qty += it.Qty
			continue
		}
		idx[it.ProductID] = len(merged)
		merged = append(merged, it)
	}
	in.Items = merged

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// serialisasi seluruh checkout dalam toko yang sama
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, in.StoreID); err != nil {
		return nil, err
	}

	// pengaturan pajak toko
	var taxEnabled bool
	var taxPct float64
	if err := tx.QueryRow(ctx,
		`SELECT tax_enabled, tax_pct FROM stores WHERE id = $1`, in.StoreID,
	).Scan(&taxEnabled, &taxPct); err != nil {
		return nil, mapDBErr(err)
	}

	type line struct {
		item model.TrxItem
		qty  int
	}
	lines := make([]line, 0, len(in.Items))
	var subtotal int64

	for _, it := range in.Items {
		if it.Qty < 1 {
			return nil, fmt.Errorf("qty harus minimal 1")
		}
		var pi model.TrxItem
		var stock int
		var active bool
		err := tx.QueryRow(ctx, `
			SELECT id, name, buy_price, sell_price, stock, active
			FROM products WHERE id = $1 AND store_id = $2 FOR UPDATE
		`, it.ProductID, in.StoreID).Scan(&pi.ProductID, &pi.Name, &pi.BuyPrice, &pi.Price, &stock, &active)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, mapDBErr(err)
		}
		if !active {
			return nil, ErrProductInactive
		}
		if stock < it.Qty {
			return nil, ErrStockInsufficient
		}
		pi.Qty = it.Qty
		subtotal += pi.Price * int64(it.Qty)
		lines = append(lines, line{item: pi, qty: it.Qty})
	}

	discount := in.Discount
	if discount < 0 || discount > subtotal {
		return nil, ErrBadDiscount
	}
	var tax int64
	if taxEnabled && taxPct > 0 {
		tax = roundHalfUp(float64(subtotal-discount) * taxPct / 100)
	}
	total := subtotal - discount + tax

	paid, change := in.Paid, int64(0)
	if in.Method == string(model.PayCash) {
		if paid < total {
			return nil, ErrPaidInsufficient
		}
		change = paid - total
	} else {
		paid = total // non-cash dicatat persis total
	}

	var seq int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM transactions WHERE store_id = $1`, in.StoreID,
	).Scan(&seq); err != nil {
		return nil, err
	}
	id := "TRX-" + pad4(seq)

	if _, err := tx.Exec(ctx, `
		INSERT INTO transactions (id, seq, store_id, cashier_id, cashier_name,
			subtotal, discount, tax, total, method, paid, change, status, customer)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'completed',$13)
	`, id, seq, in.StoreID, in.CashierID, in.CashierName,
		subtotal, discount, tax, total, in.Method, paid, change, in.Customer); err != nil {
		return nil, mapDBErr(err)
	}

	for _, l := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO transaction_items (trx_id, product_id, name, buy_price, price, qty)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, id, l.item.ProductID, l.item.Name, l.item.BuyPrice, l.item.Price, l.qty); err != nil {
			return nil, mapDBErr(err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock = stock - $3 WHERE id = $1 AND store_id = $2`,
			l.item.ProductID, in.StoreID, l.qty); err != nil {
			return nil, err
		}
		if err := insertMovementTx(ctx, tx, &model.Movement{
			StoreID:   in.StoreID,
			ProductID: l.item.ProductID,
			Type:      model.MovementSale,
			Qty:       -l.qty,
			Reason:    "Penjualan " + id,
			Actor:     in.CashierName,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	out := &model.Trx{
		ID: id, Seq: seq, CashierID: in.CashierID, CashierName: in.CashierName,
		Items:    make([]model.TrxItem, 0, len(lines)),
		Subtotal: subtotal, Discount: discount, Tax: tax, Total: total,
		Method: in.Method, Paid: paid, Change: change,
		Status: model.TrxCompleted, Customer: in.Customer, CreatedAt: time.Now(),
	}
	for _, l := range lines {
		l.item.Qty = l.qty
		out.Items = append(out.Items, l.item)
	}
	return out, nil
}

// Refund memproses refund parsial/penuh (FR-REF-001..005):
// kunci trx → tolak bila bukan 'completed' atau dobel melebihi terjual (EC-004)
// → kembalikan stok + movement 'refund' → catat refunds → status 'refunded' bila penuh.
func (r *TrxRepo) Refund(ctx context.Context, storeID, trxID string, items map[string]int, reason, byName string) (*model.Trx, error) {
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("alasan refund wajib diisi")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, storeID); err != nil {
		return nil, err
	}

	t, err := scanTrx(tx.QueryRow(ctx, `
		SELECT `+trxCols+` FROM transactions t WHERE t.id = $1 AND t.store_id = $2 FOR UPDATE
	`, trxID, storeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapDBErr(err)
	}
	if t.Status != model.TrxCompleted {
		return nil, ErrNotRefundable
	}

	// item terjual
	sold := make(map[string]model.TrxItem)
	rows, err := tx.Query(ctx,
		`SELECT product_id, name, buy_price, price, qty FROM transaction_items WHERE trx_id = $1`, trxID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var it model.TrxItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.BuyPrice, &it.Price, &it.Qty); err != nil {
			rows.Close()
			return nil, err
		}
		// baris duplikat untuk produk yang sama dijumlahkan (data lama pra-perbaikan)
		if prev, ok := sold[it.ProductID]; ok {
			prev.Qty += it.Qty
			sold[it.ProductID] = prev
		} else {
			sold[it.ProductID] = it
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// qty yang sudah direfund sebelumnya (refund parsial berulang diperbolehkan)
	refunded := make(map[string]int)
	rrows, err := tx.Query(ctx, `SELECT items FROM refunds WHERE trx_id = $1`, trxID)
	if err != nil {
		return nil, err
	}
	for rrows.Next() {
		var raw []byte
		if err := rrows.Scan(&raw); err != nil {
			rrows.Close()
			return nil, err
		}
		var m map[string]int
		if err := json.Unmarshal(raw, &m); err != nil {
			rrows.Close()
			return nil, err
		}
		for pid, q := range m {
			refunded[pid] += q
		}
	}
	rrows.Close()
	if err := rrows.Err(); err != nil {
		return nil, err
	}

	full := true
	processed := make(map[string]int, len(items))
	for pid, want := range items {
		s, ok := sold[pid]
		if !ok || want < 0 {
			return nil, ErrRefundTooMuch
		}
		if want == 0 {
			continue
		}
		if refunded[pid]+want > s.Qty {
			return nil, ErrRefundTooMuch
		}
		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock = stock + $3 WHERE id = $1 AND store_id = $2`,
			pid, storeID, want); err != nil {
			return nil, err
		}
		if err := insertMovementTx(ctx, tx, &model.Movement{
			StoreID:   storeID,
			ProductID: pid,
			Type:      model.MovementRefund,
			Qty:       want,
			Reason:    "Refund " + trxID,
			Actor:     byName,
		}); err != nil {
			return nil, err
		}
		processed[pid] = want
	}
	if len(processed) == 0 {
		return nil, ErrEmptyItems
	}
	// penuh hanya bila SETIAP item terjual kini sudah kembali seluruhnya
	for pid, s := range sold {
		if refunded[pid]+processed[pid] < s.Qty {
			full = false
			break
		}
	}

	rawItems, err := json.Marshal(processed)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refunds (store_id, trx_id, items, reason, by_name)
		VALUES ($1, $2, $3::jsonb, $4, $5)
	`, storeID, trxID, string(rawItems), strings.TrimSpace(reason), byName); err != nil {
		return nil, err
	}

	if full {
		if _, err := tx.Exec(ctx,
			`UPDATE transactions SET status = 'refunded' WHERE id = $1 AND store_id = $2`, trxID, storeID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	fresh, err := r.GetByID(ctx, storeID, trxID)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// List daftar transaksi; kasir otomatis dibatasi miliknya (cashierID ≠ ""). Items disertakan.
func (r *TrxRepo) List(ctx context.Context, storeID, cashierID, q, method, date string, page, limit int) ([]*model.Trx, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	conds := []string{"t.store_id = $1"}
	args := []any{storeID}
	if cashierID != "" {
		args = append(args, cashierID)
		conds = append(conds, `t.cashier_id = $`+strconv.Itoa(len(args)))
	}
	if qs := strings.TrimSpace(q); qs != "" {
		args = append(args, "%"+strings.ToLower(qs)+"%")
		n := len(args)
		conds = append(conds, `(lower(t.id) LIKE $`+strconv.Itoa(n)+` OR lower(t.cashier_name) LIKE $`+strconv.Itoa(n)+`)`)
	}
	if method != "" {
		args = append(args, method)
		conds = append(conds, `t.method = $`+strconv.Itoa(len(args)))
	}
	if date != "" {
		args = append(args, date)
		conds = append(conds, `t.created_at::date = $`+strconv.Itoa(len(args))+`::date`)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions t`+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	args = append(args, limit, (page-1)*limit)

	rows, err := r.pool.Query(ctx, `
		SELECT `+trxCols+` FROM transactions t`+where+`
		ORDER BY t.created_at DESC, t.seq DESC
		LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx), args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.Trx, 0, limit)
	for rows.Next() {
		t, err := scanTrx(rows)
		if err != nil {
			return nil, 0, err
		}
		t.Items = []model.TrxItem{}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(list) > 0 {
		ids := make([]string, len(list))
		for i, t := range list {
			ids[i] = t.ID
		}
		if err := attachTrxItems(ctx, r.pool, list, ids); err != nil {
			return nil, 0, err
		}
	}
	return list, total, nil
}

// rowQuerier diimplementasikan pgxpool.Pool (dan tx bila perlu).
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func attachTrxItems(ctx context.Context, q rowQuerier, list []*model.Trx, ids []string) error {
	irows, err := q.Query(ctx, `
		SELECT trx_id, product_id, name, buy_price, price, qty
		FROM transaction_items WHERE trx_id = ANY($1)
		ORDER BY id ASC
	`, ids)
	if err != nil {
		return err
	}
	defer irows.Close()

	byID := make(map[string]*model.Trx, len(list))
	for _, t := range list {
		byID[t.ID] = t
	}
	for irows.Next() {
		var tid string
		var it model.TrxItem
		if err := irows.Scan(&tid, &it.ProductID, &it.Name, &it.BuyPrice, &it.Price, &it.Qty); err != nil {
			return err
		}
		if t, ok := byID[tid]; ok {
			t.Items = append(t.Items, it)
		}
	}
	return irows.Err()
}

// GetByID detail transaksi beserta item.
func (r *TrxRepo) GetByID(ctx context.Context, storeID, id string) (*model.Trx, error) {
	t, err := scanTrx(r.pool.QueryRow(ctx, `
		SELECT `+trxCols+` FROM transactions t WHERE t.id = $1 AND t.store_id = $2
	`, id, storeID))
	if err != nil {
		return nil, mapDBErr(err)
	}
	t.Items = []model.TrxItem{}
	irows, err := r.pool.Query(ctx, `
		SELECT product_id, name, buy_price, price, qty FROM transaction_items WHERE trx_id = $1 ORDER BY id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	for irows.Next() {
		var it model.TrxItem
		if err := irows.Scan(&it.ProductID, &it.Name, &it.BuyPrice, &it.Price, &it.Qty); err != nil {
			return nil, err
		}
		t.Items = append(t.Items, it)
	}
	return t, irows.Err()
}

func pad4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func roundHalfUp(f float64) int64 {
	return int64(f + 0.5)
}
