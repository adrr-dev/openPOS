package service

import (
	"context"

	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
)

type TrxService struct {
	trx      *repo.TrxRepo
	cashiers *repo.CashierRepo
}

func NewTrxService(trx *repo.TrxRepo, cashiers *repo.CashierRepo) *TrxService {
	return &TrxService{trx: trx, cashiers: cashiers}
}

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

func (s *TrxService) Checkout(ctx context.Context, storeID string, actingAsCashierID *string, fallbackName string, cmd CheckoutCmd) (*model.Trx, error) {
	cashierID := ""
	cashierName := fallbackName

	if actingAsCashierID != nil {
		cashierID = *actingAsCashierID
		c, err := s.cashiers.GetByID(ctx, cashierID)
		if err == nil {
			cashierName = c.Name
		}
	} else {
		defID, err := s.cashiers.GetOrCreateDefault(ctx, storeID, fallbackName)
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
