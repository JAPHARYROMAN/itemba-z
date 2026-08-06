package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/integrations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.OpenWorker(ctx, databaseURL)
	if err != nil {
		logger.Error("initialize integration PostgreSQL worker", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	// Provider adapters are registered here only after contract approval and
	// protected sandbox qualification. An active route without a compiled
	// connector fails closed into the dead-letter register.
	processor := integrations.Processor{
		Repository: store, Registry: integrations.ConnectorMap{}, Clock: clock.System{},
		WorkerID: integrationWorkerID(), BatchSize: 25, Lease: time.Minute,
	}
	logger.Info("ITEMBA-Z integration worker ready", "worker_id", processor.WorkerID)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		completed, runErr := processor.RunOnce(ctx)
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			logger.Error("process integration deliveries", "error", runErr)
		}
		if completed > 0 {
			logger.Info("integration deliveries completed", "count", completed)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func integrationWorkerID() string {
	if value := os.Getenv("ITEMBA_INTEGRATION_WORKER_ID"); value != "" {
		return value
	}
	hostname, _ := os.Hostname()
	return "integration-" + hostname
}
