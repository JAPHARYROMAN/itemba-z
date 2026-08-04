package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
)

type fakeRepository struct {
	events    []Event
	published []string
	failed    []string
}

func (f *fakeRepository) TenantIDs(context.Context) ([]string, error) {
	return []string{"tenant-1"}, nil
}
func (f *fakeRepository) Claim(context.Context, string, string, int, time.Time, time.Time) ([]Event, error) {
	return append([]Event(nil), f.events...), nil
}
func (f *fakeRepository) MarkPublished(_ context.Context, _, eventID, _ string, _ time.Time) error {
	f.published = append(f.published, eventID)
	return nil
}
func (f *fakeRepository) MarkFailed(_ context.Context, _, eventID, _ string, _ time.Time, _ string) error {
	f.failed = append(f.failed, eventID)
	return nil
}

type fakePublisher struct{ failID string }

func (f fakePublisher) Publish(_ context.Context, event Event) error {
	if event.ID == f.failID {
		return errors.New("broker unavailable")
	}
	return nil
}

func TestProcessorMarksSuccessAndSchedulesFailure(t *testing.T) {
	repository := &fakeRepository{events: []Event{{ID: "event-1", TenantID: "tenant-1"}, {ID: "event-2", TenantID: "tenant-1"}}}
	processor := Processor{Repository: repository, Publisher: fakePublisher{failID: "event-2"},
		Clock: clock.Fixed{Time: time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)}, WorkerID: "worker-1",
		BatchSize: 10, Lease: time.Minute, RetryDelay: 5 * time.Minute}
	count, err := processor.RunOnce(context.Background())
	if count != 1 || err == nil {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if len(repository.published) != 1 || repository.published[0] != "event-1" {
		t.Fatalf("published=%v", repository.published)
	}
	if len(repository.failed) != 1 || repository.failed[0] != "event-2" {
		t.Fatalf("failed=%v", repository.failed)
	}
}
