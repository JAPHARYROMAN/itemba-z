package banking_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestStatementRequiresExactLedgerMatchAndIndependentApproval(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	maker, approver := "00000000-0000-4000-8000-000000000005", "00000000-0000-4000-8000-000000000006"
	accountID, journalID, sourceID := "00000000-0000-4000-8000-000000000007", "00000000-0000-4000-8000-000000000008", "00000000-0000-4000-8000-000000000009"
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	store := memory.New()
	for _, permission := range []string{"finance.bank.read", "finance.bank.import", "finance.bank.match", "finance.bank.reconcile"} {
		store.SeedPermission(scope, maker, permission)
		store.SeedPermission(scope, approver, permission)
	}
	store.SeedBankAccount(banking.Account{ID: accountID, Scope: scope, Code: "CRDB-TZS", Name: "CRDB current account", Type: banking.BankAccount, Currency: "TZS", GLAccountID: "bank-current", Active: true})
	err := store.WithTransaction(ctx, func(tx sales.Transaction) error {
		return tx.CreateJournal(ctx, finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "CUSTOMER_COLLECTION", SourceID: sourceID, Currency: "TZS", OccurredAt: at, Entries: []finance.JournalEntry{{AccountID: "bank-current", DebitMinor: 25_000, Memo: "Customer collection"}, {AccountID: "receivable", CreditMinor: 25_000, Memo: "Receivable settled"}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	service, _ := banking.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at.Add(time.Hour)})
	statement, err := service.Import(ctx, banking.ImportCommand{Scope: scope, AccountID: accountID, ExternalReference: "CRDB-2026-08-05", Currency: "TZS", PeriodStart: at.Add(-time.Hour), PeriodEnd: at.Add(time.Hour), OpeningMinor: 100_000, ClosingMinor: 125_000, Lines: []banking.ImportLine{{TransactionAt: at, ExternalReference: "DEP-100", Description: "Customer bank transfer", AmountMinor: 25_000}}, ActorID: maker, IdempotencyKey: "bank-import-request-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(statement.Lines) != 1 {
		t.Fatalf("expected one statement line, got %d", len(statement.Lines))
	}
	duplicate := banking.ImportCommand{Scope: scope, AccountID: accountID, ExternalReference: "CRDB-2026-08-05", Currency: "TZS", PeriodStart: at.Add(-time.Hour), PeriodEnd: at.Add(time.Hour), OpeningMinor: 100_000, ClosingMinor: 125_000, Lines: []banking.ImportLine{{TransactionAt: at, ExternalReference: "DEP-100", Description: "Customer bank transfer", AmountMinor: 25_000}}, ActorID: maker, IdempotencyKey: "bank-import-duplicate-01"}
	if _, err := service.Import(ctx, duplicate); !errors.Is(err, banking.ErrDuplicateStatement) {
		t.Fatalf("expected duplicate reference rejection, got %v", err)
	}
	detail, err := service.Statement(ctx, scope, maker, statement.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Lines[0].Candidates) != 1 {
		t.Fatalf("expected one exact candidate, got %d", len(detail.Lines[0].Candidates))
	}
	matched, err := service.Match(ctx, banking.MatchCommand{Scope: scope, StatementID: statement.ID, LineID: statement.Lines[0].ID, JournalLineID: detail.Lines[0].Candidates[0].JournalLineID, Reason: "Verified against CRDB reference", ActorID: maker, IdempotencyKey: "bank-match-request-00001"})
	if err != nil {
		t.Fatal(err)
	}
	if matched.Lines[0].Match == nil {
		t.Fatal("expected immutable match evidence")
	}
	_, err = service.Reconcile(ctx, banking.ReconcileCommand{Scope: scope, StatementID: statement.ID, Reason: "Attempted self approval", ActorID: maker, IdempotencyKey: "bank-reconcile-self-001"})
	if !errors.Is(err, banking.ErrSeparationOfDuties) {
		t.Fatalf("expected separation of duties, got %v", err)
	}
	reconciled, err := service.Reconcile(ctx, banking.ReconcileCommand{Scope: scope, StatementID: statement.ID, Reason: "Independently reviewed all evidence", ActorID: approver, IdempotencyKey: "bank-reconcile-ok-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Status != banking.Reconciled || reconciled.ReconciledBy != approver {
		t.Fatalf("unexpected reconciliation result: %#v", reconciled)
	}
}

func TestImportRejectsUnbalancedStatement(t *testing.T) {
	t.Parallel()
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	actor, accountID := "00000000-0000-4000-8000-000000000005", "00000000-0000-4000-8000-000000000007"
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	store := memory.New()
	store.SeedPermission(scope, actor, "finance.bank.import")
	store.SeedBankAccount(banking.Account{ID: accountID, Scope: scope, Currency: "TZS", Active: true})
	service, _ := banking.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	_, err := service.Import(context.Background(), banking.ImportCommand{Scope: scope, AccountID: accountID, ExternalReference: "BAD-STATEMENT", Currency: "TZS", PeriodStart: at.Add(-time.Hour), PeriodEnd: at.Add(time.Hour), OpeningMinor: 100, ClosingMinor: 200, Lines: []banking.ImportLine{{TransactionAt: at, Description: "Only movement", AmountMinor: 50}}, ActorID: actor, IdempotencyKey: "bank-import-bad-00001"})
	if !errors.Is(err, banking.ErrStatementImbalance) {
		t.Fatalf("expected imbalance rejection, got %v", err)
	}
}
