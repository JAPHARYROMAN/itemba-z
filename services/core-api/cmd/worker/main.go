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
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/telemetry"
)

type loggingPublisher struct{ logger *slog.Logger }

func (p loggingPublisher) Publish(_ context.Context, event outbox.Event) error {
	p.logger.Info("published local outbox event", "event_id", event.ID, "tenant_id", event.TenantID,
		"event_type", event.EventType, "aggregate_id", event.AggregateID,
		"correlation_id", event.CorrelationID, "causation_id", event.CausationID)
	return nil
}

type projectingPublisher struct {
	projector *integrations.Projector
	next      outbox.Publisher
}

func (p projectingPublisher) Publish(ctx context.Context, event outbox.Event) error {
	if err := p.projector.Publish(ctx, event); err != nil {
		return err
	}
	return p.next.Publish(ctx, event)
}

func main() {
	logger := telemetry.NewJSONLogger(os.Stdout)
	if os.Getenv("DATABASE_URL") == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if !loggingPublisherAllowed(os.Getenv("ITEMBA_ENV")) {
		logger.Error("external outbox publisher is not configured; logging publisher is allowed only in development or test")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.OpenWorker(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Error("initialize PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	projector, err := integrations.NewProjector(store, identity.UUIDGenerator{})
	if err != nil {
		logger.Error("initialize integration projector", "error", err)
		os.Exit(1)
	}
	publisher := projectingPublisher{projector: projector, next: loggingPublisher{logger}}
	processor := outbox.Processor{Repository: store, Publisher: publisher, Clock: clock.System{},
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

func loggingPublisherAllowed(environment string) bool {
	return environment == "development" || environment == "test"
}

func workerID() string {
	if value := os.Getenv("ITEMBA_WORKER_ID"); value != "" {
		return value
	}
	hostname, _ := os.Hostname()
	return "outbox-" + hostname
}
