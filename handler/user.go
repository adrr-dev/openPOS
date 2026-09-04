package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

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

func (h *UserHandler) List(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	users, err := h.svc.List(c.Request.Context(), claims.StoreID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat daftar akun."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *UserHandler) Create(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	user, err := h.svc.CreateCashier(c.Request.Context(), claims.StoreID, req.Name)
	if err != nil {
		respondUserErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (h *UserHandler) SetActive(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	var req setActiveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	if err := h.svc.SetActive(c.Request.Context(), claims.StoreID, c.Param("id"), req.Active); err != nil {
		respondUserErr(c, err)
		return
	}
	msg := "Akun diaktifkan."
	if !req.Active {
		msg = "Akun dinonaktifkan."
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func respondUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Email sudah terdaftar."})
	case errors.Is(err, service.ErrStoreMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "Akun tidak ditemukan di toko Anda."})
	case errors.Is(err, service.ErrNotEditable):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya akun kasir yang dapat dinonaktifkan."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
