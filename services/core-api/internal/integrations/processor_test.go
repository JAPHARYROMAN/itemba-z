package integrations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
)

type processorRepository struct {
	deliveries []Delivery
	succeeded  []Result
	failed     []Failure
	dead       []bool
	retryAt    []time.Time
}

func (r *processorRepository) IntegrationTenantIDs(context.Context) ([]string, error) {
	return []string{"tenant-1"}, nil
}
func (r *processorRepository) ClaimIntegrationDeliveries(context.Context, string, string, int, time.Time, time.Time) ([]Delivery, error) {
	return append([]Delivery(nil), r.deliveries...), nil
}
func (r *processorRepository) CompleteIntegrationDelivery(_ context.Context, _ Delivery, result Result, _ time.Time) error {
	r.succeeded = append(r.succeeded, result)
	return nil
}
func (r *processorRepository) FailIntegrationDelivery(_ context.Context, _ Delivery, failure Failure, retryAt time.Time, dead bool, _ time.Time) error {
	r.failed, r.retryAt, r.dead = append(r.failed, failure), append(r.retryAt, retryAt), append(r.dead, dead)
	return nil
}

type connectorFunc func(context.Context, Route, Delivery) (Result, error)

func (f connectorFunc) Deliver(ctx context.Context, route Route, delivery Delivery) (Result, error) {
	return f(ctx, route, delivery)
}

func deliveryFixture() Delivery {
	at := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	route := Route{ID: "route-1", Capability: TRAFiscalization, ProviderCode: "TRA_TEST", Timeout: time.Second, MaxAttempts: 3, BaseBackoff: 10 * time.Second, MaxBackoff: time.Minute}
	return Delivery{ID: "delivery-1", TenantID: "tenant-1", Route: route, Capability: route.Capability, AttemptCount: 1, MaxAttempts: route.MaxAttempts, AttemptStartedAt: at, RequestPayload: json.RawMessage(`{"sale":"one"}`)}
}

func TestProcessorCompletesSuccessfulDelivery(t *testing.T) {
	at := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	repository := &processorRepository{deliveries: []Delivery{deliveryFixture()}}
	registry := ConnectorMap{"TRA_FISCALIZATION:TRA_TEST": connectorFunc(func(context.Context, Route, Delivery) (Result, error) {
		return Result{ProviderReference: "FISCAL-001", Response: json.RawMessage(`{"status":"accepted"}`)}, nil
	})}
	count, err := (Processor{Repository: repository, Registry: registry, Clock: clock.Fixed{Time: at}, WorkerID: "worker-1", BatchSize: 10, Lease: time.Minute}).RunOnce(context.Background())
	if err != nil || count != 1 || len(repository.succeeded) != 1 || repository.succeeded[0].ProviderReference != "FISCAL-001" {
		t.Fatalf("count=%d succeeded=%+v err=%v", count, repository.succeeded, err)
	}
}

func TestProcessorSchedulesRetryThenDeadLettersAtBudget(t *testing.T) {
	at := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	delivery := deliveryFixture()
	repository := &processorRepository{deliveries: []Delivery{delivery}}
	registry := ConnectorMap{"TRA_FISCALIZATION:TRA_TEST": connectorFunc(func(context.Context, Route, Delivery) (Result, error) {
		return Result{}, Failure{Code: "provider_busy", Message: "provider temporarily unavailable", Retryable: true}
	})}
	processor := Processor{Repository: repository, Registry: registry, Clock: clock.Fixed{Time: at}, WorkerID: "worker-1", BatchSize: 10, Lease: time.Minute}
	if count, err := processor.RunOnce(context.Background()); err != nil || count != 0 || len(repository.failed) != 1 || repository.dead[0] || !repository.retryAt[0].After(at) {
		t.Fatalf("retry count=%d failed=%+v dead=%v retry=%v err=%v", count, repository.failed, repository.dead, repository.retryAt, err)
	}
	delivery.AttemptCount = delivery.MaxAttempts
	repository.deliveries, repository.failed, repository.dead, repository.retryAt = []Delivery{delivery}, nil, nil, nil
	if _, err := processor.RunOnce(context.Background()); err != nil || len(repository.dead) != 1 || !repository.dead[0] {
		t.Fatalf("dead-letter failed=%+v dead=%v err=%v", repository.failed, repository.dead, err)
	}
}

func TestProcessorFailsClosedForUnknownConnector(t *testing.T) {
	repository := &processorRepository{deliveries: []Delivery{deliveryFixture()}}
	processor := Processor{Repository: repository, Registry: ConnectorMap{}, Clock: clock.Fixed{Time: time.Now()}, WorkerID: "worker-1", BatchSize: 10, Lease: time.Minute}
	if count, err := processor.RunOnce(context.Background()); err != nil || count != 0 || len(repository.dead) != 1 || !repository.dead[0] || repository.failed[0].Code != "connector_unavailable" {
		t.Fatalf("count=%d failed=%+v dead=%v err=%v", count, repository.failed, repository.dead, err)
	}
}

func TestProcessorRejectsMalformedProviderEvidence(t *testing.T) {
	repository := &processorRepository{deliveries: []Delivery{deliveryFixture()}}
	registry := ConnectorMap{"TRA_FISCALIZATION:TRA_TEST": connectorFunc(func(context.Context, Route, Delivery) (Result, error) {
		return Result{ProviderReference: "FISCAL-INVALID", Response: json.RawMessage(`[]`)}, nil
	})}
	processor := Processor{Repository: repository, Registry: registry, Clock: clock.Fixed{Time: time.Now()}, WorkerID: "worker-1", BatchSize: 10, Lease: time.Minute}
	if count, err := processor.RunOnce(context.Background()); err != nil || count != 0 || len(repository.dead) != 1 || !repository.dead[0] || repository.failed[0].Code != "invalid_provider_response" {
		t.Fatalf("count=%d failed=%+v dead=%v err=%v", count, repository.failed, repository.dead, err)
	}
}

func TestProcessorClassifiesProviderTimeoutForRetry(t *testing.T) {
	delivery := deliveryFixture()
	delivery.Route.Timeout = time.Millisecond
	repository := &processorRepository{deliveries: []Delivery{delivery}}
	registry := ConnectorMap{"TRA_FISCALIZATION:TRA_TEST": connectorFunc(func(ctx context.Context, _ Route, _ Delivery) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	})}
	processor := Processor{Repository: repository, Registry: registry, Clock: clock.Fixed{Time: time.Now()}, WorkerID: "worker-1", BatchSize: 10, Lease: time.Minute}
	if count, err := processor.RunOnce(context.Background()); err != nil || count != 0 || len(repository.failed) != 1 || repository.dead[0] || repository.failed[0].Code != "provider_timeout" {
		t.Fatalf("count=%d failed=%+v dead=%v err=%v", count, repository.failed, repository.dead, err)
	}
}
