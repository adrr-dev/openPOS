package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

	// Helper for executing HTTP requests
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

	// 1. Register Admin + Store
	regBody := map[string]string{
		"name":      "Admin Toko",
		"email":     "admin_" + time.Now().Format("20060102150405") + "@tokosaya.com",
		"password":  "password123",
		"storeName": "Toko Berkah",
	}
	w, resp := doReq("POST", "/auth/register", regBody, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("Register failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	token := resp["access_token"].(string)
	userObj := resp["user"].(map[string]any)
	storeID := userObj["store_id"].(string)
	if storeID == "" {
		t.Fatalf("store_id is empty")
	}

	// 2. GET /auth/me
	w, resp = doReq("GET", "/auth/me", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Me failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	// 3. POST /users (Create Cashier)
	createCashierBody := map[string]string{"name": "Andi Kasir"}
	w, resp = doReq("POST", "/users", createCashierBody, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateCashier failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	cashierUser := resp["user"].(map[string]any)
	cashierID := cashierUser["id"].(string)

	// 4. GET /users (List Admin + Cashiers)
	w, resp = doReq("GET", "/users", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("List users failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	usersList := resp["users"].([]any)
	if len(usersList) < 2 {
		t.Fatalf("Expected at least 2 users (admin + cashier), got %d", len(usersList))
	}

	// 5. PUT /users/{id}/passcode (Set passcode for cashier)
	w, resp = doReq("PUT", "/users/"+cashierID+"/passcode", map[string]string{"passcode": "12345"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("SetPasscode failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	// 6. POST /auth/switch (Switch to cashier with passcode)
	w, resp = doReq("POST", "/auth/switch", map[string]string{"target_user_id": cashierID, "passcode": "12345"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Switch to cashier failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	cashierToken := resp["access_token"].(string)

	// 7. Create Category & Product as Admin (using admin token)
	w, resp = doReq("POST", "/categories", map[string]string{"name": "Sembako"}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateCategory failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	catObj := resp["category"].(map[string]any)
	catID := catObj["id"].(string)

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
	prodID := prodObj["id"].(string)

	// 8. Checkout as Cashier
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
	trxID := resp["id"].(string)

	// 9. GET /transactions as Cashier
	w, resp = doReq("GET", "/transactions", nil, cashierToken)
	if w.Code != http.StatusOK {
		t.Fatalf("List transactions failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	// 10. Refund as Admin
	refundBody := map[string]any{
		"items": []any{
			map[string]any{"productId": prodID, "qty": 1},
		},
		"reason": "Salah beli",
	}
	w, resp = doReq("POST", "/transactions/"+trxID+"/refund", refundBody, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Refund failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	// 11. GET /dashboard and /reports as Admin
	w, resp = doReq("GET", "/dashboard", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Dashboard failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	w, resp = doReq("GET", "/reports?period=today", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Reports failed: code=%d, body=%s", w.Code, w.Body.String())
	}

	t.Log("Integration test passed successfully!")
}
