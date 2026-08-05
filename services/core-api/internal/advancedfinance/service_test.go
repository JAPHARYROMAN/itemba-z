package advancedfinance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestGovernedBudgetAndFixedAssetLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	maker, checker := "00000000-0000-4000-8000-000000000005", "00000000-0000-4000-8000-000000000006"
	account := func(n int) string {
		return "00000000-0000-4000-8000-" + map[int]string{1: "000000000101", 2: "000000000102", 3: "000000000103", 4: "000000000104", 5: "000000000105", 6: "000000000106", 7: "000000000107"}[n]
	}
	store := memory.New()
	for _, actor := range []string{maker, checker} {
		for _, p := range []string{"finance.budgets.read", "finance.budgets.manage", "finance.budgets.approve", "finance.assets.read", "finance.assets.manage", "finance.assets.approve", "finance.assets.depreciate", "finance.assets.dispose"} {
			store.SeedPermission(scope, actor, p)
		}
	}
	ids := &identity.SequenceGenerator{Values: []string{"10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002", "10000000-0000-4000-8000-000000000003", "10000000-0000-4000-8000-000000000004", "10000000-0000-4000-8000-000000000005", "10000000-0000-4000-8000-000000000006"}}
	service, err := advancedfinance.NewService(store, ids, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.CreateBudget(ctx, advancedfinance.CreateBudgetCommand{Scope: scope, Name: "Approved operating budget", FiscalYear: 2026, Currency: "TZS", Reason: "Annual plan approved for review", ActorID: maker, IdempotencyKey: "budget-create-00001", Lines: []advancedfinance.BudgetLine{{AccountID: account(1), Month: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), AmountMinor: 2_000_000}}})
	if err != nil {
		t.Fatal(err)
	}
	b, err = service.TransitionBudget(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: b.ID, Status: advancedfinance.Submitted, Reason: "Budget evidence ready for review", ActorID: maker, IdempotencyKey: "budget-submit-00001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.TransitionBudget(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: b.ID, Status: advancedfinance.Approved, Reason: "Maker attempted own approval", ActorID: maker, IdempotencyKey: "budget-self-approve1"})
	if !errors.Is(err, advancedfinance.ErrSeparationOfDuties) {
		t.Fatalf("expected maker-checker rejection, got %v", err)
	}
	b, err = service.TransitionBudget(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: b.ID, Status: advancedfinance.Approved, Reason: "Independent budget review completed", ActorID: checker, IdempotencyKey: "budget-approve-0001"})
	if err != nil || b.Status != advancedfinance.Approved {
		t.Fatalf("approve budget: %#v %v", b, err)
	}
	a, err := service.CreateAsset(ctx, advancedfinance.CreateAssetCommand{Scope: scope, Code: "VEH-001", Name: "Delivery vehicle", Category: "Vehicles", Currency: "TZS", AcquiredAt: at, CostMinor: 12_000_000, ResidualMinor: 0, UsefulLifeMonths: 12, AssetAccountID: account(1), AccumulatedDepreciationAccountID: account(2), DepreciationExpenseAccountID: account(3), CapitalizationOffsetAccountID: account(4), DisposalGainAccountID: account(5), DisposalLossAccountID: account(6), Reason: "Approved delivery vehicle capitalization", ActorID: maker, IdempotencyKey: "asset-create-000001"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = service.TransitionAsset(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: a.ID, Status: advancedfinance.Submitted, Reason: "Asset evidence ready for review", ActorID: maker, IdempotencyKey: "asset-submit-000001"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = service.TransitionAsset(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: a.ID, Status: advancedfinance.Active, Reason: "Independent capitalization approved", ActorID: checker, IdempotencyKey: "asset-activate-0001"})
	if err != nil || a.Status != advancedfinance.Active {
		t.Fatalf("activate asset: %#v %v", a, err)
	}
	d, err := service.Depreciate(ctx, advancedfinance.DepreciateCommand{Scope: scope, AssetID: a.ID, Period: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Reason: "Monthly straight line depreciation", ActorID: checker, IdempotencyKey: "asset-depreciate-01"})
	if err != nil || d.AmountMinor != 1_000_000 {
		t.Fatalf("depreciation: %#v %v", d, err)
	}
	a, err = service.Dispose(ctx, advancedfinance.DisposeCommand{Scope: scope, AssetID: a.ID, ProceedsMinor: 10_500_000, ProceedsAccountID: account(7), Reason: "Approved vehicle disposal proceeds", ActorID: checker, IdempotencyKey: "asset-disposal-0001"})
	if err != nil || a.Status != advancedfinance.Disposed {
		t.Fatalf("dispose: %#v %v", a, err)
	}
	for _, j := range store.Snapshot().Journals {
		if err = j.Validate(); err != nil {
			t.Fatalf("unbalanced journal %#v: %v", j, err)
		}
	}
}
