package sales_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

var testTime = time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

type fixture struct {
	store   *memory.Store
	service *sales.Service
	scope   tenancy.Scope
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	store := memory.New()
	scope := tenancy.Scope{TenantID: "tenant-a", CompanyID: "company-a", BranchID: "branch-a", WarehouseID: "warehouse-a"}
	for _, permission := range []string{"sales.complete", "sales.read", "sales.reverse"} {
		store.SeedPermission(scope, "user-1", permission)
		store.SeedPermission(scope, "manager-1", permission)
	}
	store.SeedCustomer(customers.Account{ID: "general", TenantID: scope.TenantID, CompanyID: scope.CompanyID, Name: "General Customer", Active: true, General: true})
	store.SeedCustomer(customers.Account{ID: "customer-1", TenantID: scope.TenantID, CompanyID: scope.CompanyID, Name: "Registered Customer", Active: true, CreditEnabled: true, CreditLimitMinor: 1_000_000})
	store.SeedProduct(catalog.Product{
		ID: "product-1", TenantID: scope.TenantID, CompanyID: scope.CompanyID,
		SKU: "SKU-1", Name: "Product One", Active: true, Currency: "TZS",
		ListPriceMinor: 10_000, StandardCostMinor: 6_000, TaxCode: "VAT",
		RevenueAccountID: "revenue", COGSAccountID: "cogs", InventoryAccountID: "inventory",
	})
	store.SeedTaxRate(memory.TaxRate{TenantID: scope.TenantID, CompanyID: scope.CompanyID, Code: "VAT", BasisPoints: 1800, EffectiveFrom: testTime.AddDate(-1, 0, 0)})
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: scope.TenantID, CompanyID: scope.CompanyID, StartsAt: testTime.AddDate(0, -1, 0), EndsAt: testTime.AddDate(0, 1, 0), Open: true})
	store.SeedPostingConfig(scope.TenantID, scope.CompanyID, finance.SalesPostingConfig{
		ReceivableAccountID: "receivable", TaxPayableAccountID: "tax-payable",
		CashAccounts: map[string]string{
			sales.PaymentCash: "cash-on-hand", sales.PaymentMobileMoney: "mobile-money-clearing",
			sales.PaymentBankCard: "bank-card-clearing", sales.PaymentBankTransfer: "bank-current",
		},
	})
	store.SeedStock(inventory.Movement{
		ID: "opening-stock", TenantID: scope.TenantID, CompanyID: scope.CompanyID,
		BranchID: scope.BranchID, WarehouseID: scope.WarehouseID, ProductID: "product-1",
		SourceType: "OPENING", SourceID: "opening-1", Quantity: 100, OccurredAt: testTime.Add(-time.Hour),
	})
	service, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return fixture{store: store, service: service, scope: scope}
}

func (f fixture) cashCommand(key string) sales.CompleteCommand {
	return sales.CompleteCommand{
		Scope: f.scope, CustomerID: "general", Kind: sales.KindCash,
		PaymentMethod: "cash", Lines: []sales.CommandLine{{ProductID: "product-1", Quantity: 2}},
		ActorID: "user-1", IdempotencyKey: key,
	}
}

