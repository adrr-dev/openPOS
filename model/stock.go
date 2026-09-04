package model

type MovementType string

const (
	MovementSale    MovementType = "sale"
	MovementRefund  MovementType = "refund"
	MovementAdjust  MovementType = "adjust"
	MovementInitial MovementType = "initial"
)

type Movement struct {
	Model
	StoreID     uint         `gorm:"not null;index" json:"-"`
	ProductID   uint         `gorm:"not null;index" json:"product_id"`
	ProductName *string      `gorm:"-" json:"product_name"`
	Type        MovementType `gorm:"not null" json:"type"`
	Qty         int          `gorm:"not null" json:"qty"`
	Reason      string       `gorm:"not null;default:''" json:"reason"`
	Actor       string       `gorm:"not null;default:''" json:"actor"`
}

func (Movement) TableName() string {
	return "stock_movements"
}
