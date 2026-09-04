package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type MovementFilter struct {
	Type      string
	ProductID string
	Page      int
	Limit     int
}

type MovementPage struct {
	Items []*model.Movement `json:"items"`
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}

type MovementRepo struct {
	db *gorm.DB
}

func NewMovementRepo(db *gorm.DB) *MovementRepo { return &MovementRepo{db: db} }

func (r *MovementRepo) InsertTx(ctx context.Context, tx *gorm.DB, m *model.Movement) error {
	return tx.WithContext(ctx).Create(m).Error
}

func (r *MovementRepo) List(ctx context.Context, storeID string, f MovementFilter) (*MovementPage, error) {
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	query := r.db.WithContext(ctx).Model(&model.Movement{}).Where("store_id = ?", storeID)
	if f.Type != "" {
		query = query.Where("type = ?", f.Type)
	}
	if f.ProductID != "" {
		query = query.Where("product_id = ?", f.ProductID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var items = make([]*model.Movement, 0)
	offset := (page - 1) * limit
	err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&items).Error
	if err != nil {
		return nil, err
	}

	for _, m := range items {
		var prod model.Product
		if err := r.db.WithContext(ctx).First(&prod, "id = ?", m.ProductID).Error; err == nil {
			m.ProductName = &prod.Name
		}
	}

	return &MovementPage{Items: items, Total: int(total), Page: page, Limit: limit}, nil
}
