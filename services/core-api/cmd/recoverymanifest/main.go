// Command recoverymanifest captures a deterministic, non-secret manifest for
// comparing an authoritative PostgreSQL database with an isolated restore.
package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/recovery"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := telemetry.NewJSONLogger(os.Stderr)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("open PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	manifest, err := recovery.Capture(ctx, pool, "itembaz")
	if err != nil {
		logger.Error("capture recovery manifest", "error", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		logger.Error("encode recovery manifest", "error", err)
		os.Exit(1)
	}
}
