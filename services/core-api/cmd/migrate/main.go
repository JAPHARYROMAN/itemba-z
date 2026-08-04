package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("open PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	directory := os.Getenv("ITEMBA_MIGRATIONS_DIR")
	if directory == "" {
		directory = "migrations"
	}
	if err := (migrate.Runner{Pool: pool}).Apply(ctx, migrate.Directory(directory)); err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations applied")
}
