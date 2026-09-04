package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xMinomus/openPOS/backend/service"
)

func TestAPIIntegrationFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	srv, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if srv.Cleanup != nil {
		defer srv.Cleanup()
	}

	doReq := func(method, path string, body any, token string) (*httptest.ResponseRecorder, map[string]any) {
		var bodyBuf []byte
		if body != nil {
			bodyBuf, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(bodyBuf))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		srv.Handler.ServeHTTP(w, req)

		var respMap map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &respMap)
		return w, respMap
	}

	email := "admin_gin_" + time.Now().Format("20060102150405") + "@tokosaya.com"
	var lastCode string
	service.TestOnOTPSent = func(e, c string) {
		if e == email {
			lastCode = c
		}
	}

	w, _ := doReq("POST", "/auth/otp/send", map[string]string{"email": email}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("SendOTP failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	if lastCode == "" {
		t.Fatalf("OTP code was not captured")
	}

	w, _ = doReq("POST", "/auth/otp/verify", map[string]string{"email": email, "code": lastCode}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("VerifyOTP failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	regBody := map[string]string{
		"name":      "Admin Toko Gin",
		"email":     email,
		"password":  "password123",
		"storeName": "Toko Berkah Gin",
	}
	w, resp := doReq("POST", "/auth/register", regBody, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("Register failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	token := resp["access_token"].(string)
	userObj := resp["user"].(map[string]any)
	if userObj["store_id"].(float64) == 0 {
		t.Fatalf("store_id is empty")
	}

	w, _ = doReq("POST", "/auth/otp/send", map[string]string{"email": email}, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for registered email OTP send, got code=%d, body=%s", w.Code, w.Body.String())
	}

	w, resp = doReq("GET", "/auth/me", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Me failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	createCashierBody := map[string]string{"name": "Andi Kasir"}
	w, resp = doReq("POST", "/users", createCashierBody, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateCashier failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	cashierUser := resp["user"].(map[string]any)
	cashierID := uint(cashierUser["id"].(float64))

	w, resp = doReq("GET", "/users", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("List users failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	usersList := resp["users"].([]any)
	if len(usersList) < 2 {
		t.Fatalf("Expected at least 2 users (admin + cashier), got %d", len(usersList))
	}

	w, resp = doReq("PUT", fmt.Sprintf("/users/%d/passcode", cashierID), map[string]string{"passcode": "12345"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("SetPasscode failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	w, resp = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashierID, "passcode": "12345"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Switch to cashier failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	cashierToken := resp["access_token"].(string)

	w, resp = doReq("POST", "/categories", map[string]string{"name": "Sembako"}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateCategory failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	catObj := resp["category"].(map[string]any)
	catID := uint(catObj["id"].(float64))

	prodBody := map[string]any{
		"name":       "Beras 5kg",
		"sku":        "BRS-01",
		"categoryId": catID,
		"buyPrice":   50000,
		"sellPrice":  60000,
		"stock":      10,
		"unit":       "sak",
	}
	w, resp = doReq("POST", "/products", prodBody, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProduct failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	prodObj := resp
	prodID := uint(prodObj["id"].(float64))

	checkoutBody := map[string]any{
		"items": []any{
			map[string]any{"productId": prodID, "qty": 2},
		},
		"method":   "Cash",
		"paid":     150000,
		"customer": "Budi",
	}
	w, resp = doReq("POST", "/transactions", checkoutBody, cashierToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("Checkout failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	trxID := uint(resp["id"].(float64))

	w, resp = doReq("GET", "/transactions", nil, cashierToken)
	if w.Code != http.StatusOK {
		t.Fatalf("List transactions failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	refundBody := map[string]any{
		"items": []any{
			map[string]any{"productId": prodID, "qty": 1},
		},
		"reason": "Salah beli",
	}
	w, resp = doReq("POST", fmt.Sprintf("/transactions/%d/refund", trxID), refundBody, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Refund failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	w, resp = doReq("GET", "/dashboard", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Dashboard failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	w, resp = doReq("GET", "/reports?period=today", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Reports failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	t.Log("Gin + GORM integration test passed successfully!")
}
