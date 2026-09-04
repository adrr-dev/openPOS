package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	ProductID uint
	Qty       int
}

type CheckoutInput struct {
	StoreID     uint
	CashierID   uint
	CashierName string
	Items       []CheckoutItem
	Discount    int64
	Method      string
	Paid        int64
	Customer    string
}

type TrxRepo struct {
	db *gorm.DB
}

func NewTrxRepo(db *gorm.DB) *TrxRepo { return &TrxRepo{db: db} }

// isPostgres reports whether the dialector supports pg locks (FOR UPDATE,
// pg_advisory_xact_lock). Sqlite path skips them — same results, no crash.
func (r *TrxRepo) isPostgres() bool {
	return r.db.Dialector.Name() != "sqlite"
}

func (r *TrxRepo) Checkout(ctx context.Context, in CheckoutInput) (*model.Trx, error) {
	if len(in.Items) == 0 {
		return nil, ErrEmptyItems
	}
	if !model.ValidPayMethod(in.Method) {
		return nil, fmt.Errorf("metode pembayaran tidak valid")
	}

	merged := make([]CheckoutItem, 0, len(in.Items))
	idx := map[uint]int{}
	for _, it := range in.Items {
		if j, ok := idx[it.ProductID]; ok {
			merged[j].Qty += it.Qty
			continue
		}
		idx[it.ProductID] = len(merged)
		merged = append(merged, it)
	}
	in.Items = merged

	var resultTrx *model.Trx

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize checkouts per store on postgres. Skipped on sqlite.
		// IDs are numeric now, so pass the store as text for hashtext.
		if r.isPostgres() {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", strconv.FormatUint(uint64(in.StoreID), 10)).Error; err != nil {
				return err
			}
		}

		var store model.Store
		if err := tx.Where("id = ?", in.StoreID).First(&store).Error; err != nil {
			return mapDBErr(err)
		}

		type line struct {
			item model.TrxItem
			qty  int
		}
		lines := make([]line, 0, len(in.Items))
		var subtotal int64

		for _, it := range in.Items {
			if it.Qty < 1 {
				return fmt.Errorf("qty harus minimal 1")
			}
			var prod model.Product
			q := tx
			// FOR UPDATE row lock on postgres prevents concurrent oversell.
			if r.isPostgres() {
				q = q.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			if err := q.Where("id = ? AND store_id = ?", it.ProductID, in.StoreID).First(&prod).Error; err != nil {
				if isNotFound(err) {
					return ErrNotFound
				}
				return err
			}
			if !prod.Active {
				return ErrProductInactive
			}
			if prod.Stock < it.Qty {
				return ErrStockInsufficient
			}

			pi := model.TrxItem{
				ProductID: prod.ID,
				Name:      prod.Name,
				BuyPrice:  prod.BuyPrice,
				Price:     prod.SellPrice,
				Qty:       it.Qty,
			}
			subtotal += pi.Price * int64(it.Qty)
			lines = append(lines, line{item: pi, qty: it.Qty})
		}

		discount := in.Discount
		if discount < 0 || discount > subtotal {
			return ErrBadDiscount
		}

		var tax int64
		if store.TaxEnabled && store.TaxPct > 0 {
			tax = roundHalfUp(float64(subtotal-discount) * store.TaxPct / 100)
		}
		total := subtotal - discount + tax

		paid, change := in.Paid, int64(0)
		if in.Method == string(model.PayCash) {
			if paid < total {
				return ErrPaidInsufficient
			}
			change = paid - total
		} else {
			paid = total
		}

		trxItemsList := make(model.TrxItemList, len(lines))
		for i, l := range lines {
			trxItemsList[i] = l.item
		}

		trx := model.Trx{
			StoreID:     in.StoreID,
			CashierID:   in.CashierID,
			CashierName: in.CashierName,
			Items:       trxItemsList,
			Subtotal:    subtotal,
			Discount:    discount,
			Tax:         tax,
			Total:       total,
			Method:      in.Method,
			Paid:        paid,
			Change:      change,
			Status:      model.TrxCompleted,
			Customer:    in.Customer,
		}

		if err := tx.Create(&trx).Error; err != nil {
			return mapDBErr(err)
		}

		for _, l := range lines {
			tItem := model.TransactionItem{
				TrxID:     trx.ID,
				ProductID: l.item.ProductID,
				Name:      l.item.Name,
				BuyPrice:  l.item.BuyPrice,
				Price:     l.item.Price,
				Qty:       l.qty,
			}
			if err := tx.Create(&tItem).Error; err != nil {
				return err
			}

			// Atomic decrement with guard — catches race between check and write.
			res := tx.Model(&model.Product{}).
				Where("id = ? AND store_id = ? AND stock >= ?", l.item.ProductID, in.StoreID, l.qty).
				Update("stock", gorm.Expr("stock - ?", l.qty))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return ErrStockInsufficient
			}

			mv := model.Movement{
				StoreID:   in.StoreID,
				ProductID: l.item.ProductID,
				Type:      model.MovementSale,
				Qty:       -l.qty,
				Reason:    fmt.Sprintf("Penjualan %d", trx.ID),
				Actor:     in.CashierName,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return err
			}
		}

		resultTrx = &trx
		return nil
	})

	if err != nil {
		return nil, err
	}
	return resultTrx, nil
}

