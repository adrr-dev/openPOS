package model

import "time"

// StoreSettings adalah konfigurasi toko (DTO camelCase untuk klien).
type StoreSettings struct {
	Name          string  `json:"storeName"`
	Address       string  `json:"address"`
	Phone         string  `json:"phone"`
	TaxEnabled    bool    `json:"taxEnabled"`
	TaxPct        float64 `json:"taxPct"`
	ReceiptHeader string  `json:"receiptHeader"`
	ReceiptFooter string  `json:"receiptFooter"`
	Paper         string  `json:"paper"`
	Timezone      string  `json:"timezone"`
}

// ── dashboard ────────────────────────────────────────────────────────

type DayPoint struct {
	Date  string `json:"date"` // YYYY-MM-DD (zona waktu toko)
	Omzet int64  `json:"omzet"`
}

type MethodPoint struct {
	Method string `json:"method"`
	Total  int64  `json:"total"`
}

type TopProduct struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Qty       int    `json:"qty"`
	Revenue   int64  `json:"revenue"`
}

type TrxBrief struct {
	ID          string    `json:"id"`
	CashierName string    `json:"cashier_name"`
	Total       int64     `json:"total"`
	Status      TrxStatus `json:"status"`
	Time        time.Time `json:"time"`
}

type DashboardToday struct {
	Omzet     int64 `json:"omzet"`
	TrxCount  int64 `json:"trx_count"`
	ItemsSold int64 `json:"items_sold"`
	LowStock  int64 `json:"low_stock,omitempty"`
}

type DashboardAdmin struct {
	Role        string         `json:"role"`
	Today       DashboardToday `json:"today"`
	Sales7      []DayPoint     `json:"sales7"`
	Methods     []MethodPoint  `json:"methods"`
	TopProducts []TopProduct   `json:"top_products"`
	Recent      []TrxBrief     `json:"recent"`
}

type DashboardCashier struct {
	Role   string         `json:"role"`
	Today  DashboardToday `json:"today"`
	Recent []TrxBrief     `json:"recent"`
}

// ── laporan ──────────────────────────────────────────────────────────

type ReportSummary struct {
	Omzet       int64 `json:"omzet"`
	TrxCount    int64 `json:"trx_count"`
	ItemsSold   int64 `json:"items_sold"`
	GrossProfit int64 `json:"gross_profit"`
}

type ProductReportRow struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Qty       int    `json:"qty"`
	Revenue   int64  `json:"revenue"`
	Profit    int64  `json:"profit"`
}

type TrxProfitRow struct {
	Date    string    `json:"date"`
	ID      string    `json:"id"`
	Cashier string    `json:"cashier"`
	Method  string    `json:"method"`
	Total   int64     `json:"total"`
	HPP     int64     `json:"hpp"`
	Profit  int64     `json:"profit"`
	Status  TrxStatus `json:"status"`
}

type StockRow struct {
	Name       string `json:"name"`
	SKU        string `json:"sku"`
	Stock      int    `json:"stock"`
	BuyPrice   int64  `json:"buy_price"`
	SellPrice  int64  `json:"sell_price"`
	StockValue int64  `json:"stock_value"`
}

type StatusCount struct {
	Status TrxStatus `json:"status"`
	Count  int64     `json:"count"`
}

// ReportBundle: seluruh isi laporan untuk satu periode (dataset UMKM kecil).
type ReportBundle struct {
	Period       string             `json:"period"`
	Summary      ReportSummary      `json:"summary"`
	ByMethod     []MethodPoint      `json:"by_method"`
	ByStatus     []StatusCount      `json:"by_status"`
	Products     []ProductReportRow `json:"products"`
	Transactions []TrxProfitRow     `json:"transactions"`
	Stock        []StockRow         `json:"stock"`
}
