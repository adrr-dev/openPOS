package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/0xMinomus/openPOS/backend/model"
)

type RefreshRepo struct {
	db *gorm.DB
}

func NewRefreshRepo(db *gorm.DB) *RefreshRepo { return &RefreshRepo{db: db} }

func (r *RefreshRepo) Create(ctx context.Context, userID uint, tokenHash string, expiresAt time.Time) error {
	rt := model.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Revoked:   false,
	}
	return r.db.WithContext(ctx).Create(&rt).Error
}

func (r *RefreshRepo) GetActiveByHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&rt).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &rt, nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, tokenHash string) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).Where("token_hash = ?", tokenHash).Update("revoked", true).Error
}

func (r *RefreshRepo) RevokeAllForUser(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Model(&model.RefreshToken{}).Where("user_id = ?", userID).Update("revoked", true).Error
}
