package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Smoke test jalur produksi: New() harus menghasilkan handler yang menjawab
// /api/v1/health dengan database up (butuh .env + koneksi Supabase).
func TestNewHealth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	srv, err := New(ctx)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if srv.Cleanup != nil {
		defer srv.Cleanup()
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/health", nil)
	srv.Handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"database":"up"`) {
		t.Fatalf("body tidak menunjukkan database up: %s", w.Body.String())
	}
}
