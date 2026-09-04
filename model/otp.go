package model

import (
	"time"

	"gorm.io/gorm"
)

type EmailOtp struct {
	Email      string     `gorm:"primaryKey" json:"email"`
	CodeHash   string     `gorm:"not null" json:"-"`
	ExpiresAt  time.Time  `gorm:"not null;index" json:"expires_at"`
	Attempts   int        `gorm:"not null;default:0" json:"attempts"`
	LastSentAt time.Time  `gorm:"not null" json:"last_sent_at"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CreatedAt  time.Time  `gorm:"not null" json:"created_at"`
}

func (EmailOtp) TableName() string {
	return "email_otps"
}

func (o *EmailOtp) BeforeCreate(tx *gorm.DB) error {
	if o.LastSentAt.IsZero() {
		o.LastSentAt = time.Now()
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	return nil
}
