package reporting_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestFinancialStatementsReconcileAndExport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	scope := tenancy.Scope{TenantID: "30000000-0000-4000-8000-000000000001", CompanyID: "30000000-0000-4000-8000-000000000002", BranchID: "30000000-0000-4000-8000-000000000003", WarehouseID: "30000000-0000-4000-8000-000000000004"}
	actor := "30000000-0000-4000-8000-000000000005"
	store := memory.New()
	for _, permission := range []string{"reports.financial.read", "reports.financial.export"} {
		store.SeedPermission(scope, actor, permission)
	}
	for index, a := range []struct {
		id, name string
		kind     financialops.AccountType
	}{{"cash", "Cash", financialops.AccountAsset}, {"tax", "Tax payable", financialops.AccountLiability}, {"revenue", "Revenue", financialops.AccountRevenue}, {"expense", "=Expense", financialops.AccountExpense}} {
		store.SeedGLAccount(financialops.GLAccount{RecordID: "30000000-0000-4000-8000-00000000001" + string(rune('0'+index)), ID: a.id, TenantID: scope.TenantID, CompanyID: scope.CompanyID, Code: a.id, Name: a.name, Type: a.kind, Status: financialops.GovernanceActive})
	}
	store.SeedBankAccount(banking.Account{ID: "30000000-0000-4000-8000-000000000010", Scope: scope, Code: "CASH", Name: "Cash", Type: banking.CashAccount, Currency: "TZS", GLAccountID: "cash", Active: true})
	err := store.WithTransaction(ctx, func(tx sales.Transaction) error {
		if e := tx.CreateJournal(ctx, finance.Journal{ID: "30000000-0000-4000-8000-000000000011", TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "SALE", SourceID: "30000000-0000-4000-8000-000000000012", Currency: "TZS", OccurredAt: at, Entries: []finance.JournalEntry{{AccountID: "cash", DebitMinor: 118}, {AccountID: "revenue", CreditMinor: 100}, {AccountID: "tax", CreditMinor: 18}}}); e != nil {
			return e
		}
		return tx.CreateJournal(ctx, finance.Journal{ID: "30000000-0000-4000-8000-000000000013", TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "BANK_ADJUSTMENT", SourceID: "30000000-0000-4000-8000-000000000014", Currency: "TZS", OccurredAt: at.Add(time.Hour), Entries: []finance.JournalEntry{{AccountID: "expense", DebitMinor: 30}, {AccountID: "cash", CreditMinor: 30}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := reporting.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	from, to := at.Add(-time.Hour), at.Add(24*time.Hour)
	query := reporting.Query{Scope: scope, ActorID: actor, From: from, To: to, AsOf: to}
	tb, err := service.TrialBalance(ctx, query)
	if err != nil || !tb.Balanced || tb.TotalDebitMinor != 118 || tb.TotalCreditMinor != 118 {
		t.Fatalf("trial balance %#v %v", tb, err)
	}
	pl, err := service.ProfitAndLoss(ctx, query)
	if err != nil || pl.NetProfitMinor != 70 {
		t.Fatalf("profit and loss %#v %v", pl, err)
	}
	bs, err := service.BalanceSheet(ctx, query)
	if err != nil || !bs.Balanced || bs.TotalAssetsMinor != 88 || bs.TotalLiabilitiesMinor != 18 || bs.TotalEquityMinor != 70 {
		t.Fatalf("balance sheet %#v %v", bs, err)
	}
	ledger, err := service.GeneralLedger(ctx, reporting.Query{Scope: scope, ActorID: actor, From: from, To: to, AccountID: "cash"})
	if err != nil || ledger.ClosingBalanceMinor != 88 || len(ledger.Entries) != 2 {
		t.Fatalf("ledger %#v %v", ledger, err)
	}
	flow, err := service.CashFlow(ctx, query)
	if err != nil || !flow.Reconciled || flow.Operating.NetMinor != 88 || flow.ClosingCashMinor != 88 {
		t.Fatalf("cash flow %#v %v", flow, err)
	}
	export, err := service.Export(ctx, reporting.ExportCommand{Query: query, Type: reporting.ReportProfitAndLoss, IdempotencyKey: "report-export-000001"})
	if err != nil {
		t.Fatal(err)
	}
	content, err := base64.StdEncoding.DecodeString(export.ContentBase64)
	if err != nil || len(content) == 0 || !strings.Contains(string(content), "'=Expense") {
		t.Fatalf("invalid export %v", err)
	}
	repeat, err := service.Export(ctx, reporting.ExportCommand{Query: query, Type: reporting.ReportProfitAndLoss, IdempotencyKey: "report-export-000001"})
	if err != nil || repeat.ID != export.ID {
		t.Fatalf("export replay %#v %v", repeat, err)
	}
	for _, tc := range []struct {
		format     reporting.ExportFormat
		key, magic string
	}{{reporting.ExportPDF, "report-export-pdf-0001", "%PDF"}, {reporting.ExportXLSX, "report-export-xlsx-001", "PK"}} {
		artifact, exportErr := service.Export(ctx, reporting.ExportCommand{Query: query, Type: reporting.ReportProfitAndLoss, Format: tc.format, ComparisonFrom: from.AddDate(-1, 0, 0), ComparisonTo: to.AddDate(-1, 0, 0), IdempotencyKey: tc.key})
		decoded, decodeErr := base64.StdEncoding.DecodeString(artifact.ContentBase64)
		if exportErr != nil || decodeErr != nil || !strings.HasPrefix(string(decoded), tc.magic) || len(artifact.SHA256) != 64 || artifact.Format != tc.format {
			t.Fatalf("invalid %s report pack %#v %v %v", tc.format, artifact, exportErr, decodeErr)
		}
	}
}
