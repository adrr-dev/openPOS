package repo

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/0xMinomus/openPOS/backend/model"
)

type OtpRepo struct {
	db *gorm.DB
}

func NewOtpRepo(db *gorm.DB) *OtpRepo {
	return &OtpRepo{db: db}
}

func (r *OtpRepo) UpsertOTP(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	otp := model.EmailOtp{
		Email:      email,
		CodeHash:   codeHash,
		ExpiresAt:  expiresAt,
		Attempts:   0,
		LastSentAt: time.Now(),
		VerifiedAt: nil,
		CreatedAt:  time.Now(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "email"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"code_hash":    codeHash,
			"expires_at":   expiresAt,
			"attempts":     0,
			"last_sent_at": time.Now(),
			"verified_at":  nil,
			"created_at":   time.Now(),
		}),
	}).Create(&otp).Error
}

func (r *OtpRepo) GetOTP(ctx context.Context, email string) (*model.EmailOtp, error) {
	var o model.EmailOtp
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&o).Error
	if err != nil {
		return nil, mapDBErr(err)
	}
	return &o, nil
}

func (r *OtpRepo) IncrementAttempts(ctx context.Context, email string) (int, error) {
	var o model.EmailOtp
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("email = ?", email).First(&o).Error; err != nil {
			return err
		}
		o.Attempts++
		return tx.Save(&o).Error
	})
	if err != nil {
		return 0, mapDBErr(err)
	}
	return o.Attempts, nil
}

func (r *OtpRepo) MarkVerified(ctx context.Context, email string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&model.EmailOtp{}).Where("email = ?", email).Update("verified_at", &now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *OtpRepo) IsEmailVerified(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.EmailOtp{}).Where("email = ? AND verified_at IS NOT NULL", email).Count(&count).Error
	return count > 0, err
}

func (r *OtpRepo) RevokeOTP(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).Model(&model.EmailOtp{}).Where("email = ?", email).Updates(map[string]interface{}{
		"attempts":    3,
		"verified_at": nil,
	}).Error
}
