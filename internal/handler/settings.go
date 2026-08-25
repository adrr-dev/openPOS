package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/0xMinomus/openPOS/backend/internal/middleware"
	"github.com/0xMinomus/openPOS/backend/internal/model"
	"github.com/0xMinomus/openPOS/backend/internal/repo"
	"github.com/0xMinomus/openPOS/backend/internal/service"
)

type SettingsHandler struct {
	svc *service.SettingsService
}

func NewSettingsHandler(svc *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// Get — GET /api/v1/settings 🔒 (semua role; struk kasir butuh data ini)
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	s, err := h.svc.Get(r.Context(), c.StoreID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat pengaturan.")
		return
	}
	writeJSON(w, http.StatusOK, s)
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

// Update — PUT /api/v1/settings 🔒 admin
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req settingsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	in := &model.StoreSettings{
		Name: req.Name, Address: req.Address, Phone: req.Phone,
		TaxEnabled: req.TaxEnabled, TaxPct: req.TaxPct,
		ReceiptHeader: req.ReceiptHeader, ReceiptFooter: req.ReceiptFooter,
		Paper: req.Paper, Timezone: req.Timezone,
	}
	s, err := h.svc.Update(r.Context(), c.StoreID, in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadTimezone):
			writeError(w, http.StatusBadRequest, "Zona waktu tidak valid.")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, s)
}

type passcodeReq struct {
	Passcode string `json:"passcode"`
}

// SetPasscode — PUT /api/v1/users/{id}/passcode 🔒 admin
// Passcode kosong berarti menghapus passcode akun tersebut.
func (h *SettingsHandler) SetPasscode(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req passcodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	if err := h.svc.SetPasscode(r.Context(), c.StoreID, chi.URLParam(r, "id"), req.Passcode); err != nil {
		switch {
		case errors.Is(err, service.ErrStoreMismatch):
			writeError(w, http.StatusNotFound, "Akun tidak ditemukan di toko Anda.")
		case errors.Is(err, repo.ErrNotFound):
			writeError(w, http.StatusNotFound, "Akun tidak ditemukan di toko Anda.")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	msg := "Passcode disimpan."
	if req.Passcode == "" {
		msg = "Passcode dihapus."
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}

// Dashboard — GET /api/v1/dashboard 🔒 role-aware
func (h *SettingsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	data, err := h.svc.Dashboard(r.Context(), c.StoreID, c.UserID, c.Role != model.RoleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat dashboard.")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// Report — GET /api/v1/reports?period=today|yesterday|week|month|all 🔒 admin
func (h *SettingsHandler) Report(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	bundle, err := h.svc.Report(r.Context(), c.StoreID, r.URL.Query().Get("period"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBadPeriod), errors.Is(err, repo.ErrNotFound):
			writeError(w, http.StatusBadRequest, "Periode tidak valid.")
		default:
			writeError(w, http.StatusInternalServerError, "Gagal memuat laporan.")
		}
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}
