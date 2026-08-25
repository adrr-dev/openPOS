package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Memastikan pintu masuk serverless berfungsi: request /api/v1/health lewat
// Handler harus 200 dengan status database up (butuh .env + koneksi Supabase).
func TestHealthThroughHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	Handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"database":"up"`) {
		t.Fatalf("body tidak menunjukkan database up: %s", w.Body.String())
	}
}
