package router

// Google login tests. The real Google verifier is stubbed — no network.
// OTP flow is not touched here; the full sweep covers it.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/idtoken"

	"github.com/0xMinomus/openPOS/backend/service"
)

func googleTestServer(t *testing.T, clientID string) (func(method, path string, body any, token string) (*httptest.ResponseRecorder, map[string]any), func()) {
	t.Helper()
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", filepath.Join(t.TempDir(), "google.db"))
	t.Setenv("JWT_SECRET", "google-test-secret")
	t.Setenv("GOOGLE_CLIENT_ID", clientID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	srv, err := New(ctx)
	if err != nil {
		cancel()
		t.Fatalf("New(): %v", err)
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
	return doReq, func() {
		srv.Cleanup()
		cancel()
		service.VerifyGoogleToken = idtoken.Validate
	}
}

func stubGoogle(claims map[string]interface{}, err error) {
	service.VerifyGoogleToken = func(ctx context.Context, token, audience string) (*idtoken.Payload, error) {
		if err != nil {
			return nil, err
		}
		if audience != "test-client-id" {
			return nil, errInvalidAudience
		}
		return &idtoken.Payload{Claims: claims}, nil
	}
}

var errInvalidAudience = errTest("audience salah")

type errTest string

func (e errTest) Error() string { return string(e) }

func TestGoogleLogin(t *testing.T) {
	doReq, done := googleTestServer(t, "test-client-id")
	defer done()

	// invalid token -> 401
	stubGoogle(nil, errTest("bad token"))
	w, _ := doReq("POST", "/auth/google", map[string]string{"id_token": "junk"}, "")
	if w.Code != 401 {
		t.Fatalf("bad token: want 401 got %d %s", w.Code, w.Body.String())
	}

	// missing id_token -> 400
	w, _ = doReq("POST", "/auth/google", map[string]string{}, "")
	if w.Code != 400 {
		t.Fatalf("empty token: want 400 got %d %s", w.Code, w.Body.String())
	}

	// unverified email -> 400
	stubGoogle(map[string]interface{}{"email": "g@toko.com", "name": "G", "email_verified": false}, nil)
	w, _ = doReq("POST", "/auth/google", map[string]string{"id_token": "tok"}, "")
	if w.Code != 400 {
		t.Fatalf("unverified: want 400 got %d %s", w.Code, w.Body.String())
	}

	// new user -> store + admin created
	stubGoogle(map[string]interface{}{"email": "g@toko.com", "name": "Gina", "email_verified": true}, nil)
	w, resp := doReq("POST", "/auth/google", map[string]any{"id_token": "tok", "storeName": "Toko Gina"}, "")
	if w.Code != 200 {
		t.Fatalf("new user: want 200 got %d %s", w.Code, w.Body.String())
	}
	user := resp["user"].(map[string]any)
	if user["role"] != "admin" || user["store_name"] != "Toko Gina" {
		t.Fatalf("new user shape: %v", resp)
	}
	if _, ok := resp["access_token"]; !ok {
		t.Fatalf("no access token: %v", resp)
	}

	// existing email auto-links (same account, password untouched)
	stubGoogle(map[string]interface{}{"email": "g@toko.com", "name": "Gina", "email_verified": true}, nil)
	w, resp2 := doReq("POST", "/auth/google", map[string]string{"id_token": "tok"}, "")
	if w.Code != 200 {
		t.Fatalf("link: want 200 got %d %s", w.Code, w.Body.String())
	}
	if resp2["user"].(map[string]any)["id"] != user["id"] {
		t.Fatalf("link id mismatch: %v vs %v", resp2, resp)
	}

	// me works with the Google-issued token
	tok := resp2["access_token"].(string)
	w, _ = doReq("GET", "/auth/me", nil, tok)
	if w.Code != 200 {
		t.Fatalf("me: want 200 got %d %s", w.Code, w.Body.String())
	}

	// default store name when omitted
	stubGoogle(map[string]interface{}{"email": "h@toko.com", "email_verified": true}, nil)
	w, resp = doReq("POST", "/auth/google", map[string]string{"id_token": "tok"}, "")
	if w.Code != 200 {
		t.Fatalf("default store: want 200 got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(resp["user"].(map[string]any)["store_name"].(string), "h") {
		t.Fatalf("default store name: %v", resp)
	}
}

func TestGoogleNotConfigured(t *testing.T) {
	doReq, done := googleTestServer(t, "")
	defer done()

	w, _ := doReq("POST", "/auth/google", map[string]string{"id_token": "whatever"}, "")
	if w.Code != 500 {
		t.Fatalf("unconfigured: want 500 got %d %s", w.Code, w.Body.String())
	}
}
