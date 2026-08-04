package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
)

type loggingPublisher struct{ logger *slog.Logger }

func (p loggingPublisher) Publish(_ context.Context, event outbox.Event) error {
	p.logger.Info("published local outbox event", "event_id", event.ID, "tenant_id", event.TenantID,
		"event_type", event.EventType, "aggregate_id", event.AggregateID,
		"correlation_id", event.CorrelationID, "causation_id", event.CausationID)
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if os.Getenv("DATABASE_URL") == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if os.Getenv("ITEMBA_ENV") == "production" {
		logger.Error("production outbox publisher is not configured; logging publisher is development-only")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Error("initialize PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	processor := outbox.Processor{Repository: store, Publisher: loggingPublisher{logger}, Clock: clock.System{},
		WorkerID: workerID(), BatchSize: 50, Lease: time.Minute, RetryDelay: 30 * time.Second}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := processor.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("process outbox", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func workerID() string {
	if value := os.Getenv("ITEMBA_WORKER_ID"); value != "" {
		return value
	}
	hostname, _ := os.Hostname()
	return "outbox-" + hostname
}
