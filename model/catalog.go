package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Category struct {
	ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	StoreID   string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uq_category_store_name" json:"store_id"`
	Name      string    `gorm:"not null;uniqueIndex:uq_category_store_name" json:"name"`
	Active    bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (Category) TableName() string {
	return "categories"
}

func (c *Category) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	return nil
}

type Product struct {
	ID           string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	StoreID      string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uq_product_store_sku" json:"-"`
	CategoryID   *string   `gorm:"type:varchar(36);index" json:"category_id"`
	CategoryName *string   `gorm:"-" json:"category_name"`
	Name         string    `gorm:"not null" json:"name"`
	SKU          string    `gorm:"not null;uniqueIndex:uq_product_store_sku" json:"sku"`
	Barcode      string    `gorm:"not null;default:''" json:"barcode"`
	BuyPrice     int64     `gorm:"not null;default:0;check:buy_price >= 0" json:"buy_price"`
	SellPrice    int64     `gorm:"not null;default:0;check:sell_price >= 0" json:"sell_price"`
	Stock        int       `gorm:"not null;default:0;check:stock >= 0" json:"stock"`
	Unit         string    `gorm:"not null;default:'pcs'" json:"unit"`
	Active       bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
}

func (Product) TableName() string {
	return "products"
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	return nil
}
