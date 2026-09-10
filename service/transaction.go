package service

import (
	"context"
	"fmt"

	"github.com/adrr-dev/openPOS/backend/model"
	"github.com/adrr-dev/openPOS/backend/repo"
)

type TrxService struct {
	trx      TrxRepository
	cashiers CashierRepository
	prods    ProductRepository
	notif    *NotificationService
}

func NewTrxService(trx TrxRepository, cashiers CashierRepository, prods ProductRepository, notif *NotificationService) *TrxService {
	return &TrxService{trx: trx, cashiers: cashiers, prods: prods, notif: notif}
}

type CheckoutCmd struct {
	Items    []CheckoutItemCmd
	Discount int64
	Method   string
	Paid     int64
	Customer string
}

type CheckoutItemCmd struct {
	ProductID uint
	Qty       int
}

func (s *TrxService) Checkout(ctx context.Context, storeID uint, actingAsCashierID *uint, fallbackName string, cmd CheckoutCmd) (*model.Trx, error) {
	// RBAC: transaksi selalu pakai identitas dari JWT (claims), bukan dari payload.
	// fallbackName = claims.Name (JWT), actingAsCashierID = claims.ActingAsCashierID.
	// Jika admin tidak switch (actingAs==nil), buat/ambil baris cashiers bernama admin
	// sebagai pemilik transaksi agar admin tetap bisa checkout tanpa dianggap Kasir lain
	// (idempotent per store_id+name via GetOrCreateByName). Tidak pernah buat users/KASIR.
	var cashierID uint
	cashierName := fallbackName

	if actingAsCashierID != nil {
		cashierID = *actingAsCashierID
		c, err := s.cashiers.GetByID(ctx, cashierID)
		if err == nil {
			cashierName = c.Name
		}
	} else {
		defID, err := s.cashiers.GetOrCreateByName(ctx, storeID, fallbackName)
		if err != nil {
			return nil, err
		}
		cashierID = defID
		c, err := s.cashiers.GetByID(ctx, cashierID)
		if err == nil {
			cashierName = c.Name
		}
	}

	items := make([]repo.CheckoutItem, len(cmd.Items))
	for i, it := range cmd.Items {
		items[i] = repo.CheckoutItem{ProductID: it.ProductID, Qty: it.Qty}
	}
	resTrx, err := s.trx.Checkout(ctx, repo.CheckoutInput{
		StoreID: storeID, CashierID: cashierID, CashierName: cashierName,
		Items: items, Discount: cmd.Discount, Method: cmd.Method,
		Paid: cmd.Paid, Customer: cmd.Customer,
	})
	if err == nil && s.notif != nil {
		actorIDStr := fmt.Sprintf("%d", cashierID)
		refIDStr := fmt.Sprintf("%d", resTrx.ID)
		title := "Transaksi baru"
		message := fmt.Sprintf("Rp%d · %s", resTrx.Total, resTrx.Method)
		_ = s.notif.Emit(ctx, storeID, title, message, model.CategoryTransaksi, "transaction_created", actorIDStr, cashierName, "transaction", refIDStr)

		if s.prods != nil {
			for _, it := range cmd.Items {
				if prod, err := s.prods.GetByID(ctx, storeID, it.ProductID); err == nil && prod != nil {
					_ = s.notif.CheckAndNotifyLowStock(ctx, storeID, prod.ID, prod.Name, prod.Stock, prod.Unit)
				}
			}
		}
	}
	return resTrx, err
}

func (s *TrxService) List(ctx context.Context, storeID, cashierID uint, q, method, date string, page, limit int) ([]*model.Trx, int, error) {
	return s.trx.List(ctx, storeID, cashierID, q, method, date, page, limit)
}

func (s *TrxService) Get(ctx context.Context, storeID, id uint) (*model.Trx, error) {
	return s.trx.GetByID(ctx, storeID, id)
}

func (s *TrxService) Refund(ctx context.Context, storeID, trxID uint, items map[uint]int, reason, byName string) (*model.Trx, error) {
	trx, err := s.trx.Refund(ctx, storeID, trxID, items, reason, byName)
	if err == nil && s.notif != nil {
		refIDStr := fmt.Sprintf("%d", trxID)
		title := "Refund"
		message := fmt.Sprintf("Invoice #TRX-%05d oleh %s", trxID, byName)
		_ = s.notif.Emit(ctx, storeID, title, message, model.CategoryTransaksi, "refund_created", "", byName, "transaction", refIDStr)
	}
	return trx, err
}
