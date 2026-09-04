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
	initErr error
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

func Handler(w http.ResponseWriter, r *http.Request) {
	if initErr != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("INIT ERROR: " + initErr.Error() + "\n"))
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api") {
		r.URL.Path = "/api" + r.URL.Path
	}
	h.ServeHTTP(w, r)
}
