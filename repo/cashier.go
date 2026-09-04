package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type CashierRepo struct {
	db *gorm.DB
}

func NewCashierRepo(db *gorm.DB) *CashierRepo { return &CashierRepo{db: db} }

func (r *CashierRepo) GetByID(ctx context.Context, id string) (*model.Cashier, error) {
	var c model.Cashier
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &c, nil
}

func (r *CashierRepo) ListByStore(ctx context.Context, storeID string) ([]*model.Cashier, error) {
	out := make([]*model.Cashier, 0)
	err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("created_at ASC").Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CashierRepo) Create(ctx context.Context, storeID, name string) (*model.Cashier, error) {
	c := model.Cashier{
		StoreID: storeID,
		Name:    name,
		Active:  true,
	}
	err := r.db.WithContext(ctx).Create(&c).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &c, nil
}

func (r *CashierRepo) SetActive(ctx context.Context, id string, active bool) error {
	res := r.db.WithContext(ctx).Model(&model.Cashier{}).Where("id = ?", id).Update("active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CashierRepo) SetPasscode(ctx context.Context, id string, hash *string) error {
	res := r.db.WithContext(ctx).Model(&model.Cashier{}).Where("id = ?", id).Update("passcode_hash", hash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CashierRepo) GetOrCreateDefault(ctx context.Context, storeID, defaultName string) (string, error) {
	var c model.Cashier
	err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("created_at ASC").First(&c).Error
	if err == nil {
		return c.ID, nil
	}
	created, err := r.Create(ctx, storeID, defaultName)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}
