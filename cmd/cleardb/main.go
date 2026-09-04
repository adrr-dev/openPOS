package main

import (
	"context"
	"log"

	"github.com/0xMinomus/openPOS/backend/config"
	"github.com/0xMinomus/openPOS/backend/db"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()

	if err := database.Exec("TRUNCATE TABLE stores, email_otps RESTART IDENTITY CASCADE;").Error; err != nil {
		log.Fatalf("truncate error: %v", err)
	}

	log.Println("Database data cleared successfully!")
}
