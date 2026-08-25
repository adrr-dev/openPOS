// Entry point untuk Vercel serverless: setiap request HTTP memanggil Handler
// di file ini (BUKAN main() pada cmd/api). Router & pool database dibuat
// sekali per instansi hangat lalu dipakai ulang antar-request.
//
// Routing semua URL ke fungsi ini diatur oleh vercel.json (rewrites).
// Catatan: runtime @vercel/go mewajibkan paket bernama "handler".
package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	router "github.com/0xMinomus/openPOS/backend/router"
)

var (
	h       http.Handler
	initErr error // disimpan alih-alih panic, agar penyebabnya tampil di respons
)

func init() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	srv, err := router.New(ctx)
	if err != nil {
		initErr = err
		return
	}
	h = srv.Handler
}

// Handler adalah pintu masuk yang dipanggil runtime Go Vercel.
func Handler(w http.ResponseWriter, r *http.Request) {
	if initErr != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("INIT ERROR: " + initErr.Error() + "\n"))
		return
	}
	// chi kita ter-mount di /api/v1 — pastikan prefix ada
	if !strings.HasPrefix(r.URL.Path, "/api") {
		r.URL.Path = "/api" + r.URL.Path
	}
	h.ServeHTTP(w, r)
}
