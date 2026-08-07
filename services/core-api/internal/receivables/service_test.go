package receivables_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
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
		"00000000-0000-4000-8000-000000000207",
	}}
	service, err := receivables.NewService(store, ids, clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	return store, service, scope
}

func TestCollectionAllocatesInvoiceAndPostsBalancedJournal(t *testing.T) {
	store, service, scope := newFixture(t, false)
	invoiceID := "00000000-0000-4000-8000-000000000301"
	itemID := "00000000-0000-4000-8000-000000000302"
	store.SeedPermission(scope, actorID, "customers.collections.post")
	store.SeedPostingConfig(scope.TenantID, scope.CompanyID, finance.SalesPostingConfig{ReceivableAccountID: "receivable", TaxPayableAccountID: "tax", CashAccounts: map[string]string{sales.PaymentBankTransfer: "bank"}})
	store.SeedCustomerLedger(customers.LedgerEntry{ID: "opening", TenantID: scope.TenantID, CompanyID: scope.CompanyID, CustomerID: customerID, SourceType: "SALE", SourceID: invoiceID, AmountMinor: 1000, Currency: "TZS", OccurredAt: now.Add(-time.Hour)})
	due := now.AddDate(0, 0, 30)
	store.SeedReceivableItem(customers.ReceivableItem{ID: itemID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, CustomerID: customerID, Kind: customers.ReceivableInvoice, SourceType: "SALE", SourceID: invoiceID, AmountMinor: 1000, Currency: "TZS", DocumentAt: now.Add(-time.Hour), DueAt: &due, OccurredAt: now.Add(-time.Hour)})
	created, err := service.ReceiveCollection(context.Background(), receivables.ReceiveCollectionCommand{Scope: scope, ActorID: actorID, CustomerID: customerID, InvoiceSaleID: invoiceID, Method: sales.PaymentBankTransfer, AmountMinor: 600, Currency: "TZS", IdempotencyKey: "collection-request-001"})
	if err != nil {
		t.Fatal(err)
	}
	if created.AccountID != "bank" {
		t.Fatalf("account=%s", created.AccountID)
	}
	detail, err := service.Account(context.Background(), scope, actorID, customerID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Aging.LedgerBalanceMinor != 400 || detail.Aging.OpenInvoiceMinor != 400 || !detail.Aging.Reconciled {
		t.Fatalf("aging=%+v", detail.Aging)
	}
	snapshot := store.Snapshot()
	if len(snapshot.Journals) != 1 || snapshot.Journals[0].Validate() != nil {
		t.Fatalf("journal=%+v", snapshot.Journals)
	}
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
