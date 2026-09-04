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
	TargetUserID uint   `json:"target_user_id"`
	Passcode     string `json:"passcode,omitempty"`
}

type authResponse struct {
	User model.PublicUser `json:"user"`
	model.TokenPair
}

func (h *AuthHandler) SendOTP(c *gin.Context) {
	var req otpSendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	err := h.auth.SendOTP(c.Request.Context(), req.Email)
	if err != nil {
		respondOTPErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Kode OTP terkirim ke email Anda."})
}

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req otpVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	err := h.auth.VerifyOTP(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		respondOTPErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"verified": true, "message": "Email berhasil diverifikasi."})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	user, pair, err := h.auth.Register(c.Request.Context(), req.Name, req.Email, req.Password, req.StoreName)
	if err != nil {
		respondAuthErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, authResponse{User: user.Public(), TokenPair: *pair})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	user, pair, err := h.auth.Login(c.Request.Context(), req.Email, req.Password, req.Passcode)
	if err != nil {
		respondAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, authResponse{User: user.Public(), TokenPair: *pair})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token wajib diisi"})
		return
	}
	user, pair, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authResponse{User: user.Public(), TokenPair: *pair})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutReq
	_ = c.ShouldBindJSON(&req)
	h.auth.Logout(c.Request.Context(), req.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"message": "keluar berhasil"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	pubUser, err := h.auth.Me(c.Request.Context(), claims)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sesi tidak valid, silakan masuk kembali"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": pubUser})
}

func (h *AuthHandler) Switch(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req switchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	if req.TargetUserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_user_id wajib diisi"})
		return
	}
	pubUser, pair, err := h.auth.Switch(c.Request.Context(), claims, req.TargetUserID, req.Passcode)
	if err != nil {
		respondSwitchErr(c, err)
		return
	}
	c.JSON(http.StatusOK, authResponse{User: *pubUser, TokenPair: *pair})
}

func respondSwitchErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSwitchSelf):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak dapat beralih ke akun sendiri."})
	case errors.Is(err, service.ErrPasscodeRequired):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "passcode_required"})
	case errors.Is(err, service.ErrPasscodeWrong):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Passcode salah. Coba lagi."})
	case errors.Is(err, service.ErrAccountInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": "Akun dinonaktifkan."})
	case errors.Is(err, repo.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Akun tidak ditemukan."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func respondOTPErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email tidak valid."})
	case errors.Is(err, service.ErrOtpCooldown):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu sering meminta kode. Coba lagi dalam 60 detik."})
	case errors.Is(err, service.ErrOtpWrong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kode OTP salah."})
	case errors.Is(err, service.ErrOtpExpired):
		c.JSON(http.StatusGone, gin.H{"error": "Kode OTP sudah kedaluwarsa. Kirim ulang."})
	case errors.Is(err, service.ErrOtpMaxAttempts):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu banyak percobaan. Kirim ulang kode OTP."})
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Email sudah terdaftar. Silakan masuk."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func respondAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPasscodeRequired):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "passcode_required"})
	case errors.Is(err, service.ErrPasscodeWrong):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Passcode salah. Coba lagi."})
	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau kata sandi tidak cocok. Coba lagi."})
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Email sudah terdaftar. Silakan masuk."})
	case errors.Is(err, service.ErrEmailNotVerified):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email belum diverifikasi. Silakan verifikasi kode OTP terlebih dahulu."})
	case errors.Is(err, service.ErrAccountInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": "Akun dinonaktifkan. Hubungi admin toko."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
