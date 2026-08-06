package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
)

type DeliveryRepository interface {
	IntegrationTenantIDs(context.Context) ([]string, error)
	ClaimIntegrationDeliveries(context.Context, string, string, int, time.Time, time.Time) ([]Delivery, error)
	CompleteIntegrationDelivery(context.Context, Delivery, Result, time.Time) error
	FailIntegrationDelivery(context.Context, Delivery, Failure, time.Time, bool, time.Time) error
}

type Connector interface {
	Deliver(context.Context, Route, Delivery) (Result, error)
}

type Registry interface {
	Connector(Capability, string) (Connector, bool)
}

type ConnectorMap map[string]Connector

func (m ConnectorMap) Connector(capability Capability, providerCode string) (Connector, bool) {
	connector, ok := m[string(capability)+":"+providerCode]
	return connector, ok
}

type Processor struct {
	Repository DeliveryRepository
	Registry   Registry
	Clock      clock.Clock
	WorkerID   string
	BatchSize  int
	Lease      time.Duration
}

func (p Processor) RunOnce(ctx context.Context) (int, error) {
	if p.Repository == nil || p.Registry == nil || p.Clock == nil || p.WorkerID == "" || p.BatchSize < 1 || p.Lease <= 0 {
		return 0, ErrInvalidConfiguration
	}
	tenants, err := p.Repository.IntegrationTenantIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list integration tenants: %w", err)
	}
	completed := 0
	var failures []error
	for _, tenantID := range tenants {
		now := p.Clock.Now().UTC()
		deliveries, claimErr := p.Repository.ClaimIntegrationDeliveries(ctx, tenantID, p.WorkerID, p.BatchSize, now, now.Add(p.Lease))
		if claimErr != nil {
			failures = append(failures, fmt.Errorf("claim integration tenant %s: %w", tenantID, claimErr))
			continue
		}
		for _, delivery := range deliveries {
			connector, ok := p.Registry.Connector(delivery.Capability, delivery.Route.ProviderCode)
			if !ok {
				failure := Failure{Code: "connector_unavailable", Message: ErrConnectorUnavailable.Error(), Retryable: false}
				if markErr := p.Repository.FailIntegrationDelivery(ctx, delivery, failure, now, true, p.Clock.Now().UTC()); markErr != nil {
					failures = append(failures, fmt.Errorf("dead-letter delivery %s: %w", delivery.ID, markErr))
				}
				continue
			}
			deliveryContext, cancel := context.WithTimeout(ctx, delivery.Route.Timeout)
			result, deliveryErr := connector.Deliver(deliveryContext, delivery.Route, delivery)
			cancel()
			finishedAt := p.Clock.Now().UTC()
			if deliveryErr == nil && delivery.Capability == TRAFiscalization && result.ProviderReference == "" {
				deliveryErr = Failure{Code: "invalid_provider_response", Message: "fiscal provider response omitted its reference", Retryable: false}
			}
			if deliveryErr == nil && len(result.Response) > 0 {
				var object map[string]any
				if !json.Valid(result.Response) || json.Unmarshal(result.Response, &object) != nil || object == nil {
					deliveryErr = Failure{Code: "invalid_provider_response", Message: "provider response evidence must be a JSON object", Retryable: false}
				}
			}
			if deliveryErr == nil {
				if err := p.Repository.CompleteIntegrationDelivery(ctx, delivery, result, finishedAt); err != nil {
					failures = append(failures, fmt.Errorf("complete delivery %s: %w", delivery.ID, err))
					continue
				}
				completed++
				continue
			}
			failure := classifyFailure(deliveryErr)
			deadLetter := !failure.Retryable || delivery.AttemptCount >= delivery.MaxAttempts
			retryAt := finishedAt
			if !deadLetter {
				delay := retryDelay(delivery, failure)
				retryAt = finishedAt.Add(delay)
			}
			if err := p.Repository.FailIntegrationDelivery(ctx, delivery, failure, retryAt, deadLetter, finishedAt); err != nil {
				failures = append(failures, fmt.Errorf("record delivery %s failure: %w", delivery.ID, err))
			}
		}
	}
	return completed, errors.Join(failures...)
}

func classifyFailure(err error) Failure {
	var pointer *Failure
	if errors.As(err, &pointer) && pointer != nil {
		return normalizeFailure(*pointer)
	}
	var failure Failure
	if errors.As(err, &failure) {
		return normalizeFailure(failure)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Failure{Code: "provider_timeout", Message: "provider request exceeded its configured timeout", Retryable: true}
	}
	return Failure{Code: "provider_unavailable", Message: "provider delivery failed", Retryable: true}
}

func normalizeFailure(failure Failure) Failure {
	if failure.Code == "" {
		failure.Code = "provider_error"
	}
	if failure.Message == "" {
		failure.Message = failure.Code
	}
	return failure
}

func retryDelay(delivery Delivery, failure Failure) time.Duration {
	if failure.RetryAfter > 0 {
		if failure.RetryAfter > delivery.Route.MaxBackoff {
			return delivery.Route.MaxBackoff
		}
		return failure.RetryAfter
	}
	exponent := delivery.AttemptCount - 1
	if exponent < 0 {
		exponent = 0
	}
	if exponent > 30 {
		exponent = 30
	}
	base := float64(delivery.Route.BaseBackoff) * math.Pow(2, float64(exponent))
	if base > float64(delivery.Route.MaxBackoff) {
		base = float64(delivery.Route.MaxBackoff)
	}
	// Stable jitter prevents a retry stampede without making a delivery's
	// schedule impossible to reproduce during incident analysis.
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", delivery.ID, delivery.AttemptCount)))
	jitter := 0.9 + (float64(sum[0])/255.0)*0.2
	delay := time.Duration(base * jitter)
	if delay > delivery.Route.MaxBackoff {
		return delivery.Route.MaxBackoff
	}
	return delay
}
