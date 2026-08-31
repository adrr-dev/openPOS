package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
	"github.com/0xMinomus/openPOS/backend/service"
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

type otpSendReq struct {
	Email string `json:"email"`
}

type otpVerifyReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
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

type switchReq struct {
	TargetUserID string `json:"target_user_id"`
	Passcode     string `json:"passcode,omitempty"`
}

type authResponse struct {
	User model.PublicUser `json:"user"`
	model.TokenPair
}

// SendOTP mengirim kode OTP 6 digit ke email.
// POST /api/v1/auth/otp/send
func (h *AuthHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req otpSendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	err := h.auth.SendOTP(r.Context(), req.Email)
	if err != nil {
		respondOTPErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Kode OTP terkirim ke email Anda."})
}

// VerifyOTP memverifikasi kode OTP.
// POST /api/v1/auth/otp/verify
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req otpVerifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	err := h.auth.VerifyOTP(r.Context(), req.Email, req.Code)
	if err != nil {
		respondOTPErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "message": "Email berhasil diverifikasi."})
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
	pubUser, err := h.auth.Me(r.Context(), c)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "sesi tidak valid, silakan masuk kembali")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": pubUser})
}

// Switch beralih sesi ke akun lain dalam toko yang sama.
// POST /api/v1/auth/switch  (Bearer access token)
func (h *AuthHandler) Switch(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFrom(r.Context())
	var req switchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return
	}
	if req.TargetUserID == "" {
		writeError(w, http.StatusBadRequest, "target_user_id wajib diisi")
		return
	}
	pubUser, pair, err := h.auth.Switch(r.Context(), c, req.TargetUserID, req.Passcode)
	if err != nil {
		respondSwitchErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: *pubUser, TokenPair: *pair})
}

func respondSwitchErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrSwitchSelf):
		writeError(w, http.StatusBadRequest, "Tidak dapat beralih ke akun sendiri.")
	case errors.Is(err, service.ErrPasscodeRequired):
		writeError(w, http.StatusUnauthorized, "passcode_required")
	case errors.Is(err, service.ErrPasscodeWrong):
		writeError(w, http.StatusUnauthorized, "Passcode salah. Coba lagi.")
	case errors.Is(err, service.ErrAccountInactive):
		writeError(w, http.StatusForbidden, "Akun dinonaktifkan.")
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "Akun tidak ditemukan.")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func respondOTPErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "Email tidak valid.")
	case errors.Is(err, service.ErrOtpCooldown):
		writeError(w, http.StatusTooManyRequests, "Terlalu sering meminta kode. Coba lagi dalam 60 detik.")
	case errors.Is(err, service.ErrOtpWrong):
		writeError(w, http.StatusBadRequest, "Kode OTP salah.")
	case errors.Is(err, service.ErrOtpExpired):
		writeError(w, http.StatusGone, "Kode OTP sudah kedaluwarsa. Kirim ulang.")
	case errors.Is(err, service.ErrOtpMaxAttempts):
		writeError(w, http.StatusTooManyRequests, "Terlalu banyak percobaan. Kirim ulang kode OTP.")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
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
	case errors.Is(err, service.ErrEmailNotVerified):
		writeError(w, http.StatusBadRequest, "Email belum diverifikasi. Silakan verifikasi kode OTP terlebih dahulu.")
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