func TestGeneralCustomerCannotBuyOnCredit(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("credit-general")
	command.Kind, command.PaymentMethod = sales.KindCredit, ""
	_, err := f.service.Complete(context.Background(), command)
	if !errors.Is(err, sales.ErrGeneralCustomerCredit) {
		t.Fatalf("expected General Customer rejection, got %v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Payments) != 0 || len(snapshot.Outbox) != 0 {
		t.Fatalf("rejected sale left effects: %+v", snapshot)
	}
	if len(snapshot.Movements) != 1 {
		t.Fatalf("opening stock was changed on rejection")
	}
}

func TestGoldenCashSaleIsAtomicAndBalanced(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("golden-cash")
	command.CorrelationID = "019fcb0e-dc88-4372-8ee4-379e29e8f003"
	created, err := f.service.Complete(context.Background(), command)
	if err != nil {
		t.Fatalf("complete sale: %v", err)
	}
	if created.SubtotalMinor != 20_000 || created.TaxMinor != 3_600 || created.TotalMinor != 23_600 || created.COGSMinor != 12_000 {
		t.Fatalf("unexpected computed totals: %+v", created)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 1 || len(snapshot.Payments) != 1 || len(snapshot.Journals) != 1 || len(snapshot.Audits) != 1 || len(snapshot.Outbox) != 1 {
		t.Fatalf("golden transaction effects missing: %+v", snapshot)
	}
	if err := snapshot.Journals[0].Validate(); err != nil {
		t.Fatalf("journal is not balanced: %v", err)
	}
	if got := stockTotal(snapshot, f.scope, "product-1"); got != 98 {
		t.Fatalf("stock=%d want 98", got)
	}
	if snapshot.Payments[0].AmountMinor != created.TotalMinor {
		t.Fatalf("payment does not settle sale")
	}
	if snapshot.Audits[0].EntityID != created.ID || snapshot.Outbox[0].AggregateID != created.ID || snapshot.Outbox[0].EventType != "sale.posted" {
		t.Fatalf("audit/outbox correlation was lost")
	}
	if created.CorrelationID == "" || snapshot.Audits[0].CorrelationID != created.CorrelationID || snapshot.Outbox[0].CorrelationID != created.CorrelationID {
		t.Fatalf("correlation was not propagated across transaction effects")
	}
	var eventSale sales.Sale
	if err := json.Unmarshal(snapshot.Outbox[0].Payload, &eventSale); err != nil || eventSale.ID != created.ID {
		t.Fatalf("outbox payload cannot be traced to sale: %+v %v", eventSale, err)
	}
}

func TestUnauthorizedActorCannotPost(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("unauthorized")
	command.ActorID = "intruder"
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("unauthorized command left effects: %+v", snapshot)
	}
}

func TestUnsupportedPaymentMethodIsABusinessRejection(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("unsupported-payment")
	command.PaymentMethod = "CHEQUE"
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrUnsupportedPayment) {
		t.Fatalf("unsupported payment error=%v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Payments) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Outbox) != 0 {
		t.Fatalf("unsupported payment left transaction effects: %+v", snapshot)
	}
}

func TestCanonicalButUnconfiguredPaymentMethodIsABusinessRejection(t *testing.T) {
	f := newFixture(t)
	f.store.SeedPostingConfig(f.scope.TenantID, f.scope.CompanyID, finance.SalesPostingConfig{
		ReceivableAccountID: "receivable", TaxPayableAccountID: "tax-payable",
		CashAccounts: map[string]string{sales.PaymentCash: "cash-on-hand"},
	})
	command := f.cashCommand("unconfigured-canonical-payment")
	command.PaymentMethod = sales.PaymentBankCard
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrUnsupportedPayment) {
		t.Fatalf("unconfigured canonical payment error=%v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Payments) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Outbox) != 0 {
		t.Fatalf("unconfigured canonical payment left transaction effects: %+v", snapshot)
	}
}

func TestCanonicalPaymentMethodsPostToDistinctAccounts(t *testing.T) {
	methods := []string{sales.PaymentCash, sales.PaymentMobileMoney, sales.PaymentBankCard, sales.PaymentBankTransfer}
	accounts := make(map[string]bool, len(methods))
	for _, method := range methods {
		f := newFixture(t)
		command := f.cashCommand("canonical-payment-" + method)
		command.PaymentMethod = method
		created, err := f.service.Complete(context.Background(), command)
		if err != nil {
			t.Fatalf("method %s: %v", method, err)
		}
		snapshot := f.store.Snapshot()
		if created.PaymentMethod != method || len(snapshot.Payments) != 1 || snapshot.Payments[0].Method != method {
			t.Fatalf("method %s was not preserved: sale=%+v payments=%+v", method, created, snapshot.Payments)
		}
		if accounts[snapshot.Payments[0].AccountID] {
			t.Fatalf("method %s reused settlement account %q", method, snapshot.Payments[0].AccountID)
		}
		accounts[snapshot.Payments[0].AccountID] = true
	}
}

func TestSaleCannotBeReadOrReversedThroughAnotherBranchScope(t *testing.T) {
	f := newFixture(t)
	created, err := f.service.Complete(context.Background(), f.cashCommand("branch-scope-sale"))
	if err != nil {
		t.Fatal(err)
	}
	otherScope := f.scope
	otherScope.BranchID, otherScope.WarehouseID = "branch-b", "warehouse-b"
	f.store.SeedPermission(otherScope, "user-1", "sales.read")
	f.store.SeedPermission(otherScope, "user-1", "sales.reverse")
	if _, err := f.service.Get(context.Background(), otherScope, "user-1", created.ID); !errors.Is(err, sales.ErrNotFound) {
		t.Fatalf("cross-branch read error=%v", err)
	}
	if _, err := f.service.Reverse(context.Background(), sales.ReverseCommand{
		Scope: otherScope, SaleID: created.ID, Reason: "cross-branch attempt",
		ActorID: "user-1", IdempotencyKey: "cross-branch-reversal",
	}); !errors.Is(err, sales.ErrNotFound) {
		t.Fatalf("cross-branch reversal error=%v", err)
	}
}

func TestClosedPeriodRejectsPostingWithoutEffects(t *testing.T) {
	f := newFixture(t)
	f.store.ReplaceFiscalPeriods(memory.FiscalPeriod{
		TenantID: f.scope.TenantID, CompanyID: f.scope.CompanyID,
		StartsAt: testTime.AddDate(0, -1, 0), EndsAt: testTime.AddDate(0, 1, 0), Open: false,
	})
	if _, err := f.service.Complete(context.Background(), f.cashCommand("closed-period")); !errors.Is(err, sales.ErrFiscalPeriodClosed) {
		t.Fatalf("expected closed period, got %v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("closed-period command left effects: %+v", snapshot)
	}
}

func TestTransactionRollsBackPartialCrossModuleEffects(t *testing.T) {
	f := newFixture(t)
	f.store.SeedProduct(catalog.Product{
		ID: "product-2", TenantID: f.scope.TenantID, CompanyID: f.scope.CompanyID,
		SKU: "SKU-2", Name: "Product Two", Active: true, Currency: "TZS",
		ListPriceMinor: 10_000, StandardCostMinor: 6_000, TaxCode: "VAT",
		RevenueAccountID: "revenue", COGSAccountID: "cogs", InventoryAccountID: "inventory",
	})
	command := f.cashCommand("rollback")
	command.Lines = []sales.CommandLine{
		{ProductID: "product-1", Quantity: 1},
		{ProductID: "product-2", Quantity: 200},
	}
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrInsufficientStock) {
		t.Fatalf("expected stock rejection, got %v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Payments) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Audits) != 0 || len(snapshot.Outbox) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("transaction did not fully roll back: %+v", snapshot)
	}
}

func TestDuplicateProductLinesAreRejectedBeforeAllocationOrPosting(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("duplicate-product-lines")
	command.Lines = []sales.CommandLine{
		{ProductID: "product-1", Quantity: 6},
		{ProductID: "product-1", Quantity: 6},
	}
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrDuplicateProductLine) {
		t.Fatalf("expected duplicate product rejection, got %v", err)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("duplicate product command left effects: %+v", snapshot)
	}
}

func TestDuplicateProductLinesUseCanonicalIdentity(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("canonical-duplicate-product-lines")
	command.Lines = []sales.CommandLine{
		{ProductID: "product-1", Quantity: 1},
		{ProductID: " PRODUCT-1 ", Quantity: 1},
	}
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrDuplicateProductLine) {
		t.Fatalf("canonical duplicate error=%v", err)
	}
	if snapshot := f.store.Snapshot(); len(snapshot.Sales) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("canonical duplicate left effects: %+v", snapshot)
	}
}

