package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MovementType string

const (
	MovementSale    MovementType = "sale"
	MovementRefund  MovementType = "refund"
	MovementAdjust  MovementType = "adjust"
	MovementInitial MovementType = "initial"
)

type Movement struct {
	ID          string       `gorm:"type:varchar(36);primaryKey" json:"id"`
	StoreID     string       `gorm:"type:varchar(36);not null;index" json:"-"`
	ProductID   string       `gorm:"type:varchar(36);not null;index" json:"product_id"`
	ProductName *string      `gorm:"-" json:"product_name"`
	Type        MovementType `gorm:"not null" json:"type"`
	Qty         int          `gorm:"not null" json:"qty"`
	Reason      string       `gorm:"not null;default:''" json:"reason"`
	Actor       string       `gorm:"not null;default:''" json:"actor"`
	CreatedAt   time.Time    `gorm:"not null" json:"created_at"`
}

func (Movement) TableName() string {
	return "stock_movements"
}

func (m *Movement) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return nil
}
