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
	_ "time/tzdata" // zona waktu ter-embed (dashboard/laporan per toko)

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/0xMinomus/openPOS/backend/internal/config"
	"github.com/0xMinomus/openPOS/backend/internal/db"
	"github.com/0xMinomus/openPOS/backend/internal/handler"
	"github.com/0xMinomus/openPOS/backend/internal/middleware"
	"github.com/0xMinomus/openPOS/backend/internal/model"
	"github.com/0xMinomus/openPOS/backend/internal/repo"
	"github.com/0xMinomus/openPOS/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(context.Background(), pool); err != nil {
		log.Fatalf("migrasi: %v", err)
	}
	log.Println("migrasi: ok")

	userRepo := repo.NewUserRepo(pool)
	refreshRepo := repo.NewRefreshRepo(pool)
	categoryRepo := repo.NewCategoryRepo(pool)
	productRepo := repo.NewProductRepo(pool)
	movementRepo := repo.NewMovementRepo(pool)
	trxRepo := repo.NewTrxRepo(pool)
	storeRepo := repo.NewStoreRepo(pool)
	reportRepo := repo.NewReportRepo(pool)
	authSvc := service.NewAuthService(userRepo, refreshRepo, cfg.JWTSecret, cfg.AccessTTL, time.Duration(cfg.RefreshTTLDays)*24*time.Hour)
	userSvc := service.NewUserService(userRepo)
	catalogSvc := service.NewCatalogService(categoryRepo, productRepo, movementRepo)
	trxSvc := service.NewTrxService(trxRepo)
	settingsSvc := service.NewSettingsService(storeRepo, userRepo, reportRepo)

	authH := handler.NewAuthHandler(authSvc)
	userH := handler.NewUserHandler(userSvc)
	catalogH := handler.NewCatalogHandler(catalogSvc)
	stockH := handler.NewStockHandler(catalogSvc)
	trxH := handler.NewTrxHandler(trxSvc)
	settingsH := handler.NewSettingsHandler(settingsSvc)
	healthH := handler.NewHealthHandler(pool)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", healthH.Health)

		// publik
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)
		r.Post("/auth/logout", authH.Logout)

		// butuh access token
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Get("/auth/me", authH.Me)

			// katalog: semua role boleh membaca (POS kasir), tulis hanya admin
			r.Get("/categories", catalogH.ListCategories)
			r.Get("/products", catalogH.ListProducts)
			r.Get("/products/{id}", catalogH.GetProduct)
			r.Post("/transactions", trxH.Checkout)
			r.Get("/transactions", trxH.List)
			r.Get("/transactions/{id}", trxH.Get)
			r.Get("/settings", settingsH.Get)
			r.Get("/dashboard", settingsH.Dashboard)

			// khusus admin
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(string(model.RoleAdmin)))
				r.Get("/users", userH.List)
				r.Post("/users", userH.Create)
				r.Patch("/users/{id}/active", userH.SetActive)
				r.Post("/categories", catalogH.CreateCategory)
				r.Delete("/categories/{id}", catalogH.DeleteCategory)
				r.Post("/products", catalogH.CreateProduct)
				r.Put("/products/{id}", catalogH.UpdateProduct)
				r.Patch("/products/{id}/active", catalogH.SetProductActive)
				r.Get("/movements", stockH.ListMovements)
				r.Post("/stock/adjustments", stockH.AdjustStock)
				r.Post("/transactions/{id}/refund", trxH.Refund)
				r.Put("/settings", settingsH.Update)
				r.Put("/users/{id}/passcode", settingsH.SetPasscode)
				r.Get("/reports", settingsH.Report)
			})
		})
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("OpenPOS backend berjalan di http://localhost:%s (CORS: %v)", cfg.Port, cfg.CORSOrigins)
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
