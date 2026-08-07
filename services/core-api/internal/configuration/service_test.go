package configuration_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"testing"
	"time"
)

func TestGovernedConfigurationAndAtomicNumbering(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	ids := &identity.SequenceGenerator{Values: []string{"24000000-0000-4000-8000-000000000001", "24000000-0000-4000-8000-000000000002", "24000000-0000-4000-8000-000000000003", "24000000-0000-4000-8000-000000000004"}}
	store := memory.New()
	svc, e := configuration.NewService(store, ids, clock.Fixed{Time: at})
	if e != nil {
		t.Fatal(e)
	}
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	maker, checker := "maker", "checker"
	for _, a := range []string{maker, checker} {
		for _, p := range []string{"settings.read", "settings.manage", "settings.approve", "settings.numbering.manage", "settings.numbering.allocate"} {
			store.SeedPermission(scope, a, p)
		}
	}
	v, e := svc.Create(ctx, configuration.CreateCommand{Scope: scope, Category: configuration.HR, Key: "payroll-policy", NameEN: "Payroll policy", NameSW: "Sera ya mishahara", Value: json.RawMessage(`{"parallel_runs":2}`), EffectiveFrom: at, Reason: "Approved payroll control policy", ActorID: maker, IdempotencyKey: "configuration-create1"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = svc.Transition(ctx, configuration.TransitionCommand{Scope: scope, ID: v.ID, Status: configuration.Submitted, Reason: "Configuration submitted for review", ActorID: maker, IdempotencyKey: "configuration-submit1"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = svc.Transition(ctx, configuration.TransitionCommand{Scope: scope, ID: v.ID, Status: configuration.Active, Reason: "Maker attempted own activation", ActorID: maker, IdempotencyKey: "configuration-self-active"})
	if !errors.Is(e, configuration.ErrSeparationOfDuties) {
		t.Fatalf("expected separation error, got %v", e)
	}
	v, e = svc.Transition(ctx, configuration.TransitionCommand{Scope: scope, ID: v.ID, Status: configuration.Active, Reason: "Independent configuration activation", ActorID: checker, IdempotencyKey: "configuration-active1"})
	if e != nil || v.Status != configuration.Active {
		t.Fatalf("activation %+v %v", v, e)
	}
	seq, e := svc.CreateSequence(ctx, configuration.SequenceCommand{Scope: scope, Key: "sales-invoice", Prefix: "INV-", NextValue: 1, Padding: 6, ActorID: maker, IdempotencyKey: "number-sequence-create"})
	if e != nil {
		t.Fatal(e)
	}
	a, e := svc.Allocate(ctx, configuration.AllocateCommand{Scope: scope, SequenceID: seq.ID, ActorID: maker, IdempotencyKey: "number-allocation-001"})
	if e != nil || a.Number != "INV-000001" {
		t.Fatalf("allocation %+v %v", a, e)
	}
	again, e := svc.Allocate(ctx, configuration.AllocateCommand{Scope: scope, SequenceID: seq.ID, ActorID: maker, IdempotencyKey: "number-allocation-001"})
	if e != nil || again.Number != a.Number {
		t.Fatalf("replay %+v %v", again, e)
	}
	view, e := svc.Get(ctx, scope, checker)
	if e != nil || len(view.Versions) != 1 || len(view.Sequences) != 1 || view.Sequences[0].NextValue != 2 {
		t.Fatalf("snapshot %+v %v", view, e)
	}
}
