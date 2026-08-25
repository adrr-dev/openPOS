package service

import (
	"context"

	"github.com/0xMinomus/openPOS/backend/internal/model"
	"github.com/0xMinomus/openPOS/backend/internal/repo"
)

type TrxService struct {
	trx *repo.TrxRepo
}

func NewTrxService(trx *repo.TrxRepo) *TrxService { return &TrxService{trx: trx} }

type CheckoutCmd struct {
	Items    []CheckoutItemCmd
	Discount int64
	Method   string
	Paid     int64
	Customer string
}

type CheckoutItemCmd struct {
	ProductID string
	Qty       int
}

func (s *TrxService) Checkout(ctx context.Context, storeID, cashierID, cashierName string, cmd CheckoutCmd) (*model.Trx, error) {
	items := make([]repo.CheckoutItem, len(cmd.Items))
	for i, it := range cmd.Items {
		items[i] = repo.CheckoutItem{ProductID: it.ProductID, Qty: it.Qty}
	}
	return s.trx.Checkout(ctx, repo.CheckoutInput{
		StoreID: storeID, CashierID: cashierID, CashierName: cashierName,
		Items: items, Discount: cmd.Discount, Method: cmd.Method,
		Paid: cmd.Paid, Customer: cmd.Customer,
	})
}

func (s *TrxService) List(ctx context.Context, storeID, cashierID, q, method, date string, page, limit int) ([]*model.Trx, int, error) {
	return s.trx.List(ctx, storeID, cashierID, q, method, date, page, limit)
}

func (s *TrxService) Get(ctx context.Context, storeID, id string) (*model.Trx, error) {
	return s.trx.GetByID(ctx, storeID, id)
}

func (s *TrxService) Refund(ctx context.Context, storeID, trxID string, items map[string]int, reason, byName string) (*model.Trx, error) {
	return s.trx.Refund(ctx, storeID, trxID, items, reason, byName)
}
