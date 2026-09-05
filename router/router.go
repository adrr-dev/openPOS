package router

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/0xMinomus/openPOS/backend/config"
	"github.com/0xMinomus/openPOS/backend/db"
	"github.com/0xMinomus/openPOS/backend/handler"
	"github.com/0xMinomus/openPOS/backend/middleware"
	"github.com/0xMinomus/openPOS/backend/model"
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

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	sqlDB, _ := database.DB()
	cleanup := func() { _ = sqlDB.Close() }

	if err := db.Migrate(database); err != nil {
		cleanup()
		return nil, err
	}
	log.Println("migrasi GORM: ok")

	userRepo := repo.NewUserRepo(database)
	cashierRepo := repo.NewCashierRepo(database)
	refreshRepo := repo.NewRefreshRepo(database)
	otpRepo := repo.NewOtpRepo(database)
	categoryRepo := repo.NewCategoryRepo(database)
	productRepo := repo.NewProductRepo(database)
	movementRepo := repo.NewMovementRepo(database)
	trxRepo := repo.NewTrxRepo(database)
	storeRepo := repo.NewStoreRepo(database)
	reportRepo := repo.NewReportRepo(database)

	authSvc := service.NewAuthService(userRepo, cashierRepo, refreshRepo, otpRepo, cfg.JWTSecret, cfg.AccessTTL, time.Duration(cfg.RefreshTTLDays)*24*time.Hour, cfg.GoogleClientID)
	userSvc := service.NewUserService(userRepo, cashierRepo)
	catalogSvc := service.NewCatalogService(categoryRepo, productRepo, movementRepo)
	trxSvc := service.NewTrxService(trxRepo, cashierRepo)
	settingsSvc := service.NewSettingsService(storeRepo, userRepo, cashierRepo, reportRepo)

	authH := handler.NewAuthHandler(authSvc)
	userH := handler.NewUserHandler(userSvc)
	catalogH := handler.NewCatalogHandler(catalogSvc)
	stockH := handler.NewStockHandler(catalogSvc)
	trxH := handler.NewTrxHandler(trxSvc)
	settingsH := handler.NewSettingsHandler(settingsSvc)
	healthH := handler.NewHealthHandler(database)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	// 30s request timeout like old chi backend (chimw.Timeout).
	// Literal bug: slow queries could hang forever.
	r.Use(func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		// Fixed: was 300 (300ns) — cors.Config expects time.Duration.
		MaxAge: 300 * time.Second,
	}))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", healthH.Health)

		// Public
		v1.POST("/auth/register", authH.Register)
		v1.POST("/auth/login", authH.Login)
		v1.POST("/auth/google", authH.Google)
		v1.POST("/auth/refresh", authH.Refresh)
		v1.POST("/auth/logout", authH.Logout)
		v1.POST("/auth/otp/send", authH.SendOTP)
		v1.POST("/auth/otp/verify", authH.VerifyOTP)

		// Authenticated
		authGroup := v1.Group("")
		authGroup.Use(middleware.Auth(authSvc))
		{
			authGroup.GET("/auth/me", authH.Me)
			authGroup.POST("/auth/switch", authH.Switch)

			authGroup.GET("/categories", catalogH.ListCategories)
			authGroup.GET("/products", catalogH.ListProducts)
			authGroup.GET("/products/:id", catalogH.GetProduct)
			authGroup.POST("/transactions", trxH.Checkout)
			authGroup.GET("/transactions", trxH.List)
			authGroup.GET("/transactions/:id", trxH.Get)
			authGroup.GET("/settings", settingsH.Get)
			authGroup.GET("/dashboard", settingsH.Dashboard)

			// Admin only
			adminGroup := authGroup.Group("")
			adminGroup.Use(middleware.RequireRole(string(model.RoleAdmin)))
			{
				adminGroup.GET("/users", userH.List)
				adminGroup.POST("/users", userH.Create)
				adminGroup.PATCH("/users/:id/active", userH.SetActive)
				adminGroup.POST("/categories", catalogH.CreateCategory)
				adminGroup.DELETE("/categories/:id", catalogH.DeleteCategory)
				adminGroup.POST("/products", catalogH.CreateProduct)
				adminGroup.PUT("/products/:id", catalogH.UpdateProduct)
				adminGroup.PATCH("/products/:id/active", catalogH.SetProductActive)
				adminGroup.GET("/movements", stockH.ListMovements)
				adminGroup.POST("/stock/adjustments", stockH.AdjustStock)
				adminGroup.POST("/transactions/:id/refund", trxH.Refund)
				adminGroup.PUT("/settings", settingsH.Update)
				adminGroup.PUT("/users/:id/passcode", settingsH.SetPasscode)
				adminGroup.GET("/reports", settingsH.Report)
			}
		}
	}

	return &Server{Handler: r, Port: cfg.Port, Cleanup: cleanup}, nil
}
