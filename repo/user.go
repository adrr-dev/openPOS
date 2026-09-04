package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	var store model.Store

	err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	if err := r.db.WithContext(ctx).First(&store, "id = ?", u.StoreID).Error; err == nil {
		u.StoreName = store.Name
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var u model.User
	var store model.Store

	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	if err := r.db.WithContext(ctx).First(&store, "id = ?", u.StoreID).Error; err == nil {
		u.StoreName = store.Name
	}
	return &u, nil
}

func (r *UserRepo) ListByStore(ctx context.Context, storeID uint) ([]*model.User, error) {
	users := make([]*model.User, 0)
	var store model.Store
	if err := r.db.WithContext(ctx).First(&store, "id = ?", storeID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	storeName := store.Name

	err := r.db.WithContext(ctx).Where("store_id = ?", storeID).Order("created_at ASC").Find(&users).Error
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		u.StoreName = storeName
	}
	return users, nil
}

func (r *UserRepo) SetActive(ctx context.Context, id uint, active bool) error {
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) SetPasscode(ctx context.Context, id uint, hash *string) error {
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("passcode_hash", hash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) RegisterTx(ctx context.Context, storeName, email, name, passwordHash string) (*model.User, error) {
	var user *model.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := model.Store{Name: storeName}
		if err := tx.Create(&store).Error; err != nil {
			return mapDBErr(err)
		}

		now := time.Now()
		u := model.User{
			StoreID:         store.ID,
			Email:           email,
			Name:            name,
			PasswordHash:    passwordHash,
			Role:            model.RoleAdmin,
			Active:          true,
			EmailVerifiedAt: &now,
		}
		if err := tx.Create(&u).Error; err != nil {
			return mapDBErr(err)
		}
		u.StoreName = store.Name
		user = &u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}
