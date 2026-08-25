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

type CatalogHandler struct {
	svc *service.CatalogService
}

func NewCatalogHandler(svc *service.CatalogService) *CatalogHandler { return &CatalogHandler{svc: svc} }

// ── kategori ─────────────────────────────────────────────────────────

type categoryReq struct {
	Name string `json:"name"`
}

// ListCategories — GET /api/v1/categories 🔒 (admin & kasir; POS butuh daftar ini)
func (h *CatalogHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	cats, err := h.svc.ListCategories(r.Context(), c.StoreID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat kategori.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}

// CreateCategory — POST /api/v1/categories 🔒 admin
func (h *CatalogHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req categoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	cat, err := h.svc.CreateCategory(r.Context(), c.StoreID, req.Name)
	if err != nil {
		respondCatalogErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": cat})
}

// DeleteCategory — DELETE /api/v1/categories/{id} 🔒 admin
// Masih dipakai produk → soft-delete (nonaktif). Response: {"soft_deleted": true|false}
func (h *CatalogHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	soft, err := h.svc.DeleteCategory(r.Context(), c.StoreID, chi.URLParam(r, "id"))
	if err != nil {
		respondCatalogErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"soft_deleted": soft})
}

// ── produk ───────────────────────────────────────────────────────────

type productReq struct {
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	Barcode    string  `json:"barcode"`
	CategoryID *string `json:"categoryId"`
	BuyPrice   *int64  `json:"buyPrice"`
	SellPrice  int64   `json:"sellPrice"`
	Stock      *int    `json:"stock"`
	Unit       string  `json:"unit"`
}

// ListProducts — GET /api/v1/products?q=&categoryId=&active=&page=&limit= 🔒 (admin & kasir; POS butuh)
func (h *CatalogHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	q := r.URL.Query()

	f := repo.ProductFilter{Q: q.Get("q"), CategoryID: q.Get("categoryId")}
	if v := q.Get("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("active"); v == "true" || v == "false" {
		b := v == "true"
		f.Active = &b
	}

	pageData, err := h.svc.ListProducts(r.Context(), c.StoreID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat produk.")
		return
	}
	writeJSON(w, http.StatusOK, pageData)
}

// GetProduct — GET /api/v1/products/{id} 🔒
func (h *CatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	p, err := h.svc.GetProduct(r.Context(), c.StoreID, chi.URLParam(r, "id"))
	if err != nil {
		respondCatalogErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// CreateProduct — POST /api/v1/products 🔒 admin
func (h *CatalogHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	req, ok := decodeProduct(w, r)
	if !ok {
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), c.StoreID, c.Name, productInputFrom(req))
	if err != nil {
		respondCatalogErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// UpdateProduct — PUT /api/v1/products/{id} 🔒 admin (stok tidak diubah di sini)
func (h *CatalogHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	req, ok := decodeProduct(w, r)
	if !ok {
		return
	}
	p, err := h.svc.UpdateProduct(r.Context(), c.StoreID, chi.URLParam(r, "id"), productInputFrom(req))
	if err != nil {
		respondCatalogErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// SetProductActive — PATCH /api/v1/products/{id}/active 🔒 admin
func (h *CatalogHandler) SetProductActive(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	if err := h.svc.SetProductActive(r.Context(), c.StoreID, chi.URLParam(r, "id"), req.Active); err != nil {
		respondCatalogErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Status produk diperbarui."})
}

// ── helper ───────────────────────────────────────────────────────────

func decodeProduct(w http.ResponseWriter, r *http.Request) (*productReq, bool) {
	var req productReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return nil, false
	}
	return &req, true
}

func productInputFrom(req *productReq) service.ProductInput {
	in := service.ProductInput{
		Name:       req.Name,
		SKU:        req.SKU,
		Barcode:    req.Barcode,
		CategoryID: req.CategoryID,
		SellPrice:  req.SellPrice,
		Unit:       req.Unit,
	}
	if req.BuyPrice != nil {
		in.BuyPrice = *req.BuyPrice
	}
	if req.Stock != nil {
		in.Stock = *req.Stock
	}
	return in
}

func respondCatalogErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrSkuTaken):
		writeError(w, http.StatusConflict, "SKU sudah digunakan di toko ini.")
	case errors.Is(err, service.ErrCategoryTaken):
		writeError(w, http.StatusConflict, "Kategori dengan nama itu sudah ada.")
	case errors.Is(err, service.ErrCategoryInvalid):
		writeError(w, http.StatusBadRequest, "Kategori tidak ditemukan di toko Anda.")
	case errors.Is(err, service.ErrStoreMismatch):
		writeError(w, http.StatusNotFound, "Produk tidak ditemukan.")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
