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

type TrxHandler struct {
	svc *service.TrxService
}

func NewTrxHandler(svc *service.TrxService) *TrxHandler { return &TrxHandler{svc: svc} }

type checkoutItemReq struct {
	ProductID uint `json:"productId"`
	Qty       int  `json:"qty"`
}

type checkoutReq struct {
	Items    []checkoutItemReq `json:"items"`
	Discount int64             `json:"discount"`
	Method   string            `json:"method"`
	Paid     int64             `json:"paid"`
	Customer string            `json:"customer"`
}

func (h *TrxHandler) Checkout(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req checkoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	cmd := service.CheckoutCmd{
		Discount: req.Discount, Method: req.Method, Paid: req.Paid, Customer: req.Customer,
	}
	for _, it := range req.Items {
		cmd.Items = append(cmd.Items, service.CheckoutItemCmd{ProductID: it.ProductID, Qty: it.Qty})
	}

	trx, err := h.svc.Checkout(c.Request.Context(), claims.StoreID, claims.ActingAsCashierID, claims.Name, cmd)
	if err != nil {
		respondTrxErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, trx)
}

func (h *TrxHandler) List(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	qp := c.Query

	var cashierID uint
	if claims.ActingAsCashierID != nil {
		cashierID = *claims.ActingAsCashierID
	}
	page, _ := strconv.Atoi(qp("page"))
	limit, _ := strconv.Atoi(qp("limit"))

	list, total, err := h.svc.List(c.Request.Context(), claims.StoreID, cashierID,
		qp("q"), qp("method"), qp("date"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat transaksi."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": total, "page": page, "limit": limit})
}

func (h *TrxHandler) Get(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	trx, err := h.svc.Get(c.Request.Context(), claims.StoreID, id)
	if err != nil {
		respondTrxErr(c, err)
		return
	}
	if claims.ActingAsCashierID != nil && trx.CashierID != *claims.ActingAsCashierID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan."})
		return
	}
	c.JSON(http.StatusOK, trx)
}

type refundReq struct {
	Items  []checkoutItemReq `json:"items"`
	Reason string            `json:"reason"`
}

func (h *TrxHandler) Refund(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req refundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	items := make(map[uint]int, len(req.Items))
	for _, it := range req.Items {
		items[it.ProductID] += it.Qty
	}
	trx, err := h.svc.Refund(c.Request.Context(), claims.StoreID, id, items, req.Reason, claims.Name)
	if err != nil {
		respondTrxErr(c, err)
		return
	}
	c.JSON(http.StatusOK, trx)
}

func respondTrxErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrEmptyItems):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keranjang kosong."})
	case errors.Is(err, repo.ErrStockInsufficient):
		c.JSON(http.StatusConflict, gin.H{"error": "Stok tidak cukup untuk menyelesaikan transaksi."})
	case errors.Is(err, repo.ErrPaidInsufficient):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jumlah bayar kurang dari total."})
	case errors.Is(err, repo.ErrBadDiscount):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Diskon melebihi subtotal."})
	case errors.Is(err, repo.ErrProductInactive):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ada produk yang tidak aktif."})
	case errors.Is(err, repo.ErrNotRefundable):
		c.JSON(http.StatusConflict, gin.H{"error": "Transaksi ini tidak dapat direfund."})
	case errors.Is(err, repo.ErrRefundTooMuch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Qty refund melebihi jumlah terjual."})
	case errors.Is(err, repo.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
