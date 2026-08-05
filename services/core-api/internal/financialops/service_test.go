package financialops_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestManualJournalRequiresIndependentPostingAndReversesExactly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	maker, checker := "00000000-0000-4000-8000-000000000005", "00000000-0000-4000-8000-000000000006"
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	store := memory.New()
	for _, actor := range []string{maker, checker} {
		for _, permission := range []string{"finance.journals.read", "finance.journals.manage", "finance.journals.post", "finance.periods.read", "finance.periods.close"} {
			store.SeedPermission(scope, actor, permission)
		}
	}
	store.SeedFiscalPeriod(memory.FiscalPeriod{ID: "00000000-0000-4000-8000-000000000007", TenantID: scope.TenantID, CompanyID: scope.CompanyID, StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	service, err := financialops.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := service.Create(ctx, financialops.CreateCommand{Scope: scope, Type: financialops.ManualJournal, Currency: "TZS", AccountingAt: at, Reason: "Recognize approved opening correction", ActorID: maker, IdempotencyKey: "financial-create-00001", Lines: []finance.JournalEntry{{AccountID: "expense", DebitMinor: 25000, Memo: "Correction"}, {AccountID: "suspense", CreditMinor: 25000, Memo: "Correction"}}})
	if err != nil {
		t.Fatal(err)
	}
	doc, err = service.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: doc.ID, Status: financialops.Submitted, Reason: "Evidence ready for independent review", ActorID: maker, IdempotencyKey: "financial-submit-00001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: doc.ID, Status: financialops.Posted, Reason: "Attempted maker self posting review", ActorID: maker, IdempotencyKey: "financial-self-post-001"})
	if !errors.Is(err, financialops.ErrSeparationOfDuties) {
		t.Fatalf("expected maker-checker rejection, got %v", err)
	}
	doc, err = service.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: doc.ID, Status: financialops.Posted, Reason: "Independent evidence review completed", ActorID: checker, IdempotencyKey: "financial-post-000001"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != financialops.Posted || doc.JournalID == "" {
		t.Fatalf("expected posted journal, got %#v", doc)
	}
	snapshot := store.Snapshot()
	if len(snapshot.Journals) != 1 || snapshot.Journals[0].Validate() != nil {
		t.Fatalf("expected one balanced journal, got %#v", snapshot.Journals)
	}
	reversal, err := service.Create(ctx, financialops.CreateCommand{Scope: scope, Type: financialops.Reversal, Currency: "TZS", AccountingAt: at, Reason: "Reverse approved opening correction", ReversesDocumentID: doc.ID, ActorID: maker, IdempotencyKey: "financial-reverse-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if reversal.Lines[0].CreditMinor != 25000 || reversal.Lines[1].DebitMinor != 25000 {
		t.Fatalf("expected exact inverted lines, got %#v", reversal.Lines)
	}
}

func TestCashTransferDerivesLedgerAccountsAndPeriodCloseIsBlocked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := tenancy.Scope{TenantID: "10000000-0000-4000-8000-000000000001", CompanyID: "10000000-0000-4000-8000-000000000002", BranchID: "10000000-0000-4000-8000-000000000003", WarehouseID: "10000000-0000-4000-8000-000000000004"}
	maker := "10000000-0000-4000-8000-000000000005"
	checker := "10000000-0000-4000-8000-000000000006"
	periodID := "10000000-0000-4000-8000-000000000007"
	fromID := "10000000-0000-4000-8000-000000000008"
	toID := "10000000-0000-4000-8000-000000000009"
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	store := memory.New()
	for _, actor := range []string{maker, checker} {
		for _, p := range []string{"finance.journals.read", "finance.journals.manage", "finance.journals.post", "finance.cash.transfer", "finance.periods.read", "finance.periods.close"} {
			store.SeedPermission(scope, actor, p)
		}
	}
	store.SeedFiscalPeriod(memory.FiscalPeriod{ID: periodID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	store.SeedBankAccount(banking.Account{ID: fromID, Scope: scope, Code: "CASH", Name: "Till", Currency: "TZS", GLAccountID: "cash", Active: true})
	store.SeedBankAccount(banking.Account{ID: toID, Scope: scope, Code: "BANK", Name: "Bank", Currency: "TZS", GLAccountID: "bank", Active: true})
	service, _ := financialops.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	doc, err := service.Create(ctx, financialops.CreateCommand{Scope: scope, Type: financialops.CashTransfer, Currency: "TZS", AccountingAt: at, Reason: "Deposit till cash into bank", FromAccountID: fromID, ToAccountID: toID, ActorID: maker, IdempotencyKey: "transfer-create-00001", Lines: []finance.JournalEntry{{DebitMinor: 50000}}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := service.RequestPeriodAction(ctx, financialops.PeriodCommand{Scope: scope, PeriodID: periodID, Action: financialops.ClosePeriod, Reason: "Period prepared for controlled close", ActorID: maker, IdempotencyKey: "period-close-request-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ApprovePeriodAction(ctx, scope, checker, request.ID, "Independent close review completed", "period-close-approve-1")
	if !errors.Is(err, financialops.ErrPeriodCloseBlocked) {
		t.Fatalf("expected unfinished document close blocker, got %v", err)
	}
	doc, err = service.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: doc.ID, Status: financialops.Submitted, Reason: "Transfer evidence ready for review", ActorID: maker, IdempotencyKey: "transfer-submit-0001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: doc.ID, Status: financialops.Posted, Reason: "Independent transfer review complete", ActorID: checker, IdempotencyKey: "transfer-post-000001"})
	if err != nil {
		t.Fatal(err)
	}
	j := store.Snapshot().Journals[0]
	if j.Entries[0].AccountID != "bank" || j.Entries[1].AccountID != "cash" {
		t.Fatalf("expected server-derived accounts, got %#v", j.Entries)
	}
}
