package model

import "time"

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
	Name      string `json:"name"` // snapshot
	BuyPrice  int64  `json:"buy_price"`
	Price     int64  `json:"price"`
	Qty       int    `json:"qty"`
}

type Trx struct {
	ID          string    `json:"id"`
	Seq         int       `json:"seq"`
	CashierID   string    `json:"-"`
	CashierName string    `json:"cashier_name"`
	Items       []TrxItem `json:"items"`
	Subtotal    int64     `json:"subtotal"`
	Discount    int64     `json:"discount"`
	Tax         int64     `json:"tax"`
	Total       int64     `json:"total"`
	Method      string    `json:"method"`
	Paid        int64     `json:"paid"`
	Change      int64     `json:"change"`
	Status      TrxStatus `json:"status"`
	Customer    string    `json:"customer"`
	CreatedAt   time.Time `json:"time"`
}

type Refund struct {
	ID        string         `json:"id"`
	StoreID   string         `json:"-"`
	TrxID     string         `json:"trx_id"`
	Items     map[string]int `json:"-"` // productId -> qty (disimpan sebagai JSONB)
	RawItems  []TrxItem      `json:"items"`
	Reason    string         `json:"reason"`
	ByName    string         `json:"by"`
	CreatedAt time.Time      `json:"time"`
}
