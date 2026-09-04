package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
	"github.com/0xMinomus/openPOS/backend/service"
)

type SettingsHandler struct {
	svc *service.SettingsService
}

func NewSettingsHandler(svc *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

func (h *SettingsHandler) Get(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	s, err := h.svc.Get(c.Request.Context(), claims.StoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat pengaturan."})
		return
	}
	c.JSON(http.StatusOK, s)
}

type settingsReq struct {
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

func (h *SettingsHandler) Update(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req settingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	in := &model.StoreSettings{
		Name: req.Name, Address: req.Address, Phone: req.Phone,
		TaxEnabled: req.TaxEnabled, TaxPct: req.TaxPct,
		ReceiptHeader: req.ReceiptHeader, ReceiptFooter: req.ReceiptFooter,
		Paper: req.Paper, Timezone: req.Timezone,
	}
	s, err := h.svc.Update(c.Request.Context(), claims.StoreID, in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadTimezone):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Zona waktu tidak valid."})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, s)
}

type passcodeReq struct {
	Passcode string `json:"passcode"`
}

func (h *SettingsHandler) SetPasscode(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req passcodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	if err := h.svc.SetPasscode(c.Request.Context(), claims.StoreID, c.Param("id"), req.Passcode); err != nil {
		switch {
		case errors.Is(err, service.ErrStoreMismatch), errors.Is(err, repo.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Akun tidak ditemukan di toko Anda."})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	msg := "Passcode disimpan."
	if req.Passcode == "" {
		msg = "Passcode dihapus."
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h *SettingsHandler) Dashboard(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	data, err := h.svc.Dashboard(c.Request.Context(), claims.StoreID, claims.UserID, claims.Role != model.RoleAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat dashboard."})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *SettingsHandler) Report(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	bundle, err := h.svc.Report(c.Request.Context(), claims.StoreID, c.Query("period"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadPeriod), errors.Is(err, repo.ErrNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Periode tidak valid."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat laporan."})
		}
		return
	}
	c.JSON(http.StatusOK, bundle)
}
