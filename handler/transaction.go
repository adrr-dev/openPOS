package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/repo"
	"github.com/0xMinomus/openPOS/backend/service"
)

type TrxHandler struct {
	svc *service.TrxService
}

func NewTrxHandler(svc *service.TrxService) *TrxHandler { return &TrxHandler{svc: svc} }

type checkoutItemReq struct {
	ProductID string `json:"productId"`
	Qty       int    `json:"qty"`
}

type checkoutReq struct {
	Items    []checkoutItemReq `json:"items"`
	Discount int64             `json:"discount"`
	Method   string            `json:"method"`
	Paid     int64             `json:"paid"`
	Customer string            `json:"customer"`
}

// Checkout — POST /api/v1/transactions 🔒 (admin & kasir)
// Harga & total dihitung ulang di server; stok berkurang atomik.
func (h *TrxHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req checkoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	cmd := service.CheckoutCmd{
		Discount: req.Discount, Method: req.Method, Paid: req.Paid, Customer: req.Customer,
	}
	for _, it := range req.Items {
		cmd.Items = append(cmd.Items, service.CheckoutItemCmd{ProductID: it.ProductID, Qty: it.Qty})
	}

	trx, err := h.svc.Checkout(r.Context(), c.StoreID, c.UserID, c.Name, cmd)
	if err != nil {
		respondTrxErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, trx)
}

// List — GET /api/v1/transactions?q=&method=&date=&page=&limit= 🔒
// Kasir otomatis hanya melihat transaksinya sendiri (FR-TRX-002).
func (h *TrxHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	qp := r.URL.Query()

	cashierID := ""
	if c.Role != "admin" {
		cashierID = c.UserID
	}
	page, _ := strconv.Atoi(qp.Get("page"))
	limit, _ := strconv.Atoi(qp.Get("limit"))

	list, total, err := h.svc.List(r.Context(), c.StoreID, cashierID,
		qp.Get("q"), qp.Get("method"), qp.Get("date"), page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat transaksi.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "total": total, "page": page, "limit": limit})
}

// Get — GET /api/v1/transactions/{id} 🔒
func (h *TrxHandler) Get(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	trx, err := h.svc.Get(r.Context(), c.StoreID, chi.URLParam(r, "id"))
	if err != nil {
		respondTrxErr(w, err)
		return
	}
	// kasir hanya boleh detail miliknya
	if c.Role != "admin" && trx.CashierID != c.UserID {
		writeError(w, http.StatusNotFound, "Transaksi tidak ditemukan.")
		return
	}
	writeJSON(w, http.StatusOK, trx)
}

type refundReq struct {
	Items  []checkoutItemReq `json:"items"`
	Reason string            `json:"reason"`
}

// Refund — POST /api/v1/transactions/{id}/refund 🔒 admin (FR-REF-001..005)
func (h *TrxHandler) Refund(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req refundReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	items := make(map[string]int, len(req.Items))
	for _, it := range req.Items {
		items[it.ProductID] += it.Qty
	}
	trx, err := h.svc.Refund(r.Context(), c.StoreID, chi.URLParam(r, "id"), items, req.Reason, c.Name)
	if err != nil {
		respondTrxErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trx)
}

func respondTrxErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repo.ErrEmptyItems):
		writeError(w, http.StatusBadRequest, "Keranjang kosong.")
	case errors.Is(err, repo.ErrStockInsufficient):
		writeError(w, http.StatusConflict, "Stok tidak cukup untuk menyelesaikan transaksi.")
	case errors.Is(err, repo.ErrPaidInsufficient):
		writeError(w, http.StatusBadRequest, "Jumlah bayar kurang dari total.")
	case errors.Is(err, repo.ErrBadDiscount):
		writeError(w, http.StatusBadRequest, "Diskon melebihi subtotal.")
	case errors.Is(err, repo.ErrProductInactive):
		writeError(w, http.StatusBadRequest, "Ada produk yang tidak aktif.")
	case errors.Is(err, repo.ErrNotRefundable):
		writeError(w, http.StatusConflict, "Transaksi ini tidak dapat direfund.")
	case errors.Is(err, repo.ErrRefundTooMuch):
		writeError(w, http.StatusBadRequest, "Qty refund melebihi jumlah terjual.")
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "Transaksi tidak ditemukan.")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
