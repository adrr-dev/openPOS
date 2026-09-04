package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler { return &HealthHandler{db: db} }

func (h *HealthHandler) Health(c *gin.Context) {
	status := "ok"
	dbStatus := "up"
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		status = "degraded"
		dbStatus = "down"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   status,
		"database": dbStatus,
		"service":  "openpos-backend",
	})
}
