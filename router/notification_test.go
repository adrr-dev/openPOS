package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrr-dev/openPOS/backend/service"
)

func TestNotificationEndpoints(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", filepath.Join(t.TempDir(), "notifications_test.db"))
	t.Setenv("JWT_SECRET", "test-secret-notifications")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if srv.Cleanup != nil {
		defer srv.Cleanup()
	}

	doReq := func(method, path string, body any, token string) (*httptest.ResponseRecorder, map[string]any) {
		var buf []byte
		if body != nil {
			buf, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(buf))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		srv.Handler.ServeHTTP(w, req)
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		return w, m
	}

	email := "notif_" + time.Now().Format("20060102150405") + "@toko.com"
	var code string
	service.TestOnOTPSent = func(e, c string) {
		if e == email {
			code = c
		}
	}
	doReq("POST", "/auth/otp/send", map[string]string{"email": email}, "")
	if code == "" {
		t.Fatalf("OTP code not captured")
	}
	doReq("POST", "/auth/otp/verify", map[string]string{"email": email, "code": code}, "")

	_, regResp := doReq("POST", "/auth/register", map[string]string{
		"storeName": "Toko Notif",
		"email":     email,
		"name":      "Admin Notif",
		"password":  "password123",
	}, "")

	adminTok, ok := regResp["access_token"].(string)
	if !ok || adminTok == "" {
		t.Fatalf("failed to register user for token: %v", regResp)
	}

	// Test GET /notifications
	w, body := doReq("GET", "/notifications", nil, adminTok)
	if w.Code != 200 {
		t.Fatalf("GET /notifications failed: %d body=%s", w.Code, w.Body.String())
	}

	total, ok := body["total"].(float64)
	if !ok || total != 0 {
		t.Fatalf("expected total 0 notifications, got %v", body["total"])
	}

	// Test GET /notifications/unread-count
	w, bodyCount := doReq("GET", "/notifications/unread-count", nil, adminTok)
	if w.Code != 200 {
		t.Fatalf("GET /notifications/unread-count failed: %d", w.Code)
	}
	if bodyCount["total"].(float64) != 0 {
		t.Fatalf("expected unread count 0, got %v", bodyCount["total"])
	}

	// Create a cashier account to test 403 Forbidden for cashiers
	_, cashierResp := doReq("POST", "/users", map[string]any{
		"name":     "Kasir Test",
		"email":    "kasir_" + email,
		"password": "password123",
		"role":     "cashier",
	}, adminTok)

	cashierUser, ok := cashierResp["user"].(map[string]any)
	if !ok {
		t.Fatalf("failed to create cashier: %v", cashierResp)
	}
	cashierID := uint(cashierUser["id"].(float64))

	// Switch/login as cashier
	_, switchResp := doReq("POST", "/auth/switch", map[string]any{
		"target_user_id": cashierID,
	}, adminTok)
	cashierTok, ok := switchResp["access_token"].(string)
	if !ok || cashierTok == "" {
		t.Fatalf("failed to switch to cashier: %v", switchResp)
	}

	// Cashier accessing notifications should get 403 Forbidden
	wCashier, _ := doReq("GET", "/notifications", nil, cashierTok)
	if wCashier.Code != 403 {
		t.Fatalf("expected 403 Forbidden for cashier accessing /notifications, got %d", wCashier.Code)
	}

	t.Log("Notification endpoints tests passed successfully.")
}
