package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler { return &UserHandler{svc: svc} }

type createUserReq struct {
	Name string `json:"name"`
}

type setActiveReq struct {
	Active bool `json:"active"`
}

// List menampilkan seluruh akun dalam toko admin.
// GET /api/v1/users — 🔒 admin
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	users, err := h.svc.List(r.Context(), c.StoreID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Gagal memuat daftar akun.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// Create membuat akun kasir baru.
// POST /api/v1/users — 🔒 admin
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req createUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	user, err := h.svc.CreateCashier(r.Context(), c.StoreID, req.Name)
	if err != nil {
		respondUserErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

// SetActive mengaktifkan/menonaktifkan akun kasir.
// PATCH /api/v1/users/{id}/active — 🔒 admin
func (h *UserHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req setActiveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	if err := h.svc.SetActive(r.Context(), c.StoreID, chi.URLParam(r, "id"), req.Active); err != nil {
		respondUserErr(w, err)
		return
	}
	msg := "Akun diaktifkan."
	if !req.Active {
		msg = "Akun dinonaktifkan."
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}

func respondUserErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, "Email sudah terdaftar.")
	case errors.Is(err, service.ErrStoreMismatch):
		writeError(w, http.StatusNotFound, "Akun tidak ditemukan di toko Anda.")
	case errors.Is(err, service.ErrNotEditable):
		writeError(w, http.StatusBadRequest, "Hanya akun kasir yang dapat dinonaktifkan.")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
