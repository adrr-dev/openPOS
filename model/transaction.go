package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	ProductID string `json:"product_id"`
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

type RefundItemList map[string]int

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
	ID          string      `gorm:"primaryKey" json:"id"`
	Seq         int         `gorm:"not null;uniqueIndex:uq_trx_store_seq" json:"seq"`
	StoreID     string      `gorm:"type:varchar(36);not null;index;uniqueIndex:uq_trx_store_seq" json:"-"`
	CashierID   string      `gorm:"type:varchar(36);not null;index" json:"-"`
	CashierName string      `gorm:"not null" json:"cashier_name"`
	Items       TrxItemList `gorm:"serializer:json;not null" json:"items"`
	Subtotal    int64       `gorm:"not null;check:subtotal >= 0" json:"subtotal"`
	Discount    int64       `gorm:"not null;default:0;check:discount >= 0" json:"discount"`
	Tax         int64       `gorm:"not null;default:0;check:tax >= 0" json:"tax"`
	Total       int64       `gorm:"not null;check:total >= 0" json:"total"`
	Method      string      `gorm:"not null" json:"method"`
	Paid        int64       `gorm:"not null;check:paid >= 0" json:"paid"`
	Change      int64       `gorm:"not null;default:0;check:change >= 0" json:"change"`
	Status      TrxStatus   `gorm:"not null;default:'completed'" json:"status"`
	Customer    string      `gorm:"not null;default:''" json:"customer"`
	CreatedAt   time.Time   `gorm:"not null" json:"time"`
}

func (Trx) TableName() string {
	return "transactions"
}

func (t *Trx) BeforeCreate(tx *gorm.DB) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	return nil
}

type TransactionItem struct {
	ID        string `gorm:"type:varchar(36);primaryKey" json:"id"`
	TrxID     string `gorm:"not null;index" json:"trx_id"`
	ProductID string `gorm:"type:varchar(36);not null" json:"product_id"`
	Name      string `gorm:"not null" json:"name"`
	BuyPrice  int64  `gorm:"not null" json:"buy_price"`
	Price     int64  `gorm:"not null" json:"price"`
	Qty       int    `gorm:"not null;check:qty > 0" json:"qty"`
}

func (TransactionItem) TableName() string {
	return "transaction_items"
}

func (ti *TransactionItem) BeforeCreate(tx *gorm.DB) error {
	if ti.ID == "" {
		ti.ID = uuid.NewString()
	}
	return nil
}

type Refund struct {
	ID        string         `gorm:"type:varchar(36);primaryKey" json:"id"`
	StoreID   string         `gorm:"type:varchar(36);not null" json:"-"`
	TrxID     string         `gorm:"not null;index" json:"trx_id"`
	Items     RefundItemList `gorm:"serializer:json;not null" json:"-"`
	RawItems  []TrxItem      `gorm:"-" json:"items"`
	Reason    string         `gorm:"not null" json:"reason"`
	ByName    string         `gorm:"not null" json:"by"`
	CreatedAt time.Time      `gorm:"not null" json:"time"`
}

func (Refund) TableName() string {
	return "refunds"
}

func (r *Refund) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	return nil
}
