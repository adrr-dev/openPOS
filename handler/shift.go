package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adrr-dev/openPOS/backend/middleware"
	"github.com/adrr-dev/openPOS/backend/service"
)

type ShiftHandler struct {
	svc *service.ShiftService
}

func NewShiftHandler(svc *service.ShiftService) *ShiftHandler {
	return &ShiftHandler{svc: svc}
}

type startShiftReq struct {
	OpeningCash int64 `json:"opening_cash"`
}

func (h *ShiftHandler) GetCurrentShift(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	data, err := h.svc.GetCashierShift(c.Request.Context(), claims.StoreID, claims.ActingAsCashierID, claims.Name)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveShift) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tidak ada shift yang berjalan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat shift."})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *ShiftHandler) StartShift(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req startShiftReq
	_ = c.ShouldBindJSON(&req)

	data, err := h.svc.StartShift(c.Request.Context(), claims.StoreID, claims.ActingAsCashierID, claims.Name, req.OpeningCash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, data)
}

func (h *ShiftHandler) CloseShift(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	data, err := h.svc.CloseShift(c.Request.Context(), claims.StoreID, claims.ActingAsCashierID, claims.Name)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveShift) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada shift yang berjalan."})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *ShiftHandler) ListShifts(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	data, err := h.svc.ListShifts(c.Request.Context(), claims.StoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar shift."})
		return
	}
	c.JSON(http.StatusOK, data)
}
