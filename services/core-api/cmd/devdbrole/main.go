package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/dbrole"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("open administrative PostgreSQL connection", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	username := os.Getenv("ITEMBA_RUNTIME_USER")
	capability := dbrole.Capability(os.Getenv("ITEMBA_RUNTIME_CAPABILITY"))
	if err := dbrole.Provision(ctx, pool, username, os.Getenv("ITEMBA_RUNTIME_PASSWORD"), capability); err != nil {
		logger.Error("provision runtime database login", "runtime_user", username, "capability", capability, "error", err)
		os.Exit(1)
	}
	logger.Info("runtime database login provisioned", "runtime_user", username, "capability", capability)
}
