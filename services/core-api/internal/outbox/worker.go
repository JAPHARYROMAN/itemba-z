package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
)

type DeliveryRepository interface {
	TenantIDs(ctx context.Context) ([]string, error)
	Claim(ctx context.Context, tenantID, workerID string, limit int, now, lockedUntil time.Time) ([]Event, error)
	MarkPublished(ctx context.Context, tenantID, eventID, workerID string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, tenantID, eventID, workerID string, retryAt time.Time, reason string) error
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type Processor struct {
	Repository DeliveryRepository
	Publisher  Publisher
	Clock      clock.Clock
	WorkerID   string
	BatchSize  int
	Lease      time.Duration
	RetryDelay time.Duration
}

func (p Processor) RunOnce(ctx context.Context) (int, error) {
	if p.Repository == nil || p.Publisher == nil || p.Clock == nil || p.WorkerID == "" || p.BatchSize < 1 || p.Lease <= 0 || p.RetryDelay <= 0 {
		return 0, errors.New("outbox processor configuration is incomplete")
	}
	tenants, err := p.Repository.TenantIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list outbox tenants: %w", err)
	}
	published := 0
	var failures []error
	for _, tenantID := range tenants {
		now := p.Clock.Now().UTC()
		events, err := p.Repository.Claim(ctx, tenantID, p.WorkerID, p.BatchSize, now, now.Add(p.Lease))
		if err != nil {
			failures = append(failures, fmt.Errorf("claim tenant %s: %w", tenantID, err))
			continue
		}
		for _, event := range events {
			if err := p.Publisher.Publish(ctx, event); err != nil {
				retryAt := p.Clock.Now().UTC().Add(p.RetryDelay)
				if markErr := p.Repository.MarkFailed(ctx, tenantID, event.ID, p.WorkerID, retryAt, err.Error()); markErr != nil {
					failures = append(failures, fmt.Errorf("publish %s: %v; record failure: %w", event.ID, err, markErr))
				} else {
					failures = append(failures, fmt.Errorf("publish %s: %w", event.ID, err))
				}
				continue
			}
			if err := p.Repository.MarkPublished(ctx, tenantID, event.ID, p.WorkerID, p.Clock.Now().UTC()); err != nil {
				failures = append(failures, fmt.Errorf("mark event %s published: %w", event.ID, err))
				continue
			}
			published++
		}
	}
	return published, errors.Join(failures...)
}
