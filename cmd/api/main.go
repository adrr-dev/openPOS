package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	router "github.com/0xMinomus/openPOS/backend/router"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	app, err := router.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if app.Cleanup != nil {
		defer app.Cleanup()
	}

	srv := &http.Server{
		Addr:              ":" + app.Port,
		Handler:           app.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("OpenPOS backend (Gin + GORM) berjalan di http://localhost:%s", app.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("mematikan server…")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