func (r *TrxRepo) Refund(ctx context.Context, storeID, trxID uint, items map[uint]int, reason, byName string) (*model.Trx, error) {
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("alasan refund wajib diisi")
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if r.isPostgres() {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", strconv.FormatUint(uint64(storeID), 10)).Error; err != nil {
				return err
			}
		}
		var t model.Trx
		q := tx
		if r.isPostgres() {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.Where("id = ? AND store_id = ?", trxID, storeID).First(&t).Error; err != nil {
			if isNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if t.Status != model.TrxCompleted {
			return ErrNotRefundable
		}

		var tItems []model.TransactionItem
		if err := tx.Where("trx_id = ?", trxID).Find(&tItems).Error; err != nil {
			return err
		}

		sold := map[uint]model.TrxItem{}
		for _, ti := range tItems {
			if prev, ok := sold[ti.ProductID]; ok {
				prev.Qty += ti.Qty
				sold[ti.ProductID] = prev
			} else {
				sold[ti.ProductID] = model.TrxItem{
					ProductID: ti.ProductID,
					Name:      ti.Name,
					BuyPrice:  ti.BuyPrice,
					Price:     ti.Price,
					Qty:       ti.Qty,
				}
			}
		}

		var refunds []model.Refund
		if err := tx.Where("trx_id = ?", trxID).Find(&refunds).Error; err != nil {
			return err
		}

		refunded := map[uint]int{}
		for _, ref := range refunds {
			for pid, q := range ref.Items {
				refunded[pid] += q
			}
		}

		processed := model.RefundItemList{}
		for pid, want := range items {
			s, ok := sold[pid]
			if !ok || want < 0 {
				return ErrRefundTooMuch
			}
			if want == 0 {
				continue
			}
			if refunded[pid]+want > s.Qty {
				return ErrRefundTooMuch
			}

			if err := tx.Model(&model.Product{}).Where("id = ? AND store_id = ?", pid, storeID).Update("stock", gorm.Expr("stock + ?", want)).Error; err != nil {
				return err
			}

			mv := model.Movement{
				StoreID:   storeID,
				ProductID: pid,
				Type:      model.MovementRefund,
				Qty:       want,
				Reason:    fmt.Sprintf("Refund %d", trxID),
				Actor:     byName,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return err
			}

			processed[pid] = want
		}

		if len(processed) == 0 {
			return ErrEmptyItems
		}

		full := true
		for pid, s := range sold {
			if refunded[pid]+processed[pid] < s.Qty {
				full = false
				break
			}
		}

		refRecord := model.Refund{
			StoreID: storeID,
			TrxID:   trxID,
			Items:   processed,
			Reason:  strings.TrimSpace(reason),
			ByName:  byName,
		}
		if err := tx.Create(&refRecord).Error; err != nil {
			return err
		}

		if full {
			if err := tx.Model(&model.Trx{}).Where("id = ? AND store_id = ?", trxID, storeID).Update("status", model.TrxRefunded).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, storeID, trxID)
}

func (r *TrxRepo) List(ctx context.Context, storeID, cashierID uint, q, method, date string, page, limit int) ([]*model.Trx, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	query := r.db.WithContext(ctx).Model(&model.Trx{}).Where("store_id = ?", storeID)
	if cashierID != 0 {
		query = query.Where("cashier_id = ?", cashierID)
	}
	if qs := strings.TrimSpace(q); qs != "" {
		searchTerm := "%" + strings.ToLower(qs) + "%"
		// IDs are numeric now: exact match when q is a number, name search otherwise.
		if n, err := strconv.ParseUint(qs, 10, 64); err == nil {
			query = query.Where("id = ? OR lower(cashier_name) LIKE ?", n, searchTerm)
		} else {
			query = query.Where("lower(cashier_name) LIKE ?", searchTerm)
		}
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if date != "" {
		if r.db.Dialector.Name() == "sqlite" {
			query = query.Where("date(created_at) = date(?)", date)
		} else {
			query = query.Where("created_at::date = ?::date", date)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	list := make([]*model.Trx, 0)
	offset := (page - 1) * limit
	err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&list).Error
	if err != nil {
		return nil, 0, err
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

	return list, int(total), nil
}

func (r *TrxRepo) GetByID(ctx context.Context, storeID, id uint) (*model.Trx, error) {
	var t model.Trx
	if err := r.db.WithContext(ctx).Where("id = ? AND store_id = ?", id, storeID).First(&t).Error; err != nil {
		return nil, mapDBErr(err)
	}

	t.Items = []model.TrxItem{}
	var items []model.TransactionItem
	if err := r.db.WithContext(ctx).Where("trx_id = ?", id).Order("id ASC").Find(&items).Error; err == nil {
		for _, it := range items {
			t.Items = append(t.Items, model.TrxItem{
				ProductID: it.ProductID,
				Name:      it.Name,
				BuyPrice:  it.BuyPrice,
				Price:     it.Price,
				Qty:       it.Qty,
			})
		}
	}
	return &t, nil
}

func roundHalfUp(f float64) int64 {
	return int64(f + 0.5)
}
