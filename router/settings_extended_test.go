package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrr-dev/openPOS/backend/service"
)

// Kontrak Extended Store Settings: key baru selalu ada, validasi §3,
// field tak dikenal diabaikan, merge-semantics PUT, rumus inclusive + rounding.
func TestExtendedStoreSettings(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", filepath.Join(t.TempDir(), "settings_ext_test.db"))
	t.Setenv("JWT_SECRET", "test-secret-settings-ext")

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

	email := "admin_ext_" + time.Now().Format("20060102150405") + "@tokosaya.com"
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
	w, _ = doReq("POST", "/auth/otp/verify", map[string]string{"email": email, "code": lastCode}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("VerifyOTP failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	w, resp := doReq("POST", "/auth/register", map[string]string{
		"name": "Admin Ext", "email": email, "password": "password123", "storeName": "Toko Ext",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("Register failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	token := resp["access_token"].(string)

	// 1. GET mengembalikan semua key baru dengan default aman.
	w, resp = doReq("GET", "/settings", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("Get settings failed: code=%d, body=%s", w.Code, w.Body.String())
	}
	for _, k := range []string{
		"storeName", "address", "phone", "timezone", "receiptHeader", "receiptFooter",
		"paper", "taxEnabled", "taxPct",
		"businessType", "email", "city", "province", "currency", "hours",
		"receiptShowLogo", "receiptShowCashier", "receiptShowMethod",
		"receiptShowTax", "receiptShowDiscount", "receiptShowNote",
		"taxName", "taxInclusive", "taxRounding", "taxApplyTo",
	} {
		if _, ok := resp[k]; !ok {
			t.Fatalf("GET /settings missing key %q: %v", k, resp)
		}
	}
	// logoUrl & passcodeUpdatedAt disepakati batal/tunda: tidak boleh ada.
	if _, ok := resp["logoUrl"]; ok {
		t.Fatalf("logoUrl harus dibatalkan, tapi muncul: %v", resp["logoUrl"])
	}
	if _, ok := resp["passcodeUpdatedAt"]; ok {
		t.Fatalf("passcodeUpdatedAt ditunda, tapi muncul: %v", resp["passcodeUpdatedAt"])
	}
	if resp["currency"] != "IDR" || resp["taxRounding"] != "none" || resp["taxApplyTo"] != "all" {
		t.Fatalf("defaults salah: %v", resp)
	}
	if hours, ok := resp["hours"].([]any); !ok || len(hours) != 0 {
		t.Fatalf("hours default harus []: %v", resp["hours"])
	}
	for _, k := range []string{"receiptShowLogo", "receiptShowCashier", "receiptShowMethod", "receiptShowTax", "receiptShowDiscount", "receiptShowNote"} {
		if resp[k] != true {
			t.Fatalf("default %s harus true: %v", k, resp[k])
		}
	}

	// 2. Validasi §3 → 400 berbahasa Indonesia.
	badCases := []struct {
		name string
		body map[string]any
		msg  string
	}{
		{"businessType", map[string]any{"storeName": "Toko Ext", "businessType": "xxx"}, "Jenis usaha tidak valid."},
		{"email", map[string]any{"storeName": "Toko Ext", "email": "bukan-email"}, "Email tidak valid."},
		{"currency", map[string]any{"storeName": "Toko Ext", "currency": "ID"}, "Kode mata uang tidak valid."},
		{"hours-count", map[string]any{"storeName": "Toko Ext", "hours": []any{
			map[string]any{"days": "Senin – Jumat", "open": "08:00", "close": "21:00"},
		}}, "Jam operasional tidak valid."},
		{"hours-label", map[string]any{"storeName": "Toko Ext", "hours": []any{
			map[string]any{"days": "Senin-Jumat", "open": "08:00", "close": "21:00"},
			map[string]any{"days": "Sabtu", "open": "08:00", "close": "21:00"},
			map[string]any{"days": "Minggu", "open": nil, "close": nil},
		}}, "Jam operasional tidak valid."},
		{"hours-order", map[string]any{"storeName": "Toko Ext", "hours": validHoursPayload("21:00", "08:00")}, "Jam operasional tidak valid."},
		{"footer", map[string]any{"storeName": "Toko Ext", "receiptFooter": strings.Repeat("x", 201)}, "Pesan footer maksimal 200 karakter."},
		{"taxName", map[string]any{"storeName": "Toko Ext", "taxName": strings.Repeat("y", 21)}, "Nama pajak maksimal 20 karakter."},
		{"taxPct", map[string]any{"storeName": "Toko Ext", "taxPct": 150}, "Tarif pajak maksimal 100 persen."},
		{"taxRounding", map[string]any{"storeName": "Toko Ext", "taxRounding": "half"}, "Pembulatan pajak tidak valid."},
		{"taxApplyTo", map[string]any{"storeName": "Toko Ext", "taxApplyTo": "kategori"}, "Cakupan pajak tidak valid."},
		{"bool-type", map[string]any{"storeName": "Toko Ext", "receiptShowLogo": "yes"}, "body JSON tidak valid"},
	}
	for _, tc := range badCases {
		w, resp = doReq("PUT", "/settings", tc.body, token)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: want 400 got %d %s", tc.name, w.Code, w.Body.String())
		}
		if resp["error"] != tc.msg {
			t.Fatalf("%s: want error %q got %q", tc.name, tc.msg, resp["error"])
		}
	}

	// 3. Field tak dikenal diabaikan (bukan 400).
	w, _ = doReq("PUT", "/settings", map[string]any{"storeName": "Toko Ext", "ngasal": 123}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("unknown field: want 200 got %d %s", w.Code, w.Body.String())
	}

	// 4. PUT full valid tersimpan + terbaca kembali.
	full := map[string]any{
		"storeName": "Toko Ext", "address": "Jl. Merdeka", "phone": "08123",
		"timezone": "Asia/Jakarta", "receiptHeader": "H", "receiptFooter": "F",
		"paper": "80mm", "taxEnabled": true, "taxPct": 10,
		"businessType": "retail", "email": "toko@gmail.com", "city": "Jakarta",
		"province": "DKI Jakarta", "currency": "idr", "hours": validHoursPayload("08:00", "21:00"),
		"receiptShowLogo": false, "receiptShowCashier": true, "receiptShowMethod": true,
		"receiptShowTax": true, "receiptShowDiscount": false, "receiptShowNote": true,
		"taxName": "PPN", "taxInclusive": false, "taxRounding": "none", "taxApplyTo": "all",
	}
	w, resp = doReq("PUT", "/settings", full, token)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT full: want 200 got %d %s", w.Code, w.Body.String())
	}
	w, resp = doReq("GET", "/settings", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after PUT: %d", w.Code)
	}
	if resp["businessType"] != "retail" || resp["currency"] != "IDR" || resp["taxName"] != "PPN" {
		t.Fatalf("persist gagal: %v", resp)
	}
	if resp["receiptShowLogo"] != false || resp["receiptShowDiscount"] != false {
		t.Fatalf("flags receiptShow tidak tersimpan: %v", resp)
	}
	if hours, ok := resp["hours"].([]any); !ok || len(hours) != 3 {
		t.Fatalf("hours tidak tersimpan: %v", resp["hours"])
	}

	// 5. PUT parsial tidak me-wipe field extended (merge-semantics).
	w, _ = doReq("PUT", "/settings", map[string]any{"storeName": "Toko Ext 2"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT parsial: want 200 got %d %s", w.Code, w.Body.String())
	}
	w, resp = doReq("GET", "/settings", nil, token)
	if resp["storeName"] != "Toko Ext 2" || resp["businessType"] != "retail" || resp["currency"] != "IDR" {
		t.Fatalf("merge gagal: %v", resp)
	}

	// 6. Rumus inclusive opsi C: base 11000 incl. 10% → tax 1000, total 11000.
	w, resp = doReq("POST", "/products", map[string]any{"name": "Barang", "sku": "BRG-1", "sellPrice": 11000, "stock": 50}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create product: %d %s", w.Code, w.Body.String())
	}
	prodID := resp["id"].(float64)
	w, _ = doReq("PUT", "/settings", map[string]any{"storeName": "Toko Ext 2", "taxEnabled": true, "taxPct": 10, "taxInclusive": true, "taxRounding": "none"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("enable inclusive tax: %d %s", w.Code, w.Body.String())
	}
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items": []any{map[string]any{"productId": prodID, "qty": 1}}, "method": "Cash", "paid": 11000,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("checkout inclusive: %d %s", w.Code, w.Body.String())
	}
	if resp["tax"].(float64) != 1000 || resp["total"].(float64) != 11000 {
		t.Fatalf("rumus inclusive salah: %v", resp)
	}

	// 7. taxRounding down vs none pada pajak pecahan (105 × 10% = 10.5).
	w, _ = doReq("PUT", "/settings", map[string]any{"storeName": "Toko Ext 2", "taxEnabled": true, "taxPct": 10, "taxInclusive": false, "taxRounding": "down"}, token)
	w, resp = doReq("POST", "/products", map[string]any{"name": "Pecahan", "sku": "PCH-1", "sellPrice": 105, "stock": 50}, token)
	prod2 := resp["id"].(float64)
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items": []any{map[string]any{"productId": prod2, "qty": 1}}, "method": "Cash", "paid": 200,
	}, token)
	if w.Code != http.StatusCreated || resp["tax"].(float64) != 10 || resp["total"].(float64) != 115 {
		t.Fatalf("rounding down salah: %d %v", w.Code, resp)
	}
	w, _ = doReq("PUT", "/settings", map[string]any{"storeName": "Toko Ext 2", "taxRounding": "none"}, token)
	w, resp = doReq("POST", "/transactions", map[string]any{
		"items": []any{map[string]any{"productId": prod2, "qty": 1}}, "method": "Cash", "paid": 200,
	}, token)
	if w.Code != http.StatusCreated || resp["tax"].(float64) != 11 || resp["total"].(float64) != 116 {
		t.Fatalf("rounding none salah: %d %v", w.Code, resp)
	}
}

func validHoursPayload(open, close string) []any {
	return []any{
		map[string]any{"days": "Senin – Jumat", "open": open, "close": close},
		map[string]any{"days": "Sabtu", "open": open, "close": close},
		map[string]any{"days": "Minggu", "open": nil, "close": nil},
	}
}
