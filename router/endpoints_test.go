package router

// Full endpoint sweep: every route exercised against sqlite (process env
// only — .env file untouched). READ-ONLY w.r.t. app code: asserts behavior,
// fixes nothing.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xMinomus/openPOS/backend/service"
)

func TestAllEndpoints(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", filepath.Join(t.TempDir(), "endpoints.db"))
	t.Setenv("JWT_SECRET", "test-secret-for-endpoint-sweep")

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
	raw := func(method, path string, body any, token string) string {
		w, _ := doReq(method, path, body, token)
		return w.Body.String()
	}
	expect := func(name string, w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("%s: want %d got %d body=%s", name, want, w.Code, w.Body.String())
		}
		t.Logf("ok %s -> %d", name, want)
	}
	num := func(m map[string]any, key string) uint {
		t.Helper()
		v, ok := m[key].(float64)
		if !ok {
			t.Fatalf("key %q is not a number: %v", key, m)
		}
		return uint(v)
	}

	// ── 1. health ──────────────────────────────────────────────
	w, _ := doReq("GET", "/health", nil, "")
	expect("GET /health", w, 200)
	if !strings.Contains(w.Body.String(), `"database":"up"`) {
		t.Fatalf("health db not up: %s", w.Body.String())
	}

	// ── 2-6. OTP ───────────────────────────────────────────────
	email := "sweep_" + time.Now().Format("20060102150405") + "@toko.com"
	w, _ = doReq("POST", "/auth/otp/send", map[string]string{"email": "bukan-email"}, "")
	expect("OTP send invalid email -> 400", w, 400)

	var code string
	service.TestOnOTPSent = func(e, c string) {
		if e == email {
			code = c
		}
	}
	w, _ = doReq("POST", "/auth/otp/send", map[string]string{"email": email}, "")
	expect("OTP send -> 200", w, 200)
	w, _ = doReq("POST", "/auth/otp/send", map[string]string{"email": email}, "")
	expect("OTP resend cooldown -> 429", w, 429)
	if code == "" {
		t.Fatalf("OTP code not captured")
	}
	w, _ = doReq("POST", "/auth/otp/verify", map[string]string{"email": email, "code": "000000"}, "")
	expect("OTP verify wrong -> 400", w, 400)
	w, _ = doReq("POST", "/auth/otp/verify", map[string]string{"email": email, "code": code}, "")
	expect("OTP verify -> 200", w, 200)

	// ── 7-9. register ──────────────────────────────────────────
	w, _ = doReq("POST", "/auth/register", map[string]string{"name": "A", "email": "x@toko.com", "password": "short", "storeName": "S"}, "")
	expect("register short pw -> 400", w, 400)
	w, _ = doReq("POST", "/auth/register", map[string]string{"name": "A", "email": "novalid@toko.com", "password": "password123", "storeName": "S"}, "")
	expect("register unverified -> 400", w, 400)
	w, resp := doReq("POST", "/auth/register", map[string]string{"name": "Admin", "email": email, "password": "password123", "storeName": "Toko Sweep"}, "")
	expect("register -> 201", w, 201)
	adminTok := resp["access_token"].(string)
	refresh1 := resp["refresh_token"].(string)
	adminUser := resp["user"].(map[string]any)
	if adminUser["role"] != "admin" || num(adminUser, "store_id") == 0 {
		t.Fatalf("register user shape: %v", adminUser)
	}
	w, _ = doReq("POST", "/auth/register", map[string]string{"name": "A", "email": email, "password": "password123", "storeName": "S"}, "")
	expect("register duplicate -> 409", w, 409)

	// ── 10. login ──────────────────────────────────────────────
	w, _ = doReq("POST", "/auth/login", map[string]string{"email": email, "password": "salah"}, "")
	expect("login wrong pw -> 401", w, 401)
	w, resp = doReq("POST", "/auth/login", map[string]string{"email": email, "password": "password123"}, "")
	expect("login -> 200", w, 200)
	adminTok = resp["access_token"].(string)

	// owner passcode gate
	w, _ = doReq("PUT", fmt.Sprintf("/users/%d/passcode", num(adminUser, "id")), map[string]string{"passcode": "54321"}, adminTok)
	expect("set owner passcode -> 200", w, 200)
	w, _ = doReq("POST", "/auth/login", map[string]string{"email": email, "password": "password123"}, "")
	if w.Code != 401 || !strings.Contains(w.Body.String(), "passcode_required") {
		t.Fatalf("login without passcode: want 401 passcode_required, got %d %s", w.Code, w.Body.String())
	}
	t.Log("ok login without passcode -> 401 passcode_required")
	w, _ = doReq("POST", "/auth/login", map[string]string{"email": email, "password": "password123", "passcode": "00000"}, "")
	expect("login wrong passcode -> 401", w, 401)
	w, resp = doReq("POST", "/auth/login", map[string]string{"email": email, "password": "password123", "passcode": "54321"}, "")
	expect("login with passcode -> 200", w, 200)
	adminTok = resp["access_token"].(string)
	w, _ = doReq("PUT", fmt.Sprintf("/users/%d/passcode", num(adminUser, "id")),
		map[string]string{"passcode": "", "role": "admin"}, adminTok)
	expect("clear owner passcode -> 200", w, 200)

	// ── 11. refresh / logout ───────────────────────────────────
	w, _ = doReq("POST", "/auth/refresh", map[string]string{}, "")
	expect("refresh empty -> 400", w, 400)
	w, resp = doReq("POST", "/auth/refresh", map[string]string{"refresh_token": refresh1}, "")
	expect("refresh -> 200", w, 200)
	refresh2 := resp["refresh_token"].(string)
	w, _ = doReq("POST", "/auth/refresh", map[string]string{"refresh_token": refresh1}, "")
	expect("refresh reuse rotated -> 401", w, 401)
	w, _ = doReq("POST", "/auth/logout", map[string]string{"refresh_token": refresh2}, "")
	expect("logout -> 200", w, 200)
	w, _ = doReq("POST", "/auth/refresh", map[string]string{"refresh_token": refresh2}, "")
	expect("refresh after logout -> 401", w, 401)

	// ── 12. me ─────────────────────────────────────────────────
	w, _ = doReq("GET", "/auth/me", nil, "")
	expect("me no token -> 401", w, 401)
	w, resp = doReq("GET", "/auth/me", nil, adminTok)
	expect("me -> 200", w, 200)
	if resp["user"].(map[string]any)["role"] != "admin" {
		t.Fatalf("me role: %v", resp)
	}

	// ── 13-14. users ───────────────────────────────────────────
	w, _ = doReq("POST", "/users", map[string]string{"name": "  "}, adminTok)
	expect("create cashier empty -> 400", w, 400)
	w, resp = doReq("POST", "/users", map[string]string{"name": "Kasir1"}, adminTok)
	expect("create cashier -> 201", w, 201)
	cashier1 := num(resp["user"].(map[string]any), "id")
	w, resp = doReq("POST", "/users", map[string]string{"name": "Kasir2"}, adminTok)
	expect("create cashier2 -> 201", w, 201)
	cashier2 := num(resp["user"].(map[string]any), "id")
	w, resp = doReq("GET", "/users", nil, adminTok)
	expect("list users -> 200", w, 200)
	if len(resp["users"].([]any)) != 3 {
		t.Fatalf("users len: %v", resp)
	}

	// passcode
	w, _ = doReq("PUT", "/users/99999/passcode", map[string]string{"passcode": "11111"}, adminTok)
	expect("passcode missing -> 404", w, 404)
	w, _ = doReq("PUT", fmt.Sprintf("/users/%d/passcode", cashier1), map[string]string{"passcode": "abc"}, adminTok)
	expect("passcode bad format -> 400", w, 400)
	w, _ = doReq("PUT", fmt.Sprintf("/users/%d/passcode", cashier1), map[string]string{"passcode": "11111"}, adminTok)
	expect("set cashier passcode -> 200", w, 200)

	// ── 15. switch ─────────────────────────────────────────────
	w, _ = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashier1}, adminTok)
	expect("switch no passcode -> 401", w, 401)
	w, _ = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashier1, "passcode": "00000"}, adminTok)
	expect("switch wrong passcode -> 401", w, 401)
	w, resp = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashier1, "passcode": "11111"}, adminTok)
	expect("switch to cashier -> 200", w, 200)
	kTok := resp["access_token"].(string)
	if resp["user"].(map[string]any)["role"] != "cashier" {
		t.Fatalf("switch role: %v", resp)
	}
	w, _ = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashier1, "passcode": "11111"}, kTok)
	expect("switch to self -> 400", w, 400)
	w, _ = doReq("POST", "/auth/switch", map[string]any{"target_user_id": 99999}, adminTok)
	expect("switch missing -> 404", w, 404)

	// ── 16. cashier RBAC ───────────────────────────────────────
	for _, tc := range []struct{ m, p string }{
		{"POST", "/users"}, {"GET", "/users"}, {"GET", "/reports"},
		{"PUT", "/settings"}, {"POST", "/products"}, {"GET", "/movements"},
	} {
		w, _ = doReq(tc.m, tc.p, map[string]string{"name": "x"}, kTok)
		if w.Code != 403 {
			t.Fatalf("%s %s as cashier: want 403 got %d %s", tc.m, tc.p, w.Code, w.Body.String())
		}
	}
	t.Log("ok cashier admin-only routes -> 403")
	w, _ = doReq("GET", "/categories", nil, kTok)
	expect("cashier list categories -> 200", w, 200)
	w, _ = doReq("GET", "/settings", nil, kTok)
	expect("cashier get settings -> 200", w, 200)
	w, resp = doReq("GET", "/dashboard", nil, kTok)
	expect("cashier dashboard -> 200", w, 200)
	if resp["role"] != "cashier" {
		t.Fatalf("cashier dashboard role: %v", resp)
	}

	// ── 17. active toggle ──────────────────────────────────────
	w, _ = doReq("PATCH", fmt.Sprintf("/users/%d/active", cashier2), map[string]bool{"active": false}, adminTok)
	expect("deactivate -> 200", w, 200)
	w, _ = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashier2}, adminTok)
	expect("switch to inactive -> 403", w, 403)
	w, _ = doReq("PATCH", fmt.Sprintf("/users/%d/active", cashier2), map[string]bool{"active": true}, adminTok)
	expect("reactivate -> 200", w, 200)

	// ── 18-19. categories ──────────────────────────────────────
	w, resp = doReq("POST", "/categories", map[string]string{"name": "Minuman"}, adminTok)
	expect("create category -> 201", w, 201)
	cat1 := num(resp["category"].(map[string]any), "id")
	w, _ = doReq("POST", "/categories", map[string]string{"name": "Minuman"}, adminTok)
	expect("duplicate category -> 409", w, 409)
	w, resp = doReq("POST", "/categories", map[string]string{"name": "Makanan"}, adminTok)
	expect("create category2 -> 201", w, 201)
	cat2 := num(resp["category"].(map[string]any), "id")
	w, resp = doReq("POST", "/categories", map[string]string{"name": "Kosong"}, adminTok)
	expect("create category3 -> 201", w, 201)
	cat3 := num(resp["category"].(map[string]any), "id")

	// ── 20-22. products ────────────────────────────────────────
	w, _ = doReq("POST", "/products", map[string]any{"name": "X", "sku": "X-1", "categoryId": 99999, "sellPrice": 1000}, adminTok)
	expect("product bad category -> 400", w, 400)
	w, resp = doReq("POST", "/products", map[string]any{"name": "Teh", "sku": "TEH-1", "categoryId": cat1, "buyPrice": 5000, "sellPrice": 8000, "stock": 10, "unit": "pcs"}, adminTok)
	expect("create product -> 201", w, 201)
	prod1 := num(resp, "id")
	if num(resp, "stock") != 10 {
		t.Fatalf("product stock: %v", resp)
	}
	w, _ = doReq("POST", "/products", map[string]any{"name": "Teh2", "sku": "TEH-1", "sellPrice": 8000}, adminTok)
	expect("duplicate SKU -> 409", w, 409)
	w, resp = doReq("POST", "/products", map[string]any{"name": "Kopi", "sku": "KPI-1", "sellPrice": 12000, "stock": 20}, adminTok)
	expect("create product no category -> 201", w, 201)
	prod2 := num(resp, "id")
	w, resp = doReq("POST", "/products", map[string]any{"name": "Roti", "sku": "RTI-1", "categoryId": cat2, "sellPrice": 10000, "stock": 5}, adminTok)
	expect("create product3 -> 201", w, 201)
	prod3 := num(resp, "id")

	w, resp = doReq("GET", "/products?q=teh", nil, adminTok)
	expect("list products q -> 200", w, 200)
	if int(resp["total"].(float64)) != 1 {
		t.Fatalf("search q: %v", resp)
	}
	w, resp = doReq("GET", fmt.Sprintf("/products?categoryId=%d", cat1), nil, adminTok)
	expect("list products category -> 200", w, 200)
	if int(resp["total"].(float64)) != 1 {
		t.Fatalf("category filter: %v", resp)
	}
	w, resp = doReq("GET", "/products?q=zzz-no-match", nil, adminTok)
	expect("list products empty -> 200", w, 200)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("empty items not []: %s", w.Body.String())
	}
	w, resp = doReq("GET", "/products?page=1&limit=2", nil, adminTok)
	expect("list products paged -> 200", w, 200)
	if len(resp["items"].([]any)) != 2 || int(resp["total"].(float64)) != 3 {
		t.Fatalf("pagination: %v", resp)
	}

	w, _ = doReq("GET", "/products/99999", nil, adminTok)
	expect("get product missing -> 404", w, 404)
	w, resp = doReq("GET", fmt.Sprintf("/products/%d", prod1), nil, adminTok)
	expect("get product -> 200", w, 200)
	if resp["category_name"] != "Minuman" {
		t.Fatalf("category_name: %v", resp)
	}
	w, _ = doReq("PUT", fmt.Sprintf("/products/%d", prod1), map[string]any{"name": "Teh", "sku": "KPI-1", "sellPrice": 8000}, adminTok)
	expect("update SKU taken -> 409", w, 409)
	w, resp = doReq("PUT", fmt.Sprintf("/products/%d", prod1), map[string]any{"name": "Teh Manis", "sku": "TEH-1", "categoryId": cat1, "sellPrice": 9000}, adminTok)
	expect("update product -> 200", w, 200)
	if resp["name"] != "Teh Manis" {
		t.Fatalf("update name: %v", resp)
	}
	w, _ = doReq("PATCH", fmt.Sprintf("/products/%d/active", prod3), map[string]bool{"active": false}, adminTok)
	expect("deactivate product -> 200", w, 200)
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{map[string]any{"productId": prod3, "qty": 1}}, "method": "Cash", "paid": 99999}, adminTok)
	expect("checkout inactive -> 400", w, 400)
	w, _ = doReq("PATCH", fmt.Sprintf("/products/%d/active", prod3), map[string]bool{"active": true}, adminTok)
	expect("reactivate product -> 200", w, 200)
	w, resp = doReq("GET", "/products?active=false", nil, adminTok)
	expect("list inactive -> 200", w, 200)
	if int(resp["total"].(float64)) != 0 {
		t.Fatalf("inactive filter: %v", resp)
	}

	// ── 23. stock ──────────────────────────────────────────────
	w, resp = doReq("POST", "/stock/adjustments", map[string]any{"productId": prod1, "direction": "minus", "qty": 3, "reason": "rusak"}, adminTok)
	expect("adjust minus -> 200", w, 200)
	if num(resp["product"].(map[string]any), "stock") != 7 {
		t.Fatalf("stock after -3: %v", resp)
	}
	w, _ = doReq("POST", "/stock/adjustments", map[string]any{"productId": prod1, "direction": "minus", "qty": 100, "reason": "x"}, adminTok)
	expect("adjust negative -> 400", w, 400)
	w, _ = doReq("POST", "/stock/adjustments", map[string]any{"productId": prod1, "direction": "sideways", "qty": 1, "reason": "x"}, adminTok)
	expect("adjust bad direction -> 400", w, 400)
	w, _ = doReq("POST", "/stock/adjustments", map[string]any{"productId": prod1, "direction": "plus", "qty": 0, "reason": "x"}, adminTok)
	expect("adjust qty 0 -> 400", w, 400)
	w, _ = doReq("POST", "/stock/adjustments", map[string]any{"productId": prod1, "direction": "plus", "qty": 1, "reason": ""}, adminTok)
	expect("adjust no reason -> 400", w, 400)
	w, resp = doReq("POST", "/stock/adjustments", map[string]any{"productId": prod1, "direction": "plus", "qty": 5, "reason": "restock"}, adminTok)
	expect("adjust plus -> 200", w, 200)
	if num(resp["product"].(map[string]any), "stock") != 12 {
		t.Fatalf("stock after +5: %v", resp)
	}
	w, _ = doReq("POST", "/stock/adjustments", map[string]any{"productId": 99999, "direction": "plus", "qty": 1, "reason": "x"}, adminTok)
	expect("adjust missing product -> 404", w, 404)

	w, resp = doReq("GET", "/movements", nil, adminTok)
	expect("list movements -> 200", w, 200)
	if int(resp["total"].(float64)) < 4 { // 3x initial + 2x adjust
		t.Fatalf("movements total: %v", resp)
	}
	w, resp = doReq("GET", fmt.Sprintf("/movements?type=adjust&productId=%d", prod1), nil, adminTok)
	expect("movements filtered -> 200", w, 200)
	if int(resp["total"].(float64)) != 2 {
		t.Fatalf("movements filter: %v", resp)
	}
	if body := raw("GET", "/movements?type=nope", nil, adminTok); !strings.Contains(body, `"items":[]`) {
		t.Fatalf("movements empty not []: %s", body)
	}

	// ── 24. checkout errors ────────────────────────────────────
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{}, "method": "Cash", "paid": 100}, adminTok)
	expect("checkout empty -> 400", w, 400)
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{map[string]any{"productId": prod1, "qty": 1}}, "method": "Emas", "paid": 99999}, adminTok)
	expect("checkout bad method -> 400", w, 400)
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{map[string]any{"productId": prod1, "qty": 0}}, "method": "Cash", "paid": 99999}, adminTok)
	expect("checkout qty 0 -> 400", w, 400)
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{map[string]any{"productId": prod1, "qty": 1}}, "discount": 999999, "method": "Cash", "paid": 999999}, adminTok)
	expect("checkout discount -> 400", w, 400)
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{map[string]any{"productId": prod1, "qty": 1}}, "method": "Cash", "paid": 1}, adminTok)
	expect("checkout underpaid -> 400", w, 400)
	w, _ = doReq("POST", "/transactions", map[string]any{"items": []any{map[string]any{"productId": prod1, "qty": 999}}, "method": "Cash", "paid": 99999999}, adminTok)
	expect("checkout overstock -> 409", w, 409)

	// ── 25. checkout ok (cashier) ──────────────────────────────
	// P1: sell 9000 (updated), P2: sell 12000. subtotal=2*9000+12000=30000,
	// discount 1000 -> total 29000, paid 30000 -> change 1000.
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items":    []any{map[string]any{"productId": prod1, "qty": 2}, map[string]any{"productId": prod2, "qty": 1}},
		"discount": 1000, "method": "Cash", "paid": 30000, "customer": "Budi",
	}, kTok)
	expect("checkout cash -> 201", w, 201)
	trx1 := num(resp, "id")
	for k, v := range map[string]float64{"subtotal": 30000, "discount": 1000, "total": 29000, "paid": 30000, "change": 1000} {
		if resp[k].(float64) != v {
			t.Fatalf("checkout math %s: want %v got %v", k, v, resp)
		}
	}
	if resp["status"] != "completed" || resp["cashier_name"] != "Kasir1" {
		t.Fatalf("checkout meta: %v", resp)
	}
	w, resp = doReq("GET", fmt.Sprintf("/products/%d", prod1), nil, adminTok)
	if num(resp, "stock") != 10 { // 12 - 2
		t.Fatalf("stock after checkout: %v", resp)
	}

	// non-cash: paid forced to total
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items": []any{map[string]any{"productId": prod2, "qty": 1}}, "method": "QRIS", "paid": 0,
	}, adminTok)
	expect("checkout QRIS -> 201", w, 201)
	if resp["paid"].(float64) != resp["total"].(float64) {
		t.Fatalf("non-cash paid: %v", resp)
	}

	// ── 26. trx list/get ───────────────────────────────────────
	w, resp = doReq("GET", "/transactions", nil, adminTok)
	expect("list trx admin -> 200", w, 200)
	if int(resp["total"].(float64)) != 2 {
		t.Fatalf("trx total admin: %v", resp)
	}
	w, resp = doReq("GET", "/transactions", nil, kTok)
	expect("list trx cashier -> 200", w, 200)
	// QUIRK (matches old backend, not a bug): admin checkouts without
	// acting-as attach to the FIRST cashier via GetOrCreateDefault, so the
	// QRIS trx above belongs to Kasir1 too -> total 2, not 1.
	if int(resp["total"].(float64)) != 2 {
		t.Fatalf("trx cashier list: %v", resp)
	}
	// True isolation: Kasir2's own checkout must be invisible to Kasir1.
	w, resp = doReq("POST", "/auth/switch", map[string]any{"target_user_id": cashier2}, adminTok)
	expect("switch to cashier2 -> 200", w, 200)
	kTok2 := resp["access_token"].(string)
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items": []any{map[string]any{"productId": prod2, "qty": 1}}, "method": "Cash", "paid": 20000,
	}, kTok2)
	expect("checkout as cashier2 -> 201", w, 201)
	trxK2 := num(resp, "id")
	w, resp = doReq("GET", "/transactions", nil, kTok)
	expect("list trx cashier1 again -> 200", w, 200)
	if int(resp["total"].(float64)) != 2 {
		t.Fatalf("trx cashier1 isolation: %v", resp)
	}
	w, _ = doReq("GET", fmt.Sprintf("/transactions/%d", trxK2), nil, kTok)
	expect("cashier1 reads cashier2 trx -> 404", w, 404)
	w, resp = doReq("GET", "/transactions?method=QRIS", nil, adminTok)
	expect("list trx method -> 200", w, 200)
	if int(resp["total"].(float64)) != 1 {
		t.Fatalf("trx method filter: %v", resp)
	}
	if body := raw("GET", "/transactions?q=zzz-no-match", nil, adminTok); !strings.Contains(body, `"items":[]`) {
		t.Fatalf("trx empty not []: %s", body)
	}
	w, _ = doReq("GET", "/transactions/99999", nil, adminTok)
	expect("get trx missing -> 404", w, 404)
	w, _ = doReq("GET", fmt.Sprintf("/transactions/%d", trx1), nil, adminTok)
	expect("get trx admin -> 200", w, 200)

	// ── 27. refund ─────────────────────────────────────────────
	w, _ = doReq("POST", fmt.Sprintf("/transactions/%d/refund", trx1), map[string]any{
		"items": []any{map[string]any{"productId": prod1, "qty": 99}}, "reason": "x",
	}, adminTok)
	expect("refund too much -> 400", w, 400)
	w, _ = doReq("POST", fmt.Sprintf("/transactions/%d/refund", trx1), map[string]any{
		"items": []any{map[string]any{"productId": prod1, "qty": 1}}, "reason": "",
	}, adminTok)
	expect("refund no reason -> 400", w, 400)
	w, resp = doReq("POST", fmt.Sprintf("/transactions/%d/refund", trx1), map[string]any{
		"items": []any{map[string]any{"productId": prod1, "qty": 1}}, "reason": "salah",
	}, adminTok)
	expect("partial refund -> 200", w, 200)
	if resp["status"] != "completed" {
		t.Fatalf("partial refund status: %v", resp)
	}
	w, resp = doReq("GET", fmt.Sprintf("/products/%d", prod1), nil, adminTok)
	if num(resp, "stock") != 11 {
		t.Fatalf("stock after refund: %v", resp)
	}
	w, resp = doReq("POST", fmt.Sprintf("/transactions/%d/refund", trx1), map[string]any{
		"items": []any{map[string]any{"productId": prod1, "qty": 1}, map[string]any{"productId": prod2, "qty": 1}}, "reason": "semua",
	}, adminTok)
	expect("full refund -> 200", w, 200)
	if resp["status"] != "refunded" {
		t.Fatalf("full refund status: %v", resp)
	}
	w, _ = doReq("POST", fmt.Sprintf("/transactions/%d/refund", trx1), map[string]any{
		"items": []any{map[string]any{"productId": prod1, "qty": 1}}, "reason": "lagi",
	}, adminTok)
	expect("refund refunded -> 409", w, 409)

	// ── 28. category delete ────────────────────────────────────
	w, resp = doReq("DELETE", fmt.Sprintf("/categories/%d", cat1), nil, adminTok)
	expect("delete used category -> 200", w, 200)
	if resp["soft_deleted"] != true {
		t.Fatalf("soft delete flag: %v", resp)
	}
	w, resp = doReq("DELETE", fmt.Sprintf("/categories/%d", cat3), nil, adminTok)
	expect("delete empty category -> 200", w, 200)
	if resp["soft_deleted"] != false {
		t.Fatalf("hard delete flag: %v", resp)
	}
	w, _ = doReq("DELETE", "/categories/99999", nil, adminTok)
	expect("delete missing category -> 404", w, 404)

	// ── 29. settings ───────────────────────────────────────────
	w, _ = doReq("GET", "/settings", nil, adminTok)
	expect("get settings -> 200", w, 200)
	w, _ = doReq("PUT", "/settings", map[string]any{"storeName": "T", "timezone": "Mars/Olympus"}, adminTok)
	expect("settings bad tz -> 400", w, 400)
	w, resp = doReq("PUT", "/settings", map[string]any{"storeName": "Toko Sweep", "taxEnabled": true, "taxPct": 10, "paper": "80mm", "timezone": "Asia/Makassar"}, adminTok)
	expect("update settings -> 200", w, 200)
	if resp["taxEnabled"] != true || resp["paper"] != "80mm" {
		t.Fatalf("settings update: %v", resp)
	}
	// tax math: P2 x1 @12000 -> tax 1200, total 13200
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items": []any{map[string]any{"productId": prod2, "qty": 1}}, "method": "Cash", "paid": 20000,
	}, adminTok)
	expect("checkout taxed -> 201", w, 201)
	if resp["tax"].(float64) != 1200 || resp["total"].(float64) != 13200 || resp["change"].(float64) != 6800 {
		t.Fatalf("tax math: %v", resp)
	}

	// ── 30. dashboard ──────────────────────────────────────────
	w, resp = doReq("GET", "/dashboard", nil, adminTok)
	expect("dashboard admin -> 200", w, 200)
	if resp["role"] != "admin" {
		t.Fatalf("dashboard role: %v", resp)
	}
	today := resp["today"].(map[string]any)
	if today["trx_count"].(float64) < 2 || today["omzet"].(float64) <= 0 || today["items_sold"].(float64) <= 0 {
		t.Fatalf("dashboard today: %v", resp)
	}
	if _, ok := today["low_stock"]; !ok {
		t.Fatalf("dashboard low_stock missing: %v", resp)
	}
	if len(resp["sales7"].([]any)) != 7 || len(resp["methods"].([]any)) == 0 ||
		len(resp["top_products"].([]any)) == 0 || len(resp["recent"].([]any)) == 0 {
		t.Fatalf("dashboard sections: %v", resp)
	}

	// ── 31. reports ────────────────────────────────────────────
	w, _ = doReq("GET", "/reports?period=never", nil, adminTok)
	expect("reports bad period -> 400", w, 400)
	w, resp = doReq("GET", "/reports?period=today", nil, adminTok)
	expect("reports today -> 200", w, 200)
	sum := resp["summary"].(map[string]any)
	if sum["trx_count"].(float64) < 2 || sum["omzet"].(float64) <= 0 ||
		sum["items_sold"].(float64) <= 0 || sum["gross_profit"].(float64) <= 0 {
		t.Fatalf("report summary: %v", resp)
	}
	prods := resp["products"].([]any)
	if len(prods) == 0 {
		t.Fatalf("report products empty: %v", resp)
	}
	p0 := prods[0].(map[string]any)
	if p0["sku"] == "" || p0["qty"].(float64) <= 0 || p0["revenue"].(float64) <= 0 {
		t.Fatalf("report product row: %v", p0)
	}
	if len(resp["by_method"].([]any)) == 0 || len(resp["by_status"].([]any)) == 0 ||
		len(resp["transactions"].([]any)) == 0 || len(resp["stock"].([]any)) == 0 {
		t.Fatalf("report sections: %v", resp)
	}
	st0 := resp["stock"].([]any)[0].(map[string]any)
	if st0["stock_value"].(float64) != st0["buy_price"].(float64)*st0["stock"].(float64) {
		t.Fatalf("stock_value math: %v", st0)
	}
	// empty period must be [] not null (by_status is period-independent
	// in both old and new code, so it is excluded here)
	for _, k := range []string{`"by_method":[]`, `"products":[]`, `"transactions":[]`} {
		if body := raw("GET", "/reports?period=yesterday", nil, adminTok); !strings.Contains(body, k) {
			t.Fatalf("reports yesterday missing %s: %s", k, body)
		}
	}
	t.Log("reports yesterday empty arrays ok")

	t.Log("ALL ENDPOINTS SWEEP PASSED")
}