func TestUnsafeJSONIntegerIsRejectedWithoutEffects(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("unsafe-wire-integer")
	command.Lines[0].Quantity = sales.MaxWireSafeInteger + 1
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrUnsafeWireInteger) {
		t.Fatalf("unsafe integer error=%v", err)
	}
	if snapshot := f.store.Snapshot(); len(snapshot.Sales) != 0 || len(snapshot.Payments) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Outbox) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("unsafe integer left effects: %+v", snapshot)
	}
}

func TestConcurrentIdempotencyPostsStockExactlyOnce(t *testing.T) {
	f := newFixture(t)
	const workers = 16
	ids := make(chan string, workers)
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := f.service.Complete(context.Background(), f.cashCommand("mobile-device-1:tx-99"))
			if err != nil {
				errs <- err
				return
			}
			ids <- result.ID
		}()
	}
	wait.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Errorf("concurrent complete: %v", err)
	}
	var expected string
	for id := range ids {
		if expected == "" {
			expected = id
		}
		if id != expected {
			t.Errorf("idempotent result changed: %s != %s", id, expected)
		}
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 1 || len(snapshot.Payments) != 1 || len(snapshot.Journals) != 1 {
		t.Fatalf("duplicate effects: %+v", snapshot)
	}
	if got := stockTotal(snapshot, f.scope, "product-1"); got != 98 {
		t.Fatalf("stock was not reduced exactly once: %d", got)
	}
}

