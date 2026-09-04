package db

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/0xMinomus/openPOS/backend/model"
)

// DB_DRIVER selects the database explicitly:
//   DB_DRIVER=sqlite   -> local file DB (dev/test, no Supabase needed)
//   DB_DRIVER=postgres  -> force Supabase/Postgres even if URL looks like a file
//   DB_DRIVER unset     -> auto-detect: file:/.db/sqlite in DATABASE_URL means
//                          sqlite, otherwise postgres. On Vercel just leave it
//                          unset and set DATABASE_URL to the pooler string.
func Connect(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	driver := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
	lowerURL := strings.ToLower(databaseURL)
	isSQLite := driver == "sqlite" ||
		(driver == "" && (strings.HasPrefix(lowerURL, "file:") ||
			strings.Contains(lowerURL, ".db") ||
			strings.Contains(lowerURL, "sqlite")))

	if isSQLite {
		if databaseURL == "" || databaseURL == "postgres://..." {
			databaseURL = "openpos_test.db"
		}
		dialector = sqlite.Open(databaseURL)
	} else {
		// Force simple protocol so Supabase PgBouncer (port 6543,
		// transaction mode) works, which doesn't support prepared statements.
		dialector = postgres.New(postgres.Config{
			DSN:                  databaseURL,
			PreferSimpleProtocol: true,
		})
	}

	// Quiet SQL logs on hosted Postgres (Vercel log spam), verbose locally.
	// Set LOG_SQL=true to force query logging anywhere.
	logLevel := logger.Warn
	if isSQLite || strings.EqualFold(os.Getenv("LOG_SQL"), "true") {
		logLevel = logger.Info
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database via GORM: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan sql.DB: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("gagal ping database: %w", err)
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Store{},
		&model.User{},
		&model.Cashier{},
		&model.RefreshToken{},
		&model.EmailOtp{},
		&model.Category{},
		&model.Product{},
		&model.Movement{},
		&model.Trx{},
		&model.TransactionItem{},
		&model.Refund{},
	)
}
