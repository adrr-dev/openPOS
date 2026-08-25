package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/0xMinomus/openPOS/backend/internal/middleware"
	"github.com/0xMinomus/openPOS/backend/internal/repo"
	"github.com/0xMinomus/openPOS/backend/internal/service"
)

type StockHandler struct {
	svc *service.CatalogService
}

func NewStockHandler(svc *service.CatalogService) *StockHandler { return &StockHandler{svc: svc} }

// ListMovements — GET /api/v1/movements?type=&productId=&page=&limit= 🔒 admin
func (h *StockHandler) ListMovements(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	q := r.URL.Query()

	f := repo.MovementFilter{Type: q.Get("type"), ProductID: q.Get("productId")}
	if v := q.Get("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}

	pageData, err := h.svc.ListMovements(r.Context(), c.StoreID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat riwayat pergerakan.")
		return
	}
	writeJSON(w, http.StatusOK, pageData)
}

type adjustReq struct {
	ProductID string `json:"productId"`
	Direction string `json:"direction"` // plus | minus
	Qty       int64  `json:"qty"`
	Reason    string `json:"reason"`
}

// AdjustStock — POST /api/v1/stock/adjustments 🔒 admin (FR-INV-002/003/006/007)
func (h *StockHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req adjustReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	p, err := h.svc.AdjustStock(r.Context(), c.StoreID, req.ProductID, c.Name, req.Direction, req.Qty, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadDirection):
			writeError(w, http.StatusBadRequest, "arah penyesuaian harus 'plus' atau 'minus'")
		case errors.Is(err, service.ErrStoreMismatch):
			writeError(w, http.StatusNotFound, "Produk tidak ditemukan.")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": p})
}
