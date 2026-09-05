package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/repo"
	"github.com/0xMinomus/openPOS/backend/service"
)

type CatalogHandler struct {
	svc *service.CatalogService
}

func NewCatalogHandler(svc *service.CatalogService) *CatalogHandler { return &CatalogHandler{svc: svc} }

type categoryReq struct {
	Name string `json:"name"`
}

func (h *CatalogHandler) ListCategories(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	cats, err := h.svc.ListCategories(c.Request.Context(), claims.StoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat kategori."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

func (h *CatalogHandler) CreateCategory(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	cat, err := h.svc.CreateCategory(c.Request.Context(), claims.StoreID, req.Name)
	if err != nil {
		respondCatalogErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"category": cat})
}

func (h *CatalogHandler) DeleteCategory(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	soft, err := h.svc.DeleteCategory(c.Request.Context(), claims.StoreID, id)
	if err != nil {
		respondCatalogErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"soft_deleted": soft})
}

type productReq struct {
	Name       string `json:"name"`
	SKU        string `json:"sku"`
	Barcode    string `json:"barcode"`
	CategoryID *uint  `json:"categoryId"`
	BuyPrice   *int64 `json:"buyPrice"`
	SellPrice  int64  `json:"sellPrice"`
	Stock      *int   `json:"stock"`
	Unit       string `json:"unit"`
}

func (h *CatalogHandler) ListProducts(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	q := c.Query

	f := repo.ProductFilter{Q: q("q"), CategoryID: queryID(q, "categoryId")}
	if v := q("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := q("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q("active"); v == "true" || v == "false" {
		b := v == "true"
		f.Active = &b
	}

	pageData, err := h.svc.ListProducts(c.Request.Context(), claims.StoreID, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat produk."})
		return
	}
	c.JSON(http.StatusOK, pageData)
}

func (h *CatalogHandler) GetProduct(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.GetProduct(c.Request.Context(), claims.StoreID, id)
	if err != nil {
		respondCatalogErr(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *CatalogHandler) CreateProduct(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	p, err := h.svc.CreateProduct(c.Request.Context(), claims.StoreID, claims.Name, productInputFrom(&req))
	if err != nil {
		respondCatalogErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *CatalogHandler) UpdateProduct(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	p, err := h.svc.UpdateProduct(c.Request.Context(), claims.StoreID, id, productInputFrom(&req))
	if err != nil {
		respondCatalogErr(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *CatalogHandler) SetProductActive(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	if err := h.svc.SetProductActive(c.Request.Context(), claims.StoreID, id, req.Active); err != nil {
		respondCatalogErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Status produk diperbarui."})
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

func respondCatalogErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSkuTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "SKU sudah digunakan di toko ini."})
	case errors.Is(err, service.ErrCategoryTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Kategori dengan nama itu sudah ada."})
	case errors.Is(err, service.ErrCategoryInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kategori tidak ditemukan di toko Anda."})
	case errors.Is(err, service.ErrStoreMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "Produk tidak ditemukan."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
