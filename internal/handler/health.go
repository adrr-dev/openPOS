package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	pool *pgxpool.Pool
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler { return &HealthHandler{pool: pool} }

// GET /api/v1/health — status layanan & koneksi database.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	db := "up"
	if err := h.pool.Ping(r.Context()); err != nil {
		status = "degraded"
		db = "down"
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   status,
		"database": db,
		"service":  "openpos-backend",
	})
}
