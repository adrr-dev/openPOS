package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/0xMinomus/openPOS/backend/internal/middleware"
	"github.com/0xMinomus/openPOS/backend/internal/model"
	"github.com/0xMinomus/openPOS/backend/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler { return &AuthHandler{auth: auth} }

type registerReq struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	StoreName string `json:"storeName"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Passcode string `json:"passcode,omitempty"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	User model.PublicUser `json:"user"`
	model.TokenPair
}

// Register membuat Store + akun Admin sekaligus, lalu langsung login.
// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	user, pair, err := h.auth.Register(r.Context(), req.Name, req.Email, req.Password, req.StoreName)
	if err != nil {
		respondAuthErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{User: user.Public(), TokenPair: *pair})
}

// Login memverifikasi email + password (+ passcode bila akun dilindungi).
// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	user, pair, err := h.auth.Login(r.Context(), req.Email, req.Password, req.Passcode)
	if err != nil {
		respondAuthErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: user.Public(), TokenPair: *pair})
}

// Refresh menukar refresh token dengan pasangan token baru (rotasi).
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token wajib diisi")
		return
	}
	user, pair, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: user.Public(), TokenPair: *pair})
}

// Logout mencabut refresh token agar tidak bisa dipakai lagi.
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutReq
	_ = json.NewDecoder(r.Body).Decode(&req) // body opsional
	h.auth.Logout(r.Context(), req.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"message": "keluar berhasil"})
}

// Me mengembalikan profil user sesi aktif.
// GET /api/v1/auth/me  (Bearer access token)
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	user, err := h.auth.Me(r.Context(), c.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "sesi tidak valid, silakan masuk kembali")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user.Public()})
}

func respondAuthErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrPasscodeRequired):
		writeError(w, http.StatusUnauthorized, "passcode_required")
	case errors.Is(err, service.ErrPasscodeWrong):
		writeError(w, http.StatusUnauthorized, "Passcode salah. Coba lagi.")
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "Email atau kata sandi tidak cocok. Coba lagi.")
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, "Email sudah terdaftar. Silakan masuk.")
	case errors.Is(err, service.ErrAccountInactive):
		writeError(w, http.StatusForbidden, "Akun dinonaktifkan. Hubungi admin toko.")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
