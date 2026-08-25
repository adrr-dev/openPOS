package model

import "time"

type Category struct {
	ID        string    `json:"id"`
	StoreID   string    `json:"store_id"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Product struct {
	ID           string    `json:"id"`
	StoreID      string    `json:"-"`
	CategoryID   *string   `json:"category_id"`
	CategoryName *string   `json:"category_name"`
	Name         string    `json:"name"`
	SKU          string    `json:"sku"`
	Barcode      string    `json:"barcode"`
	BuyPrice     int64     `json:"buy_price"`
	SellPrice    int64     `json:"sell_price"`
	Stock        int       `json:"stock"`
	Unit         string    `json:"unit"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}
