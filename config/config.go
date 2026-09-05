package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTLDays int
	CORSOrigins    []string
	// GoogleClientID enables POST /auth/google (GIS ID-token flow).
	// Empty = endpoint returns "not configured"; OTP flow unaffected.
	GoogleClientID string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AccessTTL:      15 * time.Minute,
		RefreshTTLDays: 7,
		GoogleClientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL wajib diisi (contoh: postgres://user:pass@host:5432/postgres)")
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET wajib diisi")
	}

	if v := os.Getenv("ACCESS_TTL_MINUTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, errors.New("ACCESS_TTL_MINUTES harus angka positif")
		}
		cfg.AccessTTL = time.Duration(n) * time.Minute
	}
	if v := os.Getenv("REFRESH_TTL_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, errors.New("REFRESH_TTL_DAYS harus angka positif")
		}
		cfg.RefreshTTLDays = n
	}

	origins := getEnv("CORS_ORIGINS", "http://localhost:5173")
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
