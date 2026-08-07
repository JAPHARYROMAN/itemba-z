package integrations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
)

type projectionRepository struct {
	called    int
	operation string
	hash      string
}

func (r *projectionRepository) ProjectFiscalDelivery(_ context.Context, _ outbox.Event, _ string, operation, hash string) error {
	r.called, r.operation, r.hash = r.called+1, operation, hash
	return nil
}

func TestProjectorCreatesOnlyFiscalSaleWork(t *testing.T) {
	repository := &projectionRepository{}
	projector, err := NewProjector(repository, &identity.SequenceGenerator{Values: []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := projector.Publish(context.Background(), outbox.Event{EventType: "inventory.adjusted"}); err != nil || repository.called != 0 {
		t.Fatalf("unrelated event called=%d err=%v", repository.called, err)
	}
	payload, _ := json.Marshal(map[string]string{"id": "sale-1"})
	if err := projector.Publish(context.Background(), outbox.Event{EventType: "sale.posted", Payload: payload}); err != nil || repository.called != 1 || repository.operation != "SALES_RECEIPT" || len(repository.hash) != 64 {
		t.Fatalf("posted called=%d operation=%q hash=%q err=%v", repository.called, repository.operation, repository.hash, err)
	}
	if err := projector.Publish(context.Background(), outbox.Event{EventType: "sale.reversed", Payload: payload}); err != nil || repository.called != 2 || repository.operation != "SALES_RETURN" {
		t.Fatalf("reversed called=%d operation=%q err=%v", repository.called, repository.operation, err)
	}
}
