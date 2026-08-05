package receivables_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	customerID = "00000000-0000-4000-8000-000000000101"
	actorID    = "00000000-0000-4000-8000-000000000102"
)

var now = time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)

func newFixture(t *testing.T, general bool) (*memory.Store, *receivables.Service, tenancy.Scope) {
	t.Helper()
	store := memory.New()
	scope := tenancy.Scope{
		TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002",
		BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004",
	}
	store.SeedPermission(scope, actorID, "customers.accounts.read")
	store.SeedPermission(scope, actorID, "customers.credit.manage")
	store.SeedCustomer(customers.Account{ID: customerID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, Name: "Amani Stores", Active: true, General: general})
	ids := &identity.SequenceGenerator{Values: []string{
		"00000000-0000-4000-8000-000000000201", "00000000-0000-4000-8000-000000000202", "00000000-0000-4000-8000-000000000203",
		"00000000-0000-4000-8000-000000000204", "00000000-0000-4000-8000-000000000205", "00000000-0000-4000-8000-000000000206",
	}}
	service, err := receivables.NewService(store, ids, clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	return store, service, scope
}

func TestSchedulePolicyIsEffectiveDatedAndIdempotent(t *testing.T) {
	_, service, scope := newFixture(t, false)
	command := receivables.ScheduleCreditPolicyCommand{
		Scope: scope, ActorID: actorID, CustomerID: customerID, CreditEnabled: true, CreditLimitMinor: 50_000_000,
		PaymentTermsDays: 30, MaxOverdueDays: 7, RiskStatus: customers.CreditRiskWatch,
		Reason: "Approved after documented finance review", EffectiveFrom: now.Add(time.Hour), IdempotencyKey: "policy-request-0001",
	}
	created, err := service.ScheduleCreditPolicy(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := service.ScheduleCreditPolicy(context.Background(), command)
	if err != nil || retried.ID != created.ID {
		t.Fatalf("idempotent retry=%+v err=%v", retried, err)
	}
	detail, err := service.Account(context.Background(), scope, actorID, customerID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ActivePolicy.CreditEnabled || len(detail.ScheduledPolicies) != 1 || detail.ScheduledPolicies[0].RiskStatus != customers.CreditRiskWatch {
		t.Fatalf("unexpected policy timeline: %+v", detail)
	}
}

func TestGeneralCustomerCreditPolicyIsRejected(t *testing.T) {
	_, service, scope := newFixture(t, true)
	_, err := service.ScheduleCreditPolicy(context.Background(), receivables.ScheduleCreditPolicyCommand{
		Scope: scope, ActorID: actorID, CustomerID: customerID, CreditEnabled: true, CreditLimitMinor: 1_000,
		PaymentTermsDays: 7, RiskStatus: customers.CreditRiskStandard, Reason: "Requested by store management",
		EffectiveFrom: now.Add(time.Hour), IdempotencyKey: "policy-request-0002",
	})
	if !errors.Is(err, sales.ErrGeneralCustomerCredit) {
		t.Fatalf("expected general-customer rejection, got %v", err)
	}
}
