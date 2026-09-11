package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adrr-dev/openPOS/backend/middleware"
	"github.com/adrr-dev/openPOS/backend/service"
)

func formatID(id uint) string {
	return fmt.Sprintf("%d", id)
}

type UserHandler struct {
	svc      *service.UserService
	activity *service.ActivityService
}

func NewUserHandler(svc *service.UserService, activity ...*service.ActivityService) *UserHandler {
	h := &UserHandler{svc: svc}
	if len(activity) > 0 {
		h.activity = activity[0]
	}
	return h
}

func (h *UserHandler) logActivity(ctx context.Context, storeID uint, actorID, actorName, action, detail, refType, refID string) {
	if h.activity == nil {
		return
	}
	if len(detail) > 80 {
		detail = detail[:80]
	}
	h.activity.Log(ctx, storeID, actorID, actorName, action, detail, refType, refID)
}

type createUserReq struct {
	Name string `json:"name"`
}

type setActiveReq struct {
	Active bool `json:"active"`
}

type renameUserReq struct {
	Name string `json:"name"`
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
	h.logActivity(c.Request.Context(), claims.StoreID, formatID(claims.UserID), claims.Name, service.ActivityUserCreated, fmt.Sprintf("%s ditambahkan sebagai kasir", user.Name), "user", formatID(user.ID))
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (h *UserHandler) SetActive(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req setActiveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	// Resolve target name for activity detail (best-effort).
	targetName := fmt.Sprintf("Akun %d", id)
	if users, err := h.svc.List(c.Request.Context(), claims.StoreID); err == nil {
		for _, u := range users {
			if u.ID == id {
				targetName = u.Name
				break
			}
		}
	}
	if err := h.svc.SetActive(c.Request.Context(), claims.StoreID, id, req.Active); err != nil {
		respondUserErr(c, err)
		return
	}
	msg := "Akun diaktifkan."
	action := service.ActivityAccountEnabled
	detail := fmt.Sprintf("%s diaktifkan kembali", targetName)
	if !req.Active {
		msg = "Akun dinonaktifkan."
		action = service.ActivityAccountDisabled
		detail = fmt.Sprintf("%s dinonaktifkan", targetName)
	}
	h.logActivity(c.Request.Context(), claims.StoreID, formatID(claims.UserID), claims.Name, action, detail, "user", formatID(id))
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h *UserHandler) Delete(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), claims.StoreID, id); err != nil {
		respondUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Akun kasir berhasil dihapus."})
}

func (h *UserHandler) Rename(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var req renameUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body JSON tidak valid"})
		return
	}
	if err := h.svc.Rename(c.Request.Context(), claims.StoreID, id, req.Name); err != nil {
		respondUserErr(c, err)
		return
	}
	h.logActivity(c.Request.Context(), claims.StoreID, formatID(claims.UserID), claims.Name, service.ActivityProfileUpdated, fmt.Sprintf("%s memperbarui profil", req.Name), "user", formatID(id))
	c.JSON(http.StatusOK, gin.H{"message": "Nama kasir diperbarui."})
}

func respondUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Email sudah terdaftar."})
	case errors.Is(err, service.ErrStoreMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "Akun tidak ditemukan di toko Anda."})
	case errors.Is(err, service.ErrNotEditable):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya akun kasir yang dapat dinonaktifkan."})
	case errors.Is(err, service.ErrNotRenamable):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya akun kasir yang dapat diubah."})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
