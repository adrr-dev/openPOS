package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type CategoryRepo struct {
	db *gorm.DB
}

func NewCategoryRepo(db *gorm.DB) *CategoryRepo { return &CategoryRepo{db: db} }

func (r *CategoryRepo) ListByStore(ctx context.Context, storeID uint) ([]*model.Category, error) {
	out := make([]*model.Category, 0)
	err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("created_at ASC").Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CategoryRepo) GetByID(ctx context.Context, storeID, id uint) (*model.Category, error) {
	var c model.Category
	err := r.db.WithContext(ctx).Where("id = ? AND store_id = ?", id, storeID).First(&c).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &c, nil
}

func (r *CategoryRepo) Create(ctx context.Context, storeID uint, name string) (*model.Category, error) {
	c := model.Category{
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

func (r *CategoryRepo) Delete(ctx context.Context, storeID, id uint) (softDeleted bool, err error) {
	var c model.Category
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND store_id = ?", id, storeID).First(&c).Error; err != nil {
			if isNotFound(err) {
				return ErrNotFound
			}
			return err
		}

		var prodCount int64
		if err := tx.Model(&model.Product{}).Where("category_id = ? AND store_id = ?", id, storeID).Count(&prodCount).Error; err != nil {
			return err
		}

		if prodCount > 0 {
			softDeleted = true
			return tx.Model(&c).Update("active", false).Error
		}

		// gorm.Model soft-deletes by default; empty categories were hard
		// deleted before, so bypass the soft delete here.
		return tx.Unscoped().Delete(&c).Error
	})
	return softDeleted, mapDBErr(err)
}
