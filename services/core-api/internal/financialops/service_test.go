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
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
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
	store.SeedGLAccount(financialops.GLAccount{RecordID: "00000000-0000-4000-8000-000000000008", ID: "expense", TenantID: scope.TenantID, CompanyID: scope.CompanyID, Code: "expense", Name: "Expense", Type: financialops.AccountExpense, AllowManualPosting: true, Status: financialops.GovernanceActive})
	store.SeedGLAccount(financialops.GLAccount{RecordID: "00000000-0000-4000-8000-000000000009", ID: "suspense", TenantID: scope.TenantID, CompanyID: scope.CompanyID, Code: "suspense", Name: "Suspense", Type: financialops.AccountAsset, AllowManualPosting: true, Status: financialops.GovernanceActive})
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

func TestChartOfAccountsAndPostingMappingsRequireIndependentApproval(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := tenancy.Scope{TenantID: "20000000-0000-4000-8000-000000000001", CompanyID: "20000000-0000-4000-8000-000000000002", BranchID: "20000000-0000-4000-8000-000000000003", WarehouseID: "20000000-0000-4000-8000-000000000004"}
	maker, checker := "20000000-0000-4000-8000-000000000005", "20000000-0000-4000-8000-000000000006"
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	store := memory.New()
	store.SeedPostingConfig(scope.TenantID, scope.CompanyID, finance.SalesPostingConfig{ReceivableAccountID: "legacy-receivable", TaxPayableAccountID: "legacy-tax", CashAccounts: map[string]string{sales.PaymentCash: "legacy-cash"}})
	for _, actor := range []string{maker, checker} {
		for _, permission := range []string{"finance.accounts.read", "finance.accounts.manage", "finance.accounts.approve"} {
			store.SeedPermission(scope, actor, permission)
		}
	}
	service, err := financialops.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	account, err := service.CreateAccount(ctx, financialops.CreateAccountCommand{Scope: scope, Code: "1100-cash", Name: "Cash on hand", Type: financialops.AccountAsset, ActorID: maker, IdempotencyKey: "account-create-0001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.DecideAccount(ctx, financialops.DecideAccountCommand{Scope: scope, AccountID: account.ID, Status: financialops.GovernanceActive, Reason: "Reviewed account classification", ActorID: maker, IdempotencyKey: "account-self-approve-1"})
	if !errors.Is(err, financialops.ErrAccountGovernance) {
		t.Fatalf("expected maker-checker rejection, got %v", err)
	}
	account, err = service.DecideAccount(ctx, financialops.DecideAccountCommand{Scope: scope, AccountID: account.ID, Status: financialops.GovernanceActive, Reason: "Reviewed account classification", ActorID: checker, IdempotencyKey: "account-approve-0001"})
	if err != nil || account.Status != financialops.GovernanceActive {
		t.Fatalf("expected active account, got %#v, %v", account, err)
	}
	_, err = service.CreateMapping(ctx, financialops.CreateMappingCommand{Scope: scope, Key: financialops.MapSalesTaxPayable, AccountID: account.ID, EffectiveFrom: at, Reason: "Configure tax posting account", ActorID: maker, IdempotencyKey: "mapping-invalid-0001"})
	if !errors.Is(err, financialops.ErrAccountGovernance) {
		t.Fatalf("expected incompatible mapping rejection, got %v", err)
	}
	mapping, err := service.CreateMapping(ctx, financialops.CreateMappingCommand{Scope: scope, Key: financialops.MapPaymentCash, AccountID: account.ID, EffectiveFrom: at.Add(-time.Hour), Reason: "Configure cash posting account", ActorID: maker, IdempotencyKey: "mapping-create-00001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.DecideMapping(ctx, financialops.DecideMappingCommand{Scope: scope, MappingID: mapping.ID, Status: financialops.GovernanceActive, Reason: "Reviewed effective posting rule", ActorID: checker, IdempotencyKey: "mapping-approve-0001"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Mappings(ctx, scope, checker)
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != financialops.GovernanceActive {
		t.Fatalf("expected one active mapping, got %#v, %v", page, err)
	}
	if err = store.WithTransaction(ctx, func(tx sales.Transaction) error {
		config, configErr := tx.SalesPostingConfig(ctx, scope)
		if configErr != nil {
			return configErr
		}
		if config.CashAccounts[sales.PaymentCash] != account.ID {
			t.Fatalf("expected approved mapping to govern sales posting, got %#v", config.CashAccounts)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
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
