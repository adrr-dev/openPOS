package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0xMinomus/openPOS/backend/internal/service"
)

type ctxKey string

const claimsKey ctxKey = "claims"

// Auth memvalidasi Bearer access token dan menyimpan claims ke context.
func Auth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "token tidak disertakan")
				return
			}
			claims, err := authSvc.ParseAccess(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "sesi tidak valid, silakan masuk kembali")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole membatasi akses berdasarkan peran (dipakai modul berikutnya).
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := ClaimsFrom(r.Context())
			if c == nil {
				writeError(w, http.StatusUnauthorized, "sesi tidak valid")
				return
			}
			for _, role := range roles {
				if string(c.Role) == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "akses ditolak untuk peran Anda")
		})
	}
}

// ClaimsFrom mengambil claims user dari context (nil bila belum terautentikasi).
func ClaimsFrom(ctx context.Context) *service.Claims {
	c, _ := ctx.Value(claimsKey).(*service.Claims)
	return c
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
