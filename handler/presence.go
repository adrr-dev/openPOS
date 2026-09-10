package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/adrr-dev/openPOS/backend/middleware"
	"github.com/adrr-dev/openPOS/backend/service"
)

type PresenceHandler struct {
	auth *service.AuthService
}

func NewPresenceHandler(auth *service.AuthService) *PresenceHandler {
	return &PresenceHandler{auth: auth}
}

func (h *PresenceHandler) Heartbeat(c *gin.Context) {
	claims := middleware.ClaimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tidak terotentikasi"})
		return
	}

	now := time.Now().UTC()
	if err := h.auth.RecordHeartbeat(c.Request.Context(), claims, now); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mencatat heartbeat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}