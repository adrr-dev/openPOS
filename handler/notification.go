package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/adrr-dev/openPOS/backend/middleware"
	"github.com/adrr-dev/openPOS/backend/service"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) List(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	q := c.Query

	page, _ := strconv.Atoi(q("page"))
	limit, _ := strconv.Atoi(q("limit"))
	unreadOnly := q("unread") == "true"

	pageData, err := h.svc.List(c.Request.Context(), claims.StoreID, page, limit, unreadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat notifikasi."})
		return
	}
	c.JSON(http.StatusOK, pageData)
}

func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}

	if err := h.svc.MarkAsRead(c.Request.Context(), claims.StoreID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notifikasi tidak ditemukan."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	if err := h.svc.MarkAllAsRead(c.Request.Context(), claims.StoreID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui notifikasi."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *NotificationHandler) Delete(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), claims.StoreID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notifikasi tidak ditemukan."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
