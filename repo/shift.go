package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/adrr-dev/openPOS/backend/model"
)

type ShiftRepo struct {
	db *gorm.DB
}

func NewShiftRepo(db *gorm.DB) *ShiftRepo {
	return &ShiftRepo{db: db}
}

func (r *ShiftRepo) GetActiveShift(ctx context.Context, storeID, cashierID uint) (*model.CashierShift, error) {
	var s model.CashierShift
	err := r.db.WithContext(ctx).Where("store_id = ? AND cashier_id = ? AND closed_at IS NULL", storeID, cashierID).Order("started_at DESC").First(&s).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &s, nil
}

func (r *ShiftRepo) StartShift(ctx context.Context, storeID, cashierID uint, cashierName string, openingCash int64) (*model.CashierShift, error) {
	var active model.CashierShift
	err := r.db.WithContext(ctx).Where("store_id = ? AND cashier_id = ? AND closed_at IS NULL", storeID, cashierID).First(&active).Error
	now := time.Now()
	if err == nil {
		_ = r.db.WithContext(ctx).Model(&active).Update("closed_at", &now)
	}

	shift := model.CashierShift{
		StoreID:     storeID,
		CashierID:   cashierID,
		CashierName: cashierName,
		StartedAt:   now,
		OpeningCash: openingCash,
		ClosedAt:    nil,
	}
	if err := r.db.WithContext(ctx).Create(&shift).Error; err != nil {
		return nil, mapDBErr(err)
	}
	return &shift, nil
}

func (r *ShiftRepo) CloseShift(ctx context.Context, storeID, cashierID uint) (*model.CashierShift, error) {
	var active model.CashierShift
	err := r.db.WithContext(ctx).Where("store_id = ? AND cashier_id = ? AND closed_at IS NULL", storeID, cashierID).First(&active).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	now := time.Now()
	active.ClosedAt = &now
	if err := r.db.WithContext(ctx).Save(&active).Error; err != nil {
		return nil, mapDBErr(err)
	}
	return &active, nil
}

func (r *ShiftRepo) ListStoreShifts(ctx context.Context, storeID uint) ([]*model.CashierShift, error) {
	var shifts []*model.CashierShift
	err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("started_at DESC").Find(&shifts).Error
	if err != nil {
		return nil, err
	}
	return shifts, nil
}

func (r *ShiftRepo) GetTrxStatsForShift(ctx context.Context, storeID, cashierID uint, startTime time.Time, endTime time.Time) (int64, int64, []model.TrxItem, []model.Trx, error) {
	var trxs []model.Trx
	query := r.db.WithContext(ctx).Where("store_id = ? AND cashier_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?", storeID, cashierID, startTime, endTime)
	if err := query.Find(&trxs).Error; err != nil {
		return 0, 0, nil, nil, err
	}

	var totalSales int64
	var trxCount int64 = int64(len(trxs))
	var allItems []model.TrxItem

	for _, t := range trxs {
		totalSales += t.Total
		for _, it := range t.Items {
			allItems = append(allItems, it)
		}
	}
	return totalSales, trxCount, allItems, trxs, nil
}
