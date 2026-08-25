package model

import "time"

type MovementType string

const (
	MovementSale    MovementType = "sale"
	MovementRefund  MovementType = "refund"
	MovementAdjust  MovementType = "adjust"
	MovementInitial MovementType = "initial"
)

type Movement struct {
	ID          string       `json:"id"`
	StoreID     string       `json:"-"`
	ProductID   string       `json:"product_id"`
	ProductName *string      `json:"product_name"`
	Type        MovementType `json:"type"`
	Qty         int          `json:"qty"`
	Reason      string       `json:"reason"`
	Actor       string       `json:"actor"`
	CreatedAt   time.Time    `json:"created_at"`
}
