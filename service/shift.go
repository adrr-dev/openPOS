package service

import (
	"context"
	"errors"
	"sort"
	"time"
)

var (
	ErrNoActiveShift = errors.New("tidak ada shift yang berjalan")
)

type ShiftService struct {
	shifts   ShiftRepository
	cashiers CashierRepository
	stores   StoreRepository
}

func NewShiftService(shifts ShiftRepository, cashiers CashierRepository, stores StoreRepository) *ShiftService {
	return &ShiftService{shifts: shifts, cashiers: cashiers, stores: stores}
}

func (s *ShiftService) resolveCashier(ctx context.Context, storeID uint, actingAsCashierID *uint, fallbackName string) (uint, string, error) {
	// Admin tanpa switch memakai cashier_id=0 (shift admin sendiri),
	// TANPA membuat baris cashiers kloningan. Kasir memakai id-nya.
	cashierName := fallbackName
	if actingAsCashierID != nil {
		if c, err := s.cashiers.GetByID(ctx, *actingAsCashierID); err == nil {
			cashierName = c.Name
		}
		return *actingAsCashierID, cashierName, nil
	}
	return 0, cashierName, nil
}

func (s *ShiftService) GetCashierShift(ctx context.Context, storeID uint, actingAsCashierID *uint, fallbackName string) (any, error) {
	cashierID, _, err := s.resolveCashier(ctx, storeID, actingAsCashierID, fallbackName)
	if err != nil {
		return nil, err
	}

	shift, err := s.shifts.GetActiveShift(ctx, storeID, cashierID)
	if err != nil {
		return nil, ErrNoActiveShift
	}

	now := time.Now()
	sales, trxCount, allItems, trxs, err := s.shifts.GetTrxStatsForShift(ctx, storeID, cashierID, shift.StartedAt, now)
	if err != nil {
		return nil, err
	}

	hourMap := make(map[int]int64)
	for _, t := range trxs {
		h := t.CreatedAt.Hour()
		hourMap[h] += t.Total
	}
	var hourly []map[string]any
	for h := 0; h < 24; h++ {
		if h >= shift.StartedAt.Hour() && h <= now.Hour() {
			hourly = append(hourly, map[string]any{
				"hour":  h,
				"omzet": hourMap[h],
			})
		}
	}
	if hourly == nil {
		hourly = []map[string]any{}
	}

	prodQty := make(map[uint]struct {
		name string
		qty  int
	})
	for _, it := range allItems {
		p := prodQty[it.ProductID]
		p.name = it.Name
		p.qty += it.Qty
		prodQty[it.ProductID] = p
	}

	type topProd struct {
		ProductID uint   `json:"product_id"`
		Name      string `json:"name"`
		Qty       int    `json:"qty"`
	}
	var tops []topProd
	for pid, p := range prodQty {
		tops = append(tops, topProd{ProductID: pid, Name: p.name, Qty: p.qty})
	}
	sort.Slice(tops, func(i, j int) bool {
		return tops[i].Qty > tops[j].Qty
	})
	if len(tops) > 5 {
		tops = tops[:5]
	}
	if tops == nil {
		tops = []topProd{}
	}

	return map[string]any{
		"shift": map[string]any{
			"started_at":   shift.StartedAt,
			"opening_cash": shift.OpeningCash,
			"sales":        sales,
			"trx_count":    trxCount,
		},
		"hourly":       hourly,
		"top_products": tops,
	}, nil
}

func (s *ShiftService) StartShift(ctx context.Context, storeID uint, actingAsCashierID *uint, fallbackName string, openingCash int64) (any, error) {
	cashierID, cashierName, err := s.resolveCashier(ctx, storeID, actingAsCashierID, fallbackName)
	if err != nil {
		return nil, err
	}

	shift, err := s.shifts.StartShift(ctx, storeID, cashierID, cashierName, openingCash)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"shift": map[string]any{
			"started_at":   shift.StartedAt,
			"opening_cash": shift.OpeningCash,
			"sales":        int64(0),
			"trx_count":    int64(0),
		},
	}, nil
}

func (s *ShiftService) CloseShift(ctx context.Context, storeID uint, actingAsCashierID *uint, fallbackName string) (any, error) {
	cashierID, _, err := s.resolveCashier(ctx, storeID, actingAsCashierID, fallbackName)
	if err != nil {
		return nil, err
	}

	_, err = s.shifts.GetActiveShift(ctx, storeID, cashierID)
	if err != nil {
		return nil, ErrNoActiveShift
	}

	closed, err := s.shifts.CloseShift(ctx, storeID, cashierID)
	if err != nil {
		return nil, err
	}

	sales, trxCount, _, _, err := s.shifts.GetTrxStatsForShift(ctx, storeID, cashierID, closed.StartedAt, *closed.ClosedAt)
	if err != nil {
		sales, trxCount = 0, 0
	}

	return map[string]any{
		"message": "Shift ditutup.",
		"summary": map[string]any{
			"sales":     sales,
			"trx_count": trxCount,
		},
	}, nil
}

func (s *ShiftService) ListShifts(ctx context.Context, storeID uint) (any, error) {
	shifts, err := s.shifts.ListStoreShifts(ctx, storeID)
	if err != nil {
		return nil, err
	}

	type shiftResponse struct {
		ID          uint       `json:"id"`
		CashierName string     `json:"cashier_name"`
		StartedAt   time.Time  `json:"started_at"`
		ClosedAt    *time.Time `json:"closed_at"`
		OpeningCash int64      `json:"opening_cash"`
		Sales       int64      `json:"sales"`
		TrxCount    int64      `json:"trx_count"`
	}

	var results []shiftResponse
	for _, sh := range shifts {
		endTime := time.Now()
		if sh.ClosedAt != nil {
			endTime = *sh.ClosedAt
		}
		sales, trxCount, _, _, _ := s.shifts.GetTrxStatsForShift(ctx, storeID, sh.CashierID, sh.StartedAt, endTime)
		results = append(results, shiftResponse{
			ID:          sh.ID,
			CashierName: sh.CashierName,
			StartedAt:   sh.StartedAt,
			ClosedAt:    sh.ClosedAt,
			OpeningCash: sh.OpeningCash,
			Sales:       sales,
			TrxCount:    trxCount,
		})
	}

	if results == nil {
		results = []shiftResponse{}
	}

	return map[string]any{
		"shifts": results,
	}, nil
}
