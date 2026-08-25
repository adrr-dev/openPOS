// Entry point untuk Vercel serverless: setiap request HTTP memanggil Handler
// di file ini (BUKAN main() pada cmd/api). Router & pool database dibuat
// sekali per instansi hangat lalu dipakai ulang antar-request.
//
// Routing semua URL ke fungsi ini diatur oleh vercel.json (rewrites).
package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	router "github.com/0xMinomus/openPOS/backend/internal/router"
)

var h http.Handler

func init() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	srv, err := router.New(ctx)
	if err != nil {
		panic(err)
	}
	h = srv.Handler
}

// Handler adalah pintu masuk yang dipanggil runtime Go Vercel.
func Handler(w http.ResponseWriter, r *http.Request) {
	// chi kita ter-mount di /api/v1 — pastikan prefix ada
	if !strings.HasPrefix(r.URL.Path, "/api") {
		r.URL.Path = "/api" + r.URL.Path
	}
	h.ServeHTTP(w, r)
}

// main tidak pernah dipanggil oleh Vercel; ada hanya agar `go build ./...`
// dan tooling Go standar tetap puas terhadap paket main ini.
func main() {}
