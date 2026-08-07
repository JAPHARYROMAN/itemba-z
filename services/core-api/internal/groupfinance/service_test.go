package groupfinance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestDualCompanyApprovalAndConsolidation(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	ids := &identity.SequenceGenerator{Values: []string{"30000000-0000-4000-8000-000000000001", "30000000-0000-4000-8000-000000000002", "30000000-0000-4000-8000-000000000003"}}
	store := memory.New()
	svc, err := groupfinance.NewService(store, ids, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	counterCompany := "00000000-0000-4000-8000-000000000009"
	source := tenancy.Scope{TenantID: "tenant", CompanyID: "source", BranchID: "source-branch", WarehouseID: "source-warehouse"}
	counter := tenancy.Scope{TenantID: "tenant", CompanyID: counterCompany, BranchID: "counter-branch", WarehouseID: "counter-warehouse"}
	maker, sourceApprover, counterApprover := "maker", "source-approver", "counter-approver"
	for _, grant := range []struct {
		scope             tenancy.Scope
		actor, permission string
	}{{source, maker, "finance.intercompany.manage"}, {source, maker, "finance.intercompany.approve"}, {source, sourceApprover, "finance.intercompany.approve"}, {counter, counterApprover, "finance.intercompany.approve"}, {source, sourceApprover, "finance.consolidation.read"}} {
		store.SeedPermission(grant.scope, grant.actor, grant.permission)
	}
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: "tenant", CompanyID: "source", StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: "tenant", CompanyID: counterCompany, StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	for _, a := range []financialops.GLAccount{{ID: "ic-receivable", TenantID: "tenant", CompanyID: "source", Type: financialops.AccountAsset}, {ID: "recovery", TenantID: "tenant", CompanyID: "source", Type: financialops.AccountRevenue}, {ID: "allocated-expense", TenantID: "tenant", CompanyID: counterCompany, Type: financialops.AccountExpense}, {ID: "ic-payable", TenantID: "tenant", CompanyID: counterCompany, Type: financialops.AccountLiability}} {
		store.SeedGLAccount(a)
	}
	v, err := svc.Create(ctx, groupfinance.CreateCommand{Scope: source, CounterpartyCompanyID: counterCompany, Reference: "alloc-01", Type: groupfinance.CostAllocation, Currency: "TZS", AmountMinor: 5000, OccurredAt: at, SourceDebitAccountID: "ic-receivable", SourceCreditAccountID: "recovery", CounterpartyDebitAccountID: "allocated-expense", CounterpartyCreditAccountID: "ic-payable", Reason: "Approved shared service allocation", ActorID: maker, IdempotencyKey: "group-create-00001"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = svc.Transition(ctx, groupfinance.TransitionCommand{Scope: source, TransactionID: v.ID, Status: groupfinance.Submitted, Reason: "Allocation evidence ready for review", ActorID: maker, IdempotencyKey: "group-submit-00001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Transition(ctx, groupfinance.TransitionCommand{Scope: source, TransactionID: v.ID, Status: groupfinance.SourceApproved, Reason: "Maker attempted source approval", ActorID: maker, IdempotencyKey: "group-self-approve1"})
	if !errors.Is(err, groupfinance.ErrSeparationOfDuties) {
		t.Fatalf("expected separation failure, got %v", err)
	}
	v, err = svc.Transition(ctx, groupfinance.TransitionCommand{Scope: source, TransactionID: v.ID, Status: groupfinance.SourceApproved, Reason: "Independent source approval complete", ActorID: sourceApprover, IdempotencyKey: "group-source-approve"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = svc.Transition(ctx, groupfinance.TransitionCommand{Scope: counter, TransactionID: v.ID, Status: groupfinance.Posted, Reason: "Counterparty confirmed allocation", ActorID: counterApprover, IdempotencyKey: "group-counter-post1"})
	if err != nil || v.Status != groupfinance.Posted {
		t.Fatalf("post: %+v %v", v, err)
	}
	r, err := svc.Consolidated(ctx, source, sourceApprover, at.Add(time.Hour))
	if err != nil || r.IntercompanyBalanceEliminationMinor != 5000 || r.IntercompanyActivityEliminationMinor != 5000 || !r.Balanced {
		t.Fatalf("consolidation: %+v %v", r, err)
	}
}
