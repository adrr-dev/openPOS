package model

import (
	"time"
)

type CashierShift struct {
	Model
	StoreID     uint       `gorm:"not null;index" json:"store_id"`
	CashierID   uint       `gorm:"not null;index" json:"cashier_id"`
	CashierName string     `gorm:"not null" json:"cashier_name"`
	StartedAt   time.Time  `gorm:"not null" json:"started_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	OpeningCash int64      `gorm:"not null;default:0" json:"opening_cash"`
}

func (CashierShift) TableName() string {
	return "cashier_shifts"
}