func TestIdempotencyKeyRejectsDifferentPayload(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("same-key")
	if _, err := f.service.Complete(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	command.Lines[0].Quantity = 3
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrIdempotencyConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCrossTenantIsolation(t *testing.T) {
	f := newFixture(t)
	created, err := f.service.Complete(context.Background(), f.cashCommand("tenant-a-sale"))
	if err != nil {
		t.Fatal(err)
	}
	foreignScope := f.scope
	foreignScope.TenantID = "tenant-b"
	if _, err := f.service.Get(context.Background(), foreignScope, "user-1", created.ID); !errors.Is(err, sales.ErrForbidden) {
		t.Fatalf("cross-tenant read leaked: %v", err)
	}
	command := f.cashCommand("tenant-b-sale")
	command.Scope = foreignScope
	if _, err := f.service.Complete(context.Background(), command); !errors.Is(err, sales.ErrForbidden) {
		t.Fatalf("cross-tenant command did not fail closed: %v", err)
	}
}

func TestReversalRestoresStockReceivableAndFinance(t *testing.T) {
	f := newFixture(t)
	command := f.cashCommand("credit-sale")
	command.CustomerID, command.Kind, command.PaymentMethod = "customer-1", sales.KindCredit, ""
	original, err := f.service.Complete(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	reversal, err := f.service.Reverse(context.Background(), sales.ReverseCommand{
		Scope: f.scope, SaleID: original.ID, Reason: "Customer returned goods",
		ActorID: "manager-1", IdempotencyKey: "reverse-credit-sale",
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if reversal.RecordType != sales.RecordReversal || reversal.ReversalOf != original.ID {
		t.Fatalf("invalid reversal link: %+v", reversal)
	}
	snapshot := f.store.Snapshot()
	if len(snapshot.Sales) != 2 || len(snapshot.Journals) != 2 || len(snapshot.Ledger) != 2 || len(snapshot.Audits) != 2 || len(snapshot.Outbox) != 2 {
		t.Fatalf("reversal effects incomplete: %+v", snapshot)
	}
	if got := stockTotal(snapshot, f.scope, "product-1"); got != 100 {
		t.Fatalf("stock was not restored: %d", got)
	}
	var exposure int64
	for _, entry := range snapshot.Ledger {
		exposure += entry.AmountMinor
	}
	if exposure != 0 {
		t.Fatalf("receivable was not reversed: %d", exposure)
	}
	for _, journal := range snapshot.Journals {
		if err := journal.Validate(); err != nil {
			t.Fatalf("unbalanced journal: %v", err)
		}
	}
	balances := map[string]int64{}
	for _, journal := range snapshot.Journals {
		for _, entry := range journal.Entries {
			balances[entry.AccountID] += entry.DebitMinor - entry.CreditMinor
		}
	}
	for account, balance := range balances {
		if balance != 0 {
			t.Errorf("account %s was not restored: %d", account, balance)
		}
	}
	storedOriginal, err := f.service.Get(context.Background(), f.scope, "manager-1", original.ID)
	if err != nil || storedOriginal.Status != sales.StatusReversed || storedOriginal.ReversedAt == nil {
		t.Fatalf("original remains mutable/active: %+v %v", storedOriginal, err)
	}
	repeated, err := f.service.Reverse(context.Background(), sales.ReverseCommand{Scope: f.scope, SaleID: original.ID, Reason: "Customer returned goods", ActorID: "manager-1", IdempotencyKey: "reverse-credit-sale"})
	if err != nil || repeated.ID != reversal.ID {
		t.Fatalf("reversal was not idempotent: %+v %v", repeated, err)
	}
}

func stockTotal(snapshot memory.Snapshot, scope tenancy.Scope, productID string) int64 {
	var total int64
	for _, movement := range snapshot.Movements {
		if movement.TenantID == scope.TenantID && movement.CompanyID == scope.CompanyID && movement.BranchID == scope.BranchID && movement.WarehouseID == scope.WarehouseID && movement.ProductID == productID {
			total += movement.Quantity
		}
	}
	return total
}
