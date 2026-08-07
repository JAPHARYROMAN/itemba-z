package treasury_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

func TestFacilityLifecycle(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	ids := &identity.SequenceGenerator{Values: []string{
		"20000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000002", "20000000-0000-4000-8000-000000000003",
		"20000000-0000-4000-8000-000000000004", "20000000-0000-4000-8000-000000000005", "20000000-0000-4000-8000-000000000006",
		"20000000-0000-4000-8000-000000000007", "20000000-0000-4000-8000-000000000008", "20000000-0000-4000-8000-000000000009",
	}}
	store := memory.New()
	svc, err := treasury.NewService(store, ids, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	maker := "maker"
	checker := "checker"
	for _, a := range []string{maker, checker} {
		for _, p := range []string{"finance.treasury.read", "finance.treasury.manage", "finance.treasury.approve", "finance.treasury.transact"} {
			store.SeedPermission(scope, a, p)
		}
	}
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: "tenant", CompanyID: "company", StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	f, err := svc.CreateFacility(ctx, treasury.CreateFacilityCommand{Scope: scope, Reference: "loan-01", Lender: "Umoja Bank", Type: treasury.TermLoan, Currency: "TZS", LimitMinor: 10000, AnnualInterestBasisPoints: 1200, StartDate: at, MaturityDate: at.AddDate(1, 0, 0), BankAccountID: "cash", PrincipalAccountID: "payable", InterestExpenseAccountID: "expense", AccruedInterestAccountID: "accrued", Reason: "Approved working capital facility", ActorID: maker, IdempotencyKey: "facility-create-0001"})
	if err != nil {
		t.Fatal(err)
	}
	f, err = svc.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: f.ID, Status: treasury.Submitted, Reason: "Facility evidence ready for review", ActorID: maker, IdempotencyKey: "facility-submit-0001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: f.ID, Status: treasury.Active, Reason: "Maker attempted own approval", ActorID: maker, IdempotencyKey: "facility-self-approve"})
	if !errors.Is(err, treasury.ErrSeparationOfDuties) {
		t.Fatalf("expected separation error, got %v", err)
	}
	f, err = svc.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: f.ID, Status: treasury.Active, Reason: "Independent facility approval complete", ActorID: checker, IdempotencyKey: "facility-approve-001"})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range []struct {
		typ    treasury.TransactionType
		amount int64
	}{{treasury.Drawdown, 10000}, {treasury.InterestAccrual, 1000}, {treasury.PrincipalRepayment, 10000}, {treasury.InterestPayment, 1000}} {
		f, err = svc.PostTransaction(ctx, treasury.PostTransactionCommand{Scope: scope, FacilityID: f.ID, Type: c.typ, AmountMinor: c.amount, OccurredAt: at, Reason: "Approved treasury posting evidence", ActorID: checker, IdempotencyKey: []string{"treasury-drawdown-001", "treasury-interest-accrual", "treasury-principal-repay", "treasury-interest-payment"}[i]})
		if err != nil {
			t.Fatal(err)
		}
	}
	if f.OutstandingPrincipalMinor != 0 || f.AccruedInterestMinor != 0 || len(f.Transactions) != 4 {
		t.Fatalf("unexpected balances: %+v", f)
	}
	f, err = svc.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: f.ID, Status: treasury.Closed, Reason: "Facility fully settled and reconciled", ActorID: checker, IdempotencyKey: "facility-close-00001"})
	if err != nil || f.Status != treasury.Closed {
		t.Fatalf("close: %+v %v", f, err)
	}
}
