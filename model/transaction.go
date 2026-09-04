package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type PayMethod string

const (
	PayCash         PayMethod = "Cash"
	PayBankTransfer PayMethod = "Bank Transfer"
	PayQRIS         PayMethod = "QRIS"
	PayEWallet      PayMethod = "E-Wallet"
	PayCard         PayMethod = "Card"
)

func ValidPayMethod(m string) bool {
	switch PayMethod(m) {
	case PayCash, PayBankTransfer, PayQRIS, PayEWallet, PayCard:
		return true
	}
	return false
}

type TrxStatus string

const (
	TrxPending   TrxStatus = "pending"
	TrxCompleted TrxStatus = "completed"
	TrxCancelled TrxStatus = "cancelled"
	TrxRefunded  TrxStatus = "refunded"
)

type TrxItem struct {
	ProductID uint   `json:"product_id"`
	Name      string `json:"name"`
	BuyPrice  int64  `json:"buy_price"`
	Price     int64  `json:"price"`
	Qty       int    `json:"qty"`
}

type TrxItemList []TrxItem

func (l *TrxItemList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, okStr := value.(string)
		if !okStr {
			return errors.New("type assertion to []byte or string failed")
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, l)
}

func (l TrxItemList) Value() (driver.Value, error) {
	return json.Marshal(l)
}

type RefundItemList map[uint]int

func (m *RefundItemList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, okStr := value.(string)
		if !okStr {
			return errors.New("type assertion to []byte or string failed")
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, m)
}

func (m RefundItemList) Value() (driver.Value, error) {
	return json.Marshal(m)
}

type Trx struct {
	Model
	StoreID     uint        `gorm:"not null;index" json:"-"`
	CashierID   uint        `gorm:"not null;index" json:"-"`
	CashierName string      `gorm:"not null" json:"cashier_name"`
	Items       TrxItemList `gorm:"serializer:json" json:"items"`
	Subtotal    int64       `gorm:"not null;check:subtotal >= 0" json:"subtotal"`
	Discount    int64       `gorm:"not null;default:0;check:discount >= 0" json:"discount"`
	Tax         int64       `gorm:"not null;default:0;check:tax >= 0" json:"tax"`
	Total       int64       `gorm:"not null;check:total >= 0" json:"total"`
	Method      string      `gorm:"not null" json:"method"`
	Paid        int64       `gorm:"not null;check:paid >= 0" json:"paid"`
	Change      int64       `gorm:"not null;default:0;check:change >= 0" json:"change"`
	Status      TrxStatus   `gorm:"not null;default:'completed'" json:"status"`
	Customer    string      `gorm:"not null;default:''" json:"customer"`
}

func (Trx) TableName() string {
	return "transactions"
}

type TransactionItem struct {
	Model
	TrxID     uint   `gorm:"not null;index" json:"trx_id"`
	ProductID uint   `gorm:"not null" json:"product_id"`
	Name      string `gorm:"not null" json:"name"`
	BuyPrice  int64  `gorm:"not null" json:"buy_price"`
	Price     int64  `gorm:"not null" json:"price"`
	Qty       int    `gorm:"not null;check:qty > 0" json:"qty"`
}

func (TransactionItem) TableName() string {
	return "transaction_items"
}

type Refund struct {
	Model
	StoreID  uint           `gorm:"not null" json:"-"`
	TrxID    uint           `gorm:"not null;index" json:"trx_id"`
	Items    RefundItemList `gorm:"serializer:json" json:"-"`
	RawItems []TrxItem      `gorm:"-" json:"items"`
	Reason   string         `gorm:"not null" json:"reason"`
	ByName   string         `gorm:"not null" json:"by"`
}

func (Refund) TableName() string {
	return "refunds"
}
