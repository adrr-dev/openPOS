package router

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/0xMinomus/openPOS/backend/config"
	"github.com/0xMinomus/openPOS/backend/db"
	"github.com/0xMinomus/openPOS/backend/handler"
	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/repo"
	"github.com/0xMinomus/openPOS/backend/service"
)

type Server struct {
	Handler http.Handler
	Port    string
	Cleanup func()
}

func New(ctx context.Context) (*Server, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	cleanup := func() { pool.Close() }

	if err := db.Migrate(context.Background(), pool); err != nil {
		cleanup()
		return nil, err
	}
	log.Println("migrasi: ok")

	userRepo := repo.NewUserRepo(pool)
	cashierRepo := repo.NewCashierRepo(pool)
	refreshRepo := repo.NewRefreshRepo(pool)
	categoryRepo := repo.NewCategoryRepo(pool)
	productRepo := repo.NewProductRepo(pool)
	movementRepo := repo.NewMovementRepo(pool)
	trxRepo := repo.NewTrxRepo(pool)
	storeRepo := repo.NewStoreRepo(pool)
	reportRepo := repo.NewReportRepo(pool)

	authSvc := service.NewAuthService(userRepo, cashierRepo, refreshRepo, cfg.JWTSecret, cfg.AccessTTL, time.Duration(cfg.RefreshTTLDays)*24*time.Hour)
	userSvc := service.NewUserService(userRepo, cashierRepo)
	catalogSvc := service.NewCatalogService(categoryRepo, productRepo, movementRepo)
	trxSvc := service.NewTrxService(trxRepo)
	settingsSvc := service.NewSettingsService(storeRepo, userRepo, cashierRepo, reportRepo)

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
			r.Post("/auth/switch", authH.Switch) // owner → kasir

			// katalog: semua role boleh membaca (POS kasir), tulis hanya admin
			r.Get("/categories", catalogH.ListCategories)
			r.Get("/products", catalogH.ListProducts)
			r.Get("/products/{id}", catalogH.GetProduct)
			r.Post("/transactions", trxH.Checkout)
			r.Get("/transactions", trxH.List)
			r.Get("/transactions/{id}", trxH.Get)
			r.Get("/settings", settingsH.Get)
			r.Get("/dashboard", settingsH.Dashboard)

			// khusus admin (owner)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
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

	return &Server{Handler: r, Port: cfg.Port, Cleanup: cleanup}, nil
}
