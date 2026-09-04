package repo

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

var ErrStockInsufficient = errors.New("stok tidak cukup")

type ProductFilter struct {
	Q          string
	CategoryID string
	Active     *bool
	Page       int
	Limit      int
}

type ProductPage struct {
	Items []*model.Product `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

type ProductRepo struct {
	db *gorm.DB
}

func NewProductRepo(db *gorm.DB) *ProductRepo { return &ProductRepo{db: db} }

func (r *ProductRepo) GetByID(ctx context.Context, storeID, id string) (*model.Product, error) {
	var p model.Product
	var cat model.Category

	err := r.db.WithContext(ctx).Where("id = ? AND store_id = ?", id, storeID).First(&p).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	if p.CategoryID != nil {
		if err := r.db.WithContext(ctx).First(&cat, "id = ?", *p.CategoryID).Error; err == nil {
			p.CategoryName = &cat.Name
		}
	}
	return &p, nil
}

func (r *ProductRepo) List(ctx context.Context, storeID string, f ProductFilter) (*ProductPage, error) {
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	query := r.db.WithContext(ctx).Model(&model.Product{}).Where("store_id = ?", storeID)

	if q := strings.TrimSpace(f.Q); q != "" {
		searchTerm := "%" + strings.ToLower(q) + "%"
		query = query.Where("lower(name) LIKE ? OR lower(sku) LIKE ? OR lower(barcode) LIKE ?", searchTerm, searchTerm, searchTerm)
	}
	if f.CategoryID != "" {
		query = query.Where("category_id = ?", f.CategoryID)
	}
	if f.Active != nil {
		query = query.Where("active = ?", *f.Active)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var products = make([]*model.Product, 0)
	offset := (page - 1) * limit
	err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&products).Error
	if err != nil {
		return nil, err
	}

	// Attach category names
	for _, p := range products {
		if p.CategoryID != nil {
			var cat model.Category
			if err := r.db.WithContext(ctx).First(&cat, "id = ?", *p.CategoryID).Error; err == nil {
				p.CategoryName = &cat.Name
			}
		}
	}

	return &ProductPage{Items: products, Total: int(total), Page: page, Limit: limit}, nil
}

func (r *ProductRepo) Create(ctx context.Context, storeID string, p *model.Product) (*model.Product, error) {
	p.StoreID = storeID
	err := r.db.WithContext(ctx).Create(p).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return r.GetByID(ctx, storeID, p.ID)
}

func (r *ProductRepo) Update(ctx context.Context, storeID, id string, p *model.Product) (*model.Product, error) {
	res := r.db.WithContext(ctx).Model(&model.Product{}).Where("id = ? AND store_id = ?", id, storeID).Updates(map[string]interface{}{
		"category_id": p.CategoryID,
		"name":        p.Name,
		"sku":         p.SKU,
		"barcode":     p.Barcode,
		"buy_price":   p.BuyPrice,
		"sell_price":  p.SellPrice,
		"unit":        p.Unit,
	})
	if res.Error != nil {
		return nil, mapDBErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, storeID, id)
}

func (r *ProductRepo) SetActive(ctx context.Context, storeID, id string, active bool) error {
	res := r.db.WithContext(ctx).Model(&model.Product{}).Where("id = ? AND store_id = ?", id, storeID).Update("active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProductRepo) IsSkuTaken(ctx context.Context, storeID, sku, exceptProductID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Product{}).Where("store_id = ? AND lower(sku) = lower(?) AND id <> ?", storeID, strings.TrimSpace(sku), exceptProductID).Count(&count).Error
	return count > 0, err
}

func (r *ProductRepo) CreateWithInitialMovement(ctx context.Context, storeID string, p *model.Product, actor string) (*model.Product, error) {
	p.StoreID = storeID
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return mapDBErr(err)
		}
		if p.Stock > 0 {
			mv := model.Movement{
				StoreID:   storeID,
				ProductID: p.ID,
				Type:      model.MovementInitial,
				Qty:       p.Stock,
				Reason:    "Stok awal",
				Actor:     actor,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, storeID, p.ID)
}

func (r *ProductRepo) AdjustStock(ctx context.Context, storeID, productID string, delta int, reason, actor string) (*model.Product, error) {
	// Fixed TOCTOU: old code did First -> compute -> Save (race).
	// Now atomic: UPDATE ... SET stock = stock+delta WHERE stock+delta >= 0.
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Product{}).
			Where("id = ? AND store_id = ? AND stock + ? >= 0", productID, storeID, delta).
			Update("stock", gorm.Expr("stock + ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			var exists int64
			if err := tx.Model(&model.Product{}).
				Where("id = ? AND store_id = ?", productID, storeID).
				Count(&exists).Error; err != nil {
				return err
			}
			if exists > 0 {
				return ErrStockInsufficient
			}
			return ErrNotFound
		}

		mv := model.Movement{
			StoreID:   storeID,
			ProductID: productID,
			Type:      model.MovementAdjust,
			Qty:       delta,
			Reason:    reason,
			Actor:     actor,
		}
		return tx.Create(&mv).Error
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, storeID, productID)
}
