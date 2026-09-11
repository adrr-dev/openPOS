package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adrr-dev/openPOS/backend/middleware"
	"github.com/adrr-dev/openPOS/backend/model"
	"github.com/adrr-dev/openPOS/backend/repo"
	"github.com/adrr-dev/openPOS/backend/service"
)

type SettingsHandler struct {
	svc      *service.SettingsService
	activity *service.ActivityService
}

func NewSettingsHandler(svc *service.SettingsService, activity ...*service.ActivityService) *SettingsHandler {
	h := &SettingsHandler{svc: svc}
	if len(activity) > 0 {
		h.activity = activity[0]
	}
	return h
}

func (h *SettingsHandler) logActivity(ctx context.Context, storeID uint, actorID, actorName, action, detail, refType, refID string) {
	if h.activity == nil {
		return
	}
	if len(detail) > 80 {
		detail = detail[:80]
	}
	h.activity.Log(ctx, storeID, actorID, actorName, action, detail, refType, refID)
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
	Name                *string             `json:"storeName"`
	Address             *string             `json:"address"`
	Phone               *string             `json:"phone"`
	TaxEnabled          *bool               `json:"taxEnabled"`
	TaxPct              *float64            `json:"taxPct"`
	ReceiptHeader       *string             `json:"receiptHeader"`
	ReceiptFooter       *string             `json:"receiptFooter"`
	Paper               *string             `json:"paper"`
	Timezone            *string             `json:"timezone"`
	BusinessType        *string             `json:"businessType"`
	Email               *string             `json:"email"`
	City                *string             `json:"city"`
	Province            *string             `json:"province"`
	Currency            *string             `json:"currency"`
	Hours               *[]model.StoreHours `json:"hours"`
	ReceiptShowLogo     *bool               `json:"receiptShowLogo"`
	ReceiptShowCashier  *bool               `json:"receiptShowCashier"`
	ReceiptShowMethod   *bool               `json:"receiptShowMethod"`
	ReceiptShowTax      *bool               `json:"receiptShowTax"`
	ReceiptShowDiscount *bool               `json:"receiptShowDiscount"`
	ReceiptShowNote     *bool               `json:"receiptShowNote"`
	TaxName             *string             `json:"taxName"`
	TaxInclusive        *bool               `json:"taxInclusive"`
	TaxRounding         *string             `json:"taxRounding"`
	TaxApplyTo          *string             `json:"taxApplyTo"`
}

func (h *SettingsHandler) Update(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req settingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	// Merge-semantics: key yang absen = tidak diubah (body parsial tetap valid,
	// field tak dikenal otomatis diabaikan oleh binding).
	cur, err := h.svc.Get(c.Request.Context(), claims.StoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat pengaturan."})
		return
	}
	in := cur
	applyStr := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	applyBool := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	applyStr(&in.Name, req.Name)
	applyStr(&in.Address, req.Address)
	applyStr(&in.Phone, req.Phone)
	applyBool(&in.TaxEnabled, req.TaxEnabled)
	if req.TaxPct != nil {
		in.TaxPct = *req.TaxPct
	}
	applyStr(&in.ReceiptHeader, req.ReceiptHeader)
	applyStr(&in.ReceiptFooter, req.ReceiptFooter)
	applyStr(&in.Paper, req.Paper)
	applyStr(&in.Timezone, req.Timezone)
	applyStr(&in.BusinessType, req.BusinessType)
	applyStr(&in.Email, req.Email)
	applyStr(&in.City, req.City)
	applyStr(&in.Province, req.Province)
	applyStr(&in.Currency, req.Currency)
	if req.Hours != nil {
		in.Hours = *req.Hours
	}
	applyBool(&in.ReceiptShowLogo, req.ReceiptShowLogo)
	applyBool(&in.ReceiptShowCashier, req.ReceiptShowCashier)
	applyBool(&in.ReceiptShowMethod, req.ReceiptShowMethod)
	applyBool(&in.ReceiptShowTax, req.ReceiptShowTax)
	applyBool(&in.ReceiptShowDiscount, req.ReceiptShowDiscount)
	applyBool(&in.ReceiptShowNote, req.ReceiptShowNote)
	applyStr(&in.TaxName, req.TaxName)
	applyBool(&in.TaxInclusive, req.TaxInclusive)
	applyStr(&in.TaxRounding, req.TaxRounding)
	applyStr(&in.TaxApplyTo, req.TaxApplyTo)
	s, err := h.svc.Update(c.Request.Context(), claims.StoreID, in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadTimezone):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Zona waktu tidak valid."})
		case errors.Is(err, service.ErrBadBusinessType),
			errors.Is(err, service.ErrBadStoreEmail),
			errors.Is(err, service.ErrBadCurrency),
			errors.Is(err, service.ErrBadHours),
			errors.Is(err, service.ErrBadReceiptFooter),
			errors.Is(err, service.ErrBadTaxName),
			errors.Is(err, service.ErrBadTaxPct),
			errors.Is(err, service.ErrBadTaxRounding),
			errors.Is(err, service.ErrBadTaxApplyTo):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	h.logActivity(c.Request.Context(), claims.StoreID, formatID(claims.UserID), claims.Name, service.ActivityProfileUpdated, fmt.Sprintf("%s memperbarui profil", claims.Name), "", "")
	c.JSON(http.StatusOK, s)
}

type passcodeReq struct {
	Passcode string `json:"passcode"`
	// Role disambiguates owner vs cashier accounts sharing one number
	// ("admin"/"cashier", optional — legacy cashier-first order when empty).
	Role string `json:"role"`
}

func (h *SettingsHandler) SetPasscode(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req passcodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	if err := h.svc.SetPasscode(c.Request.Context(), claims.StoreID, id, req.Passcode, req.Role); err != nil {
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
	h.logActivity(c.Request.Context(), claims.StoreID, formatID(claims.UserID), claims.Name, service.ActivityPasscodeChanged, fmt.Sprintf("Passcode akun %d diperbarui", id), "user", formatID(id))
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h *SettingsHandler) Dashboard(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	// Cashiers must be scoped to their own cashier ID, not the owner ID.
	cashierID := claims.UserID
	if claims.ActingAsCashierID != nil {
		cashierID = *claims.ActingAsCashierID
	}
	data, err := h.svc.Dashboard(c.Request.Context(), claims.StoreID, cashierID, claims.Role != model.RoleAdmin)
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
