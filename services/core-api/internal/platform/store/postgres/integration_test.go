package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/dbrole"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

func TestPostgresGoldenSaleIdempotencyAndReversal(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "itembaz_test_" + time.Now().UTC().Format("20060102150405")
	for _, name := range []string{"000001_core.up.sql", "000002_live_golden.up.sql", "000003_runtime_security.up.sql", "000004_runtime_capabilities.up.sql", "000005_offline_and_version_ack.up.sql", "000006_version_ack_serialization.up.sql", "000007_offline_sales_leases.up.sql", "000008_catalog_snapshot_tokens.up.sql", "000009_catalog_publications.up.sql", "000010_mobile_reconciliation.up.sql", "000011_offline_posting_policy.up.sql", "000012_mobile_device_governance.up.sql", "000013_customer_receivables.up.sql", "000014_commercial_operations.up.sql", "000015_bank_reconciliation.up.sql", "000016_financial_controls.up.sql", "000017_chart_of_accounts.up.sql", "000018_financial_reporting.up.sql", "000019_advanced_finance.up.sql", "000020_treasury.up.sql", "000021_group_finance.up.sql", "000022_purchase_asset_clearing.up.sql", "000023_people_payroll.up.sql", "000024_governed_settings.up.sql"} {
		applyTestMigration(t, ctx, pool, schema, name)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	}()
	store, err := postgres.New(pool, schema)
	if err != nil {
		t.Fatal(err)
	}

	const (
		tenantID          = "00000000-0000-4000-8000-000000000001"
		companyID         = "00000000-0000-4000-8000-000000000002"
		branchID          = "00000000-0000-4000-8000-0000000000a3"
		warehouseID       = "00000000-0000-4000-8000-0000000000b4"
		userID            = "00000000-0000-4000-8000-0000000000d5"
		roleID            = "00000000-0000-4000-8000-000000000006"
		scopeID           = "00000000-0000-4000-8000-000000000007"
		customerID        = "00000000-0000-4000-8000-0000000000e8"
		creditCustomerID  = "00000000-0000-4000-8000-0000000000e9"
		productID         = "00000000-0000-4000-8000-0000000000c9"
		taxID             = "00000000-0000-4000-8000-00000000000a"
		periodID          = "00000000-0000-4000-8000-00000000000b"
		stockID           = "00000000-0000-4000-8000-00000000000c"
		openingID         = "00000000-0000-4000-8000-00000000000d"
		otherBranchID     = "00000000-0000-4000-8000-00000000000e"
		otherWarehouseID  = "00000000-0000-4000-8000-00000000000f"
		otherScopeID      = "00000000-0000-4000-8000-000000000010"
		approverID        = "00000000-0000-4000-8000-000000000011"
		approverScopeID   = "00000000-0000-4000-8000-000000000012"
		supplierID        = "00000000-0000-4000-8000-000000000013"
		bankAccountID     = "00000000-0000-4000-8000-000000000014"
		secondCompanyID   = "00000000-0000-4000-8000-000000000015"
		secondBranchID    = "00000000-0000-4000-8000-000000000016"
		secondWarehouseID = "00000000-0000-4000-8000-000000000017"
		counterApproverID = "00000000-0000-4000-8000-000000000018"
		counterScopeID    = "00000000-0000-4000-8000-000000000019"
		secondPeriodID    = "00000000-0000-4000-8000-00000000001a"
	)
	var testTime time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&testTime); err != nil {
		t.Fatal(err)
	}
	testTime = testTime.UTC().Truncate(time.Second)
	seed, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = seed.Rollback(context.Background()) }()
	if _, err := seed.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()+", public"); err != nil {
		t.Fatal(err)
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenants(id,name) VALUES($1,'Tenant')`, []any{tenantID}},
		{`INSERT INTO legal_companies(id,tenant_id,name) VALUES($1,$2,'Company')`, []any{companyID, tenantID}},
		{`INSERT INTO legal_companies(id,tenant_id,name) VALUES($1,$2,'Second Company')`, []any{secondCompanyID, tenantID}},
		{`INSERT INTO branches(id,tenant_id,company_id,name) VALUES($1,$2,$3,'Branch')`, []any{branchID, tenantID, companyID}},
		{`INSERT INTO warehouses(id,tenant_id,company_id,branch_id,name) VALUES($1,$2,$3,$4,'Warehouse')`, []any{warehouseID, tenantID, companyID, branchID}},
		{`INSERT INTO branches(id,tenant_id,company_id,name) VALUES($1,$2,$3,'Second Branch')`, []any{secondBranchID, tenantID, secondCompanyID}},
		{`INSERT INTO warehouses(id,tenant_id,company_id,branch_id,name) VALUES($1,$2,$3,$4,'Second Warehouse')`, []any{secondWarehouseID, tenantID, secondCompanyID, secondBranchID}},
		{`INSERT INTO branches(id,tenant_id,company_id,name) VALUES($1,$2,$3,'Other Branch')`, []any{otherBranchID, tenantID, companyID}},
		{`INSERT INTO warehouses(id,tenant_id,company_id,branch_id,name) VALUES($1,$2,$3,$4,'Other Warehouse')`, []any{otherWarehouseID, tenantID, companyID, otherBranchID}},
		{`INSERT INTO users(id,tenant_id,email) VALUES($1,$2,'user@example.test')`, []any{userID, tenantID}},
		{`INSERT INTO users(id,tenant_id,email) VALUES($1,$2,'approver@example.test')`, []any{approverID, tenantID}},
		{`INSERT INTO users(id,tenant_id,email) VALUES($1,$2,'counter@example.test')`, []any{counterApproverID, tenantID}},
		{`INSERT INTO roles(id,tenant_id,name) VALUES($1,$2,'Manager')`, []any{roleID, tenantID}},
		{`INSERT INTO role_permissions(tenant_id,role_id,permission_code) SELECT $1,$2,code FROM permissions`, []any{tenantID, roleID}},
		{`INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, []any{scopeID, tenantID, userID, roleID, companyID, branchID, warehouseID}},
		{`INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, []any{otherScopeID, tenantID, userID, roleID, companyID, otherBranchID, otherWarehouseID}},
		{`INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, []any{approverScopeID, tenantID, approverID, roleID, companyID, branchID, warehouseID}},
		{`INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, []any{counterScopeID, tenantID, counterApproverID, roleID, secondCompanyID, secondBranchID, secondWarehouseID}},
		{`INSERT INTO customer_accounts(id,tenant_id,company_id,code,name,active,is_general) VALUES($1,$2,$3,'GENERAL','General Customer',true,true)`, []any{customerID, tenantID, companyID}},
		{`INSERT INTO customer_accounts(id,tenant_id,company_id,code,name,active,is_general,credit_enabled,credit_limit_minor) VALUES($1,$2,$3,'CREDIT','Credit Customer',true,false,true,1000000)`, []any{creditCustomerID, tenantID, companyID}},
		{`INSERT INTO products(id,tenant_id,company_id,sku,name,currency,list_price_minor,standard_cost_minor,tax_code,revenue_account_id,cogs_account_id,inventory_account_id) VALUES($1,$2,$3,'SKU','Product','TZS',10000,6000,'VAT','revenue','cogs','inventory')`, []any{productID, tenantID, companyID}},
		{`INSERT INTO suppliers(id,tenant_id,company_id,code,name,active,payment_terms_days) VALUES($1,$2,$3,'SUP','Supplier',true,30)`, []any{supplierID, tenantID, companyID}},
		{`INSERT INTO tax_rules(id,tenant_id,company_id,code,basis_points,effective_from) VALUES($1,$2,$3,'VAT',1800,$4)`, []any{taxID, tenantID, companyID, testTime.AddDate(-1, 0, 0)}},
		{`INSERT INTO fiscal_periods(id,tenant_id,company_id,starts_at,ends_at,is_open) VALUES($1,$2,$3,$4,$5,true)`, []any{periodID, tenantID, companyID, testTime.AddDate(0, -1, 0), testTime.AddDate(0, 1, 0)}},
		{`INSERT INTO fiscal_periods(id,tenant_id,company_id,starts_at,ends_at,is_open) VALUES($1,$2,$3,$4,$5,true)`, []any{secondPeriodID, tenantID, secondCompanyID, testTime.AddDate(0, -1, 0), testTime.AddDate(0, 1, 0)}},
		{`INSERT INTO offline_posting_policies(id,tenant_id,company_id,accounting_time_basis,maximum_future_skew_seconds,require_same_fiscal_period,effective_from,created_by,created_at) VALUES(gen_random_uuid(),$1,$2,'SERVER_RECEIPT',14400,true,$3,$4,$5)`, []any{tenantID, companyID, testTime.AddDate(-1, 0, 0), userID, testTime}},
		{`INSERT INTO sales_posting_config(tenant_id,company_id,receivable_account_id,tax_payable_account_id,cash_accounts) VALUES($1,$2,'receivable','tax-payable','{"CASH":"cash"}')`, []any{tenantID, companyID}},
		{`INSERT INTO procurement_posting_config(tenant_id,company_id,grni_account_id,payable_account_id,inventory_adjustment_account_id,stock_in_transit_account_id,cash_accounts) VALUES($1,$2,'grni','payable','inventory-adjustment','stock-in-transit','{"CASH":"cash"}')`, []any{tenantID, companyID}},
		{`INSERT INTO bank_accounts(id,tenant_id,company_id,branch_id,warehouse_id,code,name,account_type,currency,gl_account_id,active) VALUES($1,$2,$3,$4,$5,'CASH','Till cash','CASH','TZS','cash',true)`, []any{bankAccountID, tenantID, companyID, branchID, warehouseID}},
		{`INSERT INTO gl_accounts(record_id,tenant_id,company_id,id,code,name,account_type,allow_manual_posting,status,created_at) SELECT gen_random_uuid(),$1,$2,id,id,name,kind,manual,'ACTIVE',$3 FROM (VALUES ('expense','Integration expense','EXPENSE',true),('suspense','Integration suspense','ASSET',true),('cash','Cash','ASSET',false),('revenue','Revenue','REVENUE',false),('tax-payable','Tax payable','LIABILITY',false),('cogs','Cost of goods sold','EXPENSE',false),('inventory','Inventory','ASSET',false),('receivable','Receivable','ASSET',false),('grni','GRNI','LIABILITY',false),('payable','Payable','LIABILITY',false),('inventory-adjustment','Inventory adjustment','EXPENSE',false),('stock-in-transit','Stock in transit','ASSET',false)) a(id,name,kind,manual)`, []any{tenantID, companyID, testTime}},
		{`INSERT INTO gl_accounts(record_id,tenant_id,company_id,id,code,name,account_type,allow_manual_posting,status,created_at) SELECT gen_random_uuid(),$1,$2,id,id,name,kind,false,'ACTIVE',$3 FROM (VALUES ('ic-receivable','Intercompany receivable','ASSET'),('recovery','Cost recovery','REVENUE')) a(id,name,kind)`, []any{tenantID, companyID, testTime}},
		{`INSERT INTO gl_accounts(record_id,tenant_id,company_id,id,code,name,account_type,allow_manual_posting,status,created_at) SELECT gen_random_uuid(),$1,$2,id,id,name,kind,false,'ACTIVE',$3 FROM (VALUES ('allocated-expense','Allocated expense','EXPENSE'),('ic-payable','Intercompany payable','LIABILITY')) a(id,name,kind)`, []any{tenantID, secondCompanyID, testTime}},
		{`INSERT INTO inventory_stock_ledger(id,tenant_id,company_id,branch_id,warehouse_id,product_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,'OPENING',$7,100,$8)`, []any{stockID, tenantID, companyID, branchID, warehouseID, productID, openingID, testTime.Add(-time.Hour)}},
	}
	for _, statement := range statements {
		if _, err := seed.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := seed.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	service, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime})
	if err != nil {
		t.Fatal(err)
	}
	scope := tenancy.Scope{TenantID: tenantID, CompanyID: companyID, BranchID: branchID, WarehouseID: warehouseID}
	command := sales.CompleteCommand{Scope: scope, CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: "CASH", Lines: []sales.CommandLine{{ProductID: productID, Quantity: 2}}, ActorID: userID, IdempotencyKey: "integration-idempotency-key"}
	created, err := service.Complete(ctx, command)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if created.CorrelationID == "" {
		t.Fatal("sale correlation id was not persisted")
	}
	repeated, err := service.Complete(ctx, command)
	if err != nil || repeated.ID != created.ID {
		t.Fatalf("idempotency: %+v %v", repeated, err)
	}
	otherScope := tenancy.Scope{TenantID: tenantID, CompanyID: companyID, BranchID: otherBranchID, WarehouseID: otherWarehouseID}
	if _, err := service.Get(ctx, otherScope, userID, created.ID); !errors.Is(err, sales.ErrNotFound) {
		t.Fatalf("cross-branch PostgreSQL read error=%v", err)
	}
	if _, err := service.Reverse(ctx, sales.ReverseCommand{
		Scope: otherScope, SaleID: created.ID, Reason: "Cross-branch attempt",
		ActorID: userID, IdempotencyKey: "integration-cross-branch-reversal",
	}); !errors.Is(err, sales.ErrNotFound) {
		t.Fatalf("cross-branch PostgreSQL reversal error=%v", err)
	}
	bankingService, err := banking.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(30 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	statement, err := bankingService.Import(ctx, banking.ImportCommand{Scope: scope, AccountID: bankAccountID, ExternalReference: "INTEGRATION-CASH-DAY", Currency: "TZS", PeriodStart: testTime.Add(-time.Hour), PeriodEnd: testTime.Add(time.Hour), OpeningMinor: 0, ClosingMinor: created.TotalMinor, Lines: []banking.ImportLine{{TransactionAt: testTime, ExternalReference: created.ID, Description: "Golden sale cash receipt", AmountMinor: created.TotalMinor}}, ActorID: userID, IdempotencyKey: "integration-bank-import-001"})
	if err != nil {
		t.Fatalf("import bank statement: %v", err)
	}
	detail, err := bankingService.Statement(ctx, scope, userID, statement.ID)
	if err != nil || len(detail.Lines) != 1 || len(detail.Lines[0].Candidates) != 1 {
		t.Fatalf("bank candidates: %+v %v", detail, err)
	}
	matched, err := bankingService.Match(ctx, banking.MatchCommand{Scope: scope, StatementID: statement.ID, LineID: detail.Lines[0].ID, JournalLineID: detail.Lines[0].Candidates[0].JournalLineID, Reason: "Verified against golden sale", ActorID: userID, IdempotencyKey: "integration-bank-match-0001"})
	if err != nil || matched.Lines[0].Match == nil {
		t.Fatalf("match bank statement: %+v %v", matched, err)
	}
	if _, err := bankingService.Reconcile(ctx, banking.ReconcileCommand{Scope: scope, StatementID: statement.ID, Reason: "Independent finance approval", ActorID: approverID, IdempotencyKey: "integration-bank-reconcile-1"}); err != nil {
		t.Fatalf("reconcile bank statement: %v", err)
	}
	financialService, err := financialops.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(45 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	financialDocument, err := financialService.Create(ctx, financialops.CreateCommand{Scope: scope, Type: financialops.ManualJournal, Currency: "TZS", AccountingAt: testTime, Reason: "Integration governed journal correction", ActorID: userID, IdempotencyKey: "integration-finance-create-1", Lines: []finance.JournalEntry{{AccountID: "expense", DebitMinor: 1500, Memo: "Correction"}, {AccountID: "suspense", CreditMinor: 1500, Memo: "Correction"}}})
	if err != nil {
		t.Fatalf("create financial document: %v", err)
	}
	financialDocument, err = financialService.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: financialDocument.ID, Status: financialops.Submitted, Reason: "Integration evidence ready for review", ActorID: userID, IdempotencyKey: "integration-finance-submit-1"})
	if err != nil {
		t.Fatalf("submit financial document: %v", err)
	}
	if _, err = financialService.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: financialDocument.ID, Status: financialops.Posted, Reason: "Maker attempted own posting approval", ActorID: userID, IdempotencyKey: "integration-finance-selfpost"}); !errors.Is(err, financialops.ErrSeparationOfDuties) {
		t.Fatalf("financial separation of duties: %v", err)
	}
	financialDocument, err = financialService.Transition(ctx, financialops.TransitionCommand{Scope: scope, DocumentID: financialDocument.ID, Status: financialops.Posted, Reason: "Independent posting evidence verified", ActorID: approverID, IdempotencyKey: "integration-finance-post-001"})
	if err != nil || financialDocument.JournalID == "" {
		t.Fatalf("post financial document: %+v %v", financialDocument, err)
	}
	var debit, credit, evidence int64
	if err = pool.QueryRow(ctx, `SELECT COALESCE(sum(l.debit_minor),0),COALESCE(sum(l.credit_minor),0),(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events WHERE entity_id=$1) FROM `+pgx.Identifier{schema}.Sanitize()+`.journal_lines l WHERE l.journal_id=$2`, financialDocument.ID, financialDocument.JournalID).Scan(&debit, &credit, &evidence); err != nil || debit != 1500 || credit != 1500 || evidence < 3 {
		t.Fatalf("financial journal/evidence debit=%d credit=%d evidence=%d err=%v", debit, credit, evidence, err)
	}
	reportingService, err := reporting.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	reportQuery := reporting.Query{Scope: scope, ActorID: approverID, From: testTime.Add(-24 * time.Hour), To: testTime.Add(24 * time.Hour), AsOf: testTime.Add(24 * time.Hour)}
	trial, err := reportingService.TrialBalance(ctx, reportQuery)
	if err != nil || !trial.Balanced || trial.TotalDebitMinor != trial.TotalCreditMinor {
		t.Fatalf("trial balance: %+v %v", trial, err)
	}
	profit, err := reportingService.ProfitAndLoss(ctx, reportQuery)
	if err != nil || profit.NetProfitMinor == 0 {
		t.Fatalf("profit and loss: %+v %v", profit, err)
	}
	balance, err := reportingService.BalanceSheet(ctx, reportQuery)
	if err != nil || !balance.Balanced {
		t.Fatalf("balance sheet: %+v %v", balance, err)
	}
	flow, err := reportingService.CashFlow(ctx, reportQuery)
	if err != nil || !flow.Reconciled || flow.ClosingCashMinor == 0 {
		t.Fatalf("cash flow: %+v %v", flow, err)
	}
	artifact, err := reportingService.Export(ctx, reporting.ExportCommand{Query: reportQuery, Type: reporting.ReportTrialBalance, IdempotencyKey: "integration-report-export-1"})
	if err != nil || artifact.ContentBase64 == "" {
		t.Fatalf("report export: %+v %v", artifact, err)
	}
	advancedService, err := advancedfinance.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(90 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	budget, err := advancedService.CreateBudget(ctx, advancedfinance.CreateBudgetCommand{Scope: scope, Name: "Integration operating budget", FiscalYear: testTime.Year(), Currency: "TZS", Reason: "Approved integration operating plan", ActorID: userID, IdempotencyKey: "integration-budget-create-1", Lines: []advancedfinance.BudgetLine{{AccountID: "expense", Month: time.Date(testTime.Year(), testTime.Month(), 1, 0, 0, 0, 0, time.UTC), AmountMinor: 5000}}})
	if err != nil {
		t.Fatalf("create budget: %v", err)
	}
	budget, err = advancedService.TransitionBudget(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: budget.ID, Status: advancedfinance.Submitted, Reason: "Budget evidence ready for review", ActorID: userID, IdempotencyKey: "integration-budget-submit-1"})
	if err != nil {
		t.Fatalf("submit budget: %v", err)
	}
	budget, err = advancedService.TransitionBudget(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: budget.ID, Status: advancedfinance.Approved, Reason: "Independent budget review completed", ActorID: approverID, IdempotencyKey: "integration-budget-approve1"})
	if err != nil {
		t.Fatalf("approve budget: %v", err)
	}
	actual, err := advancedService.BudgetActual(ctx, scope, approverID, budget.ID)
	if err != nil || len(actual.Lines) != 1 || actual.Lines[0].ActualMinor != 1500 {
		t.Fatalf("budget actual: %+v %v", actual, err)
	}
	asset, err := advancedService.CreateAsset(ctx, advancedfinance.CreateAssetCommand{Scope: scope, Code: "INT-ASSET-1", Name: "Integration fixed asset", Category: "Equipment", Currency: "TZS", AcquiredAt: testTime, CostMinor: 12000, ResidualMinor: 0, UsefulLifeMonths: 12, AssetAccountID: "inventory", AccumulatedDepreciationAccountID: "suspense", DepreciationExpenseAccountID: "expense", CapitalizationOffsetAccountID: "payable", DisposalGainAccountID: "revenue", DisposalLossAccountID: "cogs", Reason: "Approved integration asset purchase", ActorID: userID, IdempotencyKey: "integration-asset-create-1"})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	asset, err = advancedService.TransitionAsset(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: asset.ID, Status: advancedfinance.Submitted, Reason: "Asset evidence ready for review", ActorID: userID, IdempotencyKey: "integration-asset-submit-1"})
	if err != nil {
		t.Fatalf("submit asset: %v", err)
	}
	asset, err = advancedService.TransitionAsset(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: asset.ID, Status: advancedfinance.Active, Reason: "Independent capitalization approved", ActorID: approverID, IdempotencyKey: "integration-asset-active-1"})
	if err != nil {
		t.Fatalf("activate asset: %v", err)
	}
	depreciation, err := advancedService.Depreciate(ctx, advancedfinance.DepreciateCommand{Scope: scope, AssetID: asset.ID, Period: time.Date(testTime.Year(), testTime.Month(), 1, 0, 0, 0, 0, time.UTC), Reason: "Approved monthly asset depreciation", ActorID: approverID, IdempotencyKey: "integration-asset-depr-001"})
	if err != nil || depreciation.AmountMinor != 1000 {
		t.Fatalf("depreciate asset: %+v %v", depreciation, err)
	}
	asset, err = advancedService.Dispose(ctx, advancedfinance.DisposeCommand{Scope: scope, AssetID: asset.ID, ProceedsMinor: 10000, ProceedsAccountID: "cash", Reason: "Approved integration asset disposal", ActorID: approverID, IdempotencyKey: "integration-asset-dispose1"})
	if err != nil || asset.Status != advancedfinance.Disposed {
		t.Fatalf("dispose asset: %+v %v", asset, err)
	}
	treasuryService, err := treasury.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(95 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	facility, err := treasuryService.CreateFacility(ctx, treasury.CreateFacilityCommand{Scope: scope, Reference: "INT-LOAN-1", Lender: "Integration Bank", Type: treasury.TermLoan, Currency: "TZS", LimitMinor: 10000, AnnualInterestBasisPoints: 1200, StartDate: testTime.AddDate(0, -1, 0), MaturityDate: testTime.AddDate(1, 0, 0), BankAccountID: "cash", PrincipalAccountID: "payable", InterestExpenseAccountID: "expense", AccruedInterestAccountID: "tax-payable", Reason: "Approved integration working capital facility", ActorID: userID, IdempotencyKey: "integration-facility-create"})
	if err != nil {
		t.Fatalf("create facility: %v", err)
	}
	facility, err = treasuryService.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: facility.ID, Status: treasury.Submitted, Reason: "Facility evidence ready for review", ActorID: userID, IdempotencyKey: "integration-facility-submit"})
	if err != nil {
		t.Fatalf("submit facility: %v", err)
	}
	facility, err = treasuryService.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: facility.ID, Status: treasury.Active, Reason: "Independent facility approval complete", ActorID: approverID, IdempotencyKey: "integration-facility-active"})
	if err != nil {
		t.Fatalf("approve facility: %v", err)
	}
	for index, posting := range []struct {
		kind   treasury.TransactionType
		amount int64
	}{{treasury.Drawdown, 10000}, {treasury.InterestAccrual, 1000}, {treasury.PrincipalRepayment, 10000}, {treasury.InterestPayment, 1000}} {
		facility, err = treasuryService.PostTransaction(ctx, treasury.PostTransactionCommand{Scope: scope, FacilityID: facility.ID, Type: posting.kind, AmountMinor: posting.amount, OccurredAt: testTime, Reason: "Approved integration treasury posting", ActorID: approverID, IdempotencyKey: []string{"integration-drawdown-0001", "integration-interest-accrual", "integration-principal-repay", "integration-interest-payment"}[index]})
		if err != nil {
			t.Fatalf("post treasury %s: %v", posting.kind, err)
		}
	}
	if facility.OutstandingPrincipalMinor != 0 || facility.AccruedInterestMinor != 0 || len(facility.Transactions) != 4 {
		t.Fatalf("treasury balances: %+v", facility)
	}
	facility, err = treasuryService.TransitionFacility(ctx, treasury.TransitionCommand{Scope: scope, FacilityID: facility.ID, Status: treasury.Closed, Reason: "Facility fully settled and reconciled", ActorID: approverID, IdempotencyKey: "integration-facility-close1"})
	if err != nil || facility.Status != treasury.Closed {
		t.Fatalf("close facility: %+v %v", facility, err)
	}
	groupService, err := groupfinance.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(100 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	intercompany, err := groupService.Create(ctx, groupfinance.CreateCommand{Scope: scope, CounterpartyCompanyID: secondCompanyID, Reference: "INT-ALLOC-1", Type: groupfinance.CostAllocation, Currency: "TZS", AmountMinor: 5000, OccurredAt: testTime, SourceDebitAccountID: "ic-receivable", SourceCreditAccountID: "recovery", CounterpartyDebitAccountID: "allocated-expense", CounterpartyCreditAccountID: "ic-payable", Reason: "Approved integration shared service allocation", ActorID: userID, IdempotencyKey: "integration-group-create1"})
	if err != nil {
		t.Fatalf("create intercompany: %v", err)
	}
	intercompany, err = groupService.Transition(ctx, groupfinance.TransitionCommand{Scope: scope, TransactionID: intercompany.ID, Status: groupfinance.Submitted, Reason: "Intercompany evidence ready for review", ActorID: userID, IdempotencyKey: "integration-group-submit1"})
	if err != nil {
		t.Fatalf("submit intercompany: %v", err)
	}
	intercompany, err = groupService.Transition(ctx, groupfinance.TransitionCommand{Scope: scope, TransactionID: intercompany.ID, Status: groupfinance.SourceApproved, Reason: "Independent source company approval complete", ActorID: approverID, IdempotencyKey: "integration-group-source1"})
	if err != nil {
		t.Fatalf("source approve intercompany: %v", err)
	}
	counterScope := tenancy.Scope{TenantID: tenantID, CompanyID: secondCompanyID, BranchID: secondBranchID, WarehouseID: secondWarehouseID}
	intercompany, err = groupService.Transition(ctx, groupfinance.TransitionCommand{Scope: counterScope, TransactionID: intercompany.ID, Status: groupfinance.Posted, Reason: "Counterparty confirmed allocation posting", ActorID: counterApproverID, IdempotencyKey: "integration-group-counter"})
	if err != nil || intercompany.Status != groupfinance.Posted {
		t.Fatalf("post intercompany: %+v %v", intercompany, err)
	}
	consolidated, err := groupService.Consolidated(ctx, scope, approverID, testTime.Add(time.Hour))
	if err != nil || consolidated.IntercompanyBalanceEliminationMinor != 5000 || consolidated.IntercompanyActivityEliminationMinor != 5000 || !consolidated.Balanced {
		t.Fatalf("consolidation: %+v %v", consolidated, err)
	}
	peopleService, err := people.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(105 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	employee, err := peopleService.CreateEmployee(ctx, people.CreateEmployeeCommand{Scope: scope, Number: "EMP-INT-1", FullName: "Asha Integration", JobTitle: "Accountant", Department: "Finance", Currency: "TZS", HireDate: testTime.AddDate(-1, 0, 0), BaseSalaryMinor: 100000, ActorID: userID, IdempotencyKey: "integration-employee-create"})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	leaveType, err := peopleService.CreateLeaveType(ctx, people.LeaveTypeCommand{Scope: scope, Code: "ANNUAL", NameEN: "Annual leave", NameSW: "Likizo ya mwaka", AnnualEntitlementDays: 28, EffectiveFrom: testTime, ActorID: userID, IdempotencyKey: "integration-leave-type-01"})
	if err != nil {
		t.Fatalf("create leave type: %v", err)
	}
	leave, err := peopleService.CreateLeave(ctx, people.LeaveCommand{Scope: scope, EmployeeID: employee.ID, LeaveTypeID: leaveType.ID, StartsOn: testTime.AddDate(0, 0, 2), EndsOn: testTime.AddDate(0, 0, 4), Reason: "Integration annual leave evidence", ActorID: userID, IdempotencyKey: "integration-leave-create1"})
	if err != nil {
		t.Fatalf("create leave: %v", err)
	}
	leave, err = peopleService.TransitionLeave(ctx, people.TransitionCommand{Scope: scope, ID: leave.ID, Status: people.Submitted, Reason: "Leave evidence submitted for review", ActorID: userID, IdempotencyKey: "integration-leave-submit1"})
	if err != nil {
		t.Fatalf("submit leave: %v", err)
	}
	leave, err = peopleService.TransitionLeave(ctx, people.TransitionCommand{Scope: scope, ID: leave.ID, Status: people.Approved, Reason: "Independent leave approval complete", ActorID: approverID, IdempotencyKey: "integration-leave-approve"})
	if err != nil || leave.Status != people.Approved {
		t.Fatalf("approve leave: %+v %v", leave, err)
	}
	loan, err := peopleService.CreateLoan(ctx, people.LoanCommand{Scope: scope, EmployeeID: employee.ID, Reference: "INT-STAFF-LOAN", Currency: "TZS", PrincipalMinor: 20000, ReceivableAccountID: "ic-receivable", BankAccountID: "cash", Reason: "Integration employee loan evidence", ActorID: userID, IdempotencyKey: "integration-loan-create01"})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}
	for i, status := range []people.WorkflowStatus{people.Submitted, people.Approved, people.Posted} {
		actor := userID
		if status != people.Submitted {
			actor = approverID
		}
		loan, err = peopleService.TransitionLoan(ctx, people.TransitionCommand{Scope: scope, ID: loan.ID, Status: status, Reason: "Employee loan controlled lifecycle evidence", ActorID: actor, IdempotencyKey: []string{"integration-loan-submit01", "integration-loan-approve1", "integration-loan-posting1"}[i]})
		if err != nil {
			t.Fatalf("loan %s: %v", status, err)
		}
	}
	payroll, err := peopleService.CreatePayroll(ctx, people.PayrollCommand{Scope: scope, Reference: "INT-PAYROLL-AUG", Currency: "TZS", PeriodStart: testTime, PeriodEnd: testTime.AddDate(0, 0, 20), PaymentDate: testTime, SalaryExpenseAccountID: "expense", PayrollPayableAccountID: "payable", DeductionLiabilityAccountID: "tax-payable", Reason: "Integration payroll posting evidence", Lines: []people.PayrollLine{{EmployeeID: employee.ID, GrossMinor: 100000, OtherDeductionsMinor: 10000, LoanDeductionMinor: 5000}}, ActorID: userID, IdempotencyKey: "integration-payroll-create"})
	if err != nil {
		t.Fatalf("create payroll: %v", err)
	}
	for i, status := range []people.WorkflowStatus{people.Submitted, people.Approved, people.Posted} {
		actor := userID
		if status != people.Submitted {
			actor = approverID
		}
		payroll, err = peopleService.TransitionPayroll(ctx, people.TransitionCommand{Scope: scope, ID: payroll.ID, Status: status, Reason: "Payroll independently approved and posted", ActorID: actor, IdempotencyKey: []string{"integration-payroll-submit", "integration-payroll-approve", "integration-payroll-posted"}[i]})
		if err != nil {
			t.Fatalf("payroll %s: %v", status, err)
		}
	}
	peopleView, err := peopleService.Get(ctx, scope, approverID)
	if err != nil || payroll.NetMinor != 85000 || len(peopleView.PayrollRuns) != 1 || peopleView.Loans[0].OutstandingMinor != 15000 {
		t.Fatalf("people snapshot: %+v %v", peopleView, err)
	}
	configurationService, err := configuration.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(110 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	config, err := configurationService.Create(ctx, configuration.CreateCommand{Scope: scope, Category: configuration.HR, Key: "PAYROLL-POLICY", NameEN: "Payroll policy", NameSW: "Sera ya mishahara", Value: json.RawMessage(`{"parallel_runs":2}`), EffectiveFrom: testTime, Reason: "Integration payroll configuration evidence", ActorID: userID, IdempotencyKey: "integration-config-create1"})
	if err != nil {
		t.Fatalf("create configuration: %v", err)
	}
	config, err = configurationService.Transition(ctx, configuration.TransitionCommand{Scope: scope, ID: config.ID, Status: configuration.Submitted, Reason: "Configuration submitted for review", ActorID: userID, IdempotencyKey: "integration-config-submit1"})
	if err != nil {
		t.Fatalf("submit configuration: %v", err)
	}
	config, err = configurationService.Transition(ctx, configuration.TransitionCommand{Scope: scope, ID: config.ID, Status: configuration.Active, Reason: "Independent configuration activation", ActorID: approverID, IdempotencyKey: "integration-config-active1"})
	if err != nil || config.Status != configuration.Active {
		t.Fatalf("activate configuration: %+v %v", config, err)
	}
	sequence, err := configurationService.CreateSequence(ctx, configuration.SequenceCommand{Scope: scope, Key: "SALES-INVOICE", Prefix: "INV-", NextValue: 1, Padding: 6, ActorID: userID, IdempotencyKey: "integration-sequence-create"})
	if err != nil {
		t.Fatalf("create sequence: %v", err)
	}
	first, err := configurationService.Allocate(ctx, configuration.AllocateCommand{Scope: scope, SequenceID: sequence.ID, ActorID: userID, IdempotencyKey: "integration-number-first1"})
	if err != nil || first.Number != "INV-000001" {
		t.Fatalf("first number: %+v %v", first, err)
	}
	second, err := configurationService.Allocate(ctx, configuration.AllocateCommand{Scope: scope, SequenceID: sequence.ID, ActorID: userID, IdempotencyKey: "integration-number-second"})
	if err != nil || second.Number != "INV-000002" {
		t.Fatalf("second number: %+v %v", second, err)
	}
	configView, err := configurationService.Get(ctx, scope, approverID)
	if err != nil || len(configView.Versions) != 1 || len(configView.Sequences) != 1 || configView.Sequences[0].NextValue != 3 {
		t.Fatalf("configuration snapshot: %+v %v", configView, err)
	}
	closeRequest, err := financialService.RequestPeriodAction(ctx, financialops.PeriodCommand{Scope: scope, PeriodID: periodID, Action: financialops.ClosePeriod, Reason: "Integration reconciliations ready for close", ActorID: userID, IdempotencyKey: "integration-period-close-01"})
	if err != nil {
		t.Fatalf("request period close: %v", err)
	}
	actions, err := financialService.PeriodActions(ctx, scope, approverID)
	if err != nil || len(actions.Items) != 1 || actions.Items[0].ID != closeRequest.ID {
		t.Fatalf("list period actions: %+v %v", actions, err)
	}
	if _, err = financialService.ApprovePeriodAction(ctx, scope, approverID, closeRequest.ID, "Independent period close review completed", "integration-period-approve1"); err != nil {
		t.Fatalf("approve period close: %v", err)
	}
	reopenRequest, err := financialService.RequestPeriodAction(ctx, financialops.PeriodCommand{Scope: scope, PeriodID: periodID, Action: financialops.ReopenPeriod, Reason: "Integration continuation requires controlled reopen", ActorID: approverID, IdempotencyKey: "integration-period-reopen-1"})
	if err != nil {
		t.Fatalf("request period reopen: %v", err)
	}
	if _, err = financialService.ApprovePeriodAction(ctx, scope, userID, reopenRequest.ID, "Independent period reopen review completed", "integration-period-approve2"); err != nil {
		t.Fatalf("approve period reopen: %v", err)
	}
	reversal, err := service.Reverse(ctx, sales.ReverseCommand{Scope: scope, SaleID: created.ID, Reason: "Integration return", ActorID: userID, IdempotencyKey: "integration-reversal-key"})
	if err != nil || reversal.ReversalOf != created.ID {
		t.Fatalf("reverse: %+v %v", reversal, err)
	}
	readService, err := readmodel.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	workingContext, err := readService.Context(ctx, scope, userID)
	if err != nil || workingContext.ActorID != userID || workingContext.TimeZone != "Africa/Dar_es_Salaam" || workingContext.WarehouseID != warehouseID {
		t.Fatalf("working context: %+v %v", workingContext, err)
	}
	masterVersion := workingContext.MasterDataVersion
	priceVersion := workingContext.PriceVersion
	snapshotToken := workingContext.CatalogSnapshotToken
	customersPage, err := readService.Customers(ctx, scope, userID, "General", "", "", nil, 10)
	if err != nil || len(customersPage.Items) != 1 || customersPage.Items[0].ID != customerID {
		t.Fatalf("customers: %+v %v", customersPage, err)
	}
	productsPage, err := readService.Products(ctx, scope, userID, "SKU", "", "", 10)
	if err != nil || len(productsPage.Items) != 1 || productsPage.Items[0].AvailableQuantity != 100 || productsPage.Items[0].TaxBasisPoints != 1800 {
		t.Fatalf("products: %+v %v", productsPage, err)
	}
	creditPostingService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	creditSale, err := creditPostingService.Complete(ctx, sales.CompleteCommand{Scope: scope, CustomerID: creditCustomerID, Kind: sales.KindCredit,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, ActorID: userID, IdempotencyKey: "integration-credit-sale"})
	if err != nil {
		t.Fatalf("credit sale: %v", err)
	}
	receivablesService, err := receivables.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	account, err := receivablesService.Account(ctx, scope, userID, creditCustomerID)
	if err != nil || !account.Aging.Reconciled || account.Aging.CalculatedExposure != creditSale.TotalMinor || len(account.OpenItems) != 1 {
		t.Fatalf("credit account: %+v err=%v", account, err)
	}
	if _, err := creditPostingService.Reverse(ctx, sales.ReverseCommand{Scope: scope, SaleID: creditSale.ID, Reason: "Integration credit return", ActorID: userID, IdempotencyKey: "integration-credit-reversal"}); err != nil {
		t.Fatalf("credit reversal: %v", err)
	}
	account, err = receivablesService.Account(ctx, scope, userID, creditCustomerID)
	if err != nil || !account.Aging.Reconciled || account.Aging.CalculatedExposure != 0 || len(account.OpenItems) != 0 {
		t.Fatalf("reversed credit account: %+v err=%v", account, err)
	}
	operationsService, err := operations.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	createDocument := func(kind operations.DocumentType, party operations.PartyType, partyID, source, destination string, quantity, price int64, key string) operations.Document {
		t.Helper()
		value, createErr := operationsService.Create(ctx, operations.CreateCommand{Scope: scope, Type: kind, PartyType: party, PartyID: partyID, SourceDocumentID: source, DestinationWarehouseID: destination, Currency: "TZS", Reason: "Integration controlled workflow", Lines: []operations.CommandLine{{ProductID: productID, Quantity: quantity, UnitPriceMinor: price}}, ActorID: userID, IdempotencyKey: key})
		if createErr != nil {
			t.Fatalf("create %s: %v", kind, createErr)
		}
		return value
	}
	advance := func(value operations.Document, status operations.Status, actor, key string, transitionScope tenancy.Scope) operations.Document {
		t.Helper()
		changed, transitionErr := operationsService.Transition(ctx, operations.TransitionCommand{Scope: transitionScope, DocumentID: value.ID, ToStatus: status, Reason: "Integration authorized transition", PaymentMethod: map[bool]string{true: "CASH"}[value.Type == operations.SupplierPayment && status == operations.Posted], ActorID: actor, IdempotencyKey: key})
		if transitionErr != nil {
			t.Fatalf("transition %s to %s: %v", value.Type, status, transitionErr)
		}
		return changed
	}
	request := createDocument(operations.PurchaseRequest, operations.NoParty, "", "", "", 2, 0, "integration-pr-create")
	request = advance(request, operations.Submitted, userID, "integration-pr-submit", scope)
	request = advance(request, operations.Approved, approverID, "integration-pr-approve", scope)
	order := createDocument(operations.PurchaseOrder, operations.SupplierParty, supplierID, request.ID, "", 2, 5000, "integration-po-create")
	order = advance(order, operations.Submitted, userID, "integration-po-submit", scope)
	order = advance(order, operations.Approved, approverID, "integration-po-approve", scope)
	receipt := createDocument(operations.GoodsReceipt, operations.SupplierParty, supplierID, order.ID, "", 2, 5000, "integration-gr-create")
	receipt = advance(receipt, operations.Submitted, userID, "integration-gr-submit", scope)
	receipt = advance(receipt, operations.Approved, approverID, "integration-gr-approve", scope)
	receipt = advance(receipt, operations.Posted, userID, "integration-gr-posted", scope)
	invoice := createDocument(operations.SupplierInvoice, operations.SupplierParty, supplierID, receipt.ID, "", 2, 5000, "integration-si-create")
	invoice = advance(invoice, operations.Submitted, userID, "integration-si-submit", scope)
	invoice = advance(invoice, operations.Approved, approverID, "integration-si-approve", scope)
	invoice = advance(invoice, operations.Posted, userID, "integration-si-posted", scope)
	purchaseReturn := createDocument(operations.PurchaseReturn, operations.SupplierParty, supplierID, invoice.ID, "", 1, 5000, "integration-return-create")
	purchaseReturn = advance(purchaseReturn, operations.Submitted, userID, "integration-return-submit", scope)
	purchaseReturn = advance(purchaseReturn, operations.Approved, approverID, "integration-return-approve", scope)
	purchaseReturn = advance(purchaseReturn, operations.Posted, userID, "integration-return-posted", scope)
	purchasedAsset, err := advancedService.CreatePurchasedAsset(ctx, advancedfinance.CreatePurchasedAssetCommand{Scope: scope, SourceDocumentID: invoice.ID, SourceProductID: productID, Code: "INT-PURCHASE-ASSET", Name: "Purchased integration asset", Category: "Equipment", Currency: "TZS", ResidualMinor: 0, UsefulLifeMonths: 24, AssetAccountID: "suspense", AccumulatedDepreciationAccountID: "inventory", DepreciationExpenseAccountID: "expense", CapitalizationOffsetAccountID: "inventory", DisposalGainAccountID: "revenue", DisposalLossAccountID: "cogs", Reason: "Capitalize matched supplier invoice balance", ActorID: userID, IdempotencyKey: "integration-purchased-asset"})
	if err != nil || purchasedAsset.CostMinor != 5000 || purchasedAsset.SourceDocumentID != invoice.ID {
		t.Fatalf("create purchased asset: %+v %v", purchasedAsset, err)
	}
	purchasedAsset, err = advancedService.TransitionAsset(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: purchasedAsset.ID, Status: advancedfinance.Submitted, Reason: "Purchase-linked asset ready for review", ActorID: userID, IdempotencyKey: "integration-passet-submit"})
	if err != nil {
		t.Fatalf("submit purchased asset: %v", err)
	}
	purchasedAsset, err = advancedService.TransitionAsset(ctx, advancedfinance.TransitionCommand{Scope: scope, ID: purchasedAsset.ID, Status: advancedfinance.Active, Reason: "Independent purchase capitalization approved", ActorID: approverID, IdempotencyKey: "integration-passet-active"})
	if err != nil || purchasedAsset.Status != advancedfinance.Active {
		t.Fatalf("activate purchased asset: %+v %v", purchasedAsset, err)
	}
	payment := createDocument(operations.SupplierPayment, operations.SupplierParty, supplierID, invoice.ID, "", 1, 5000, "integration-sp-create")
	payment = advance(payment, operations.Submitted, userID, "integration-sp-submit", scope)
	payment = advance(payment, operations.Approved, approverID, "integration-sp-approve", scope)
	payment = advance(payment, operations.Posted, userID, "integration-sp-posted", scope)
	var supplierBalance int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0)::bigint FROM `+pgx.Identifier{schema}.Sanitize()+`.supplier_ledger WHERE tenant_id=$1 AND company_id=$2 AND supplier_id=$3`, tenantID, companyID, supplierID).Scan(&supplierBalance); err != nil || supplierBalance != 0 {
		t.Fatalf("supplier balance=%d err=%v", supplierBalance, err)
	}
	transfer := createDocument(operations.StockTransfer, operations.NoParty, "", "", otherWarehouseID, 1, 0, "integration-st-create")
	transfer = advance(transfer, operations.Submitted, userID, "integration-st-submit", scope)
	transfer = advance(transfer, operations.Approved, approverID, "integration-st-approve", scope)
	transfer = advance(transfer, operations.Dispatched, userID, "integration-st-dispatch", scope)
	transfer = advance(transfer, operations.Received, userID, "integration-st-receive", otherScope)
	if transfer.Status != operations.Received {
		t.Fatalf("transfer status=%s", transfer.Status)
	}
	var destinationStock int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM `+pgx.Identifier{schema}.Sanitize()+`.inventory_stock_ledger WHERE tenant_id=$1 AND company_id=$2 AND warehouse_id=$3 AND product_id=$4`, tenantID, companyID, otherWarehouseID, productID).Scan(&destinationStock); err != nil || destinationStock != 1 {
		t.Fatalf("destination stock=%d err=%v", destinationStock, err)
	}
	count := createDocument(operations.StockCount, operations.NoParty, "", "", "", 99, 0, "integration-count-create")
	count = advance(count, operations.Submitted, userID, "integration-count-submit", scope)
	count = advance(count, operations.Approved, approverID, "integration-count-approve", scope)
	count = advance(count, operations.Posted, userID, "integration-count-posted", scope)
	adjustment := createDocument(operations.StockAdjustment, operations.NoParty, "", "", "", 1, 0, "integration-adjust-create")
	adjustment = advance(adjustment, operations.Submitted, userID, "integration-adjust-submit", scope)
	adjustment = advance(adjustment, operations.Approved, approverID, "integration-adjust-approve", scope)
	adjustment = advance(adjustment, operations.Posted, userID, "integration-adjust-posted", scope)
	oldSnapshotToken := snapshotToken
	if _, err := pool.Exec(ctx, `UPDATE `+pgx.Identifier{schema}.Sanitize()+`.products SET name='Published Product' WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, tenantID, companyID, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := readService.Products(ctx, scope, userID, "SKU", "", oldSnapshotToken, 10); !errors.Is(err, devices.ErrStaleMasterData) {
		t.Fatalf("stale catalog page token error=%v", err)
	}
	workingContext, err = readService.Context(ctx, scope, userID)
	if err != nil || workingContext.CatalogSnapshotToken == oldSnapshotToken {
		t.Fatalf("governed product update did not rotate catalog snapshot: %+v err=%v", workingContext, err)
	}
	masterVersion = workingContext.MasterDataVersion
	priceVersion = workingContext.PriceVersion
	snapshotToken = workingContext.CatalogSnapshotToken
	mobileService, err := mobile.NewService(store, service, identity.UUIDGenerator{}, clock.Fixed{Time: testTime.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	const (
		deviceID    = "10000000-0000-4000-8000-000000000001"
		clientID    = "10000000-0000-4000-8000-000000000002"
		offlineID   = "10000000-0000-4000-8000-000000000004"
		futureTaxID = "10000000-0000-4000-8000-000000000005"
	)
	enrollment, err := mobileService.Enroll(ctx, mobile.EnrollCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, DeviceName: "Integration POS", AppVersion: "1.0.0",
	})
	if err != nil || enrollment.ActorID != userID || enrollment.TimeZone != "Africa/Dar_es_Salaam" || enrollment.StockAllocations == nil || enrollment.MasterDataVersion != 0 || enrollment.AvailableMasterDataVersion != masterVersion || enrollment.AvailablePriceVersion != priceVersion || enrollment.AvailableCatalogSnapshotToken != snapshotToken {
		t.Fatalf("enrollment: %+v %v", enrollment, err)
	}
	enrollment, err = mobileService.Enroll(ctx, mobile.EnrollCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, DeviceName: "Integration POS", AppVersion: "1.0.0",
		InstalledMasterDataVersion: &masterVersion, InstalledPriceVersion: &priceVersion, InstalledCatalogSnapshotToken: &snapshotToken,
	})
	if err != nil || enrollment.MasterDataVersion != masterVersion || enrollment.PriceVersion != priceVersion || enrollment.CatalogSnapshotToken != snapshotToken {
		t.Fatalf("enrollment acknowledgement: %+v %v", enrollment, err)
	}
	managedPage, err := mobileService.ManagedDevices(ctx, scope, userID, "", 10)
	if err != nil || len(managedPage.Items) != 1 || managedPage.Items[0].ID != deviceID {
		t.Fatalf("managed device list: %+v err=%v", managedPage, err)
	}
	suspended, err := mobileService.ChangeDeviceStatus(ctx, mobile.ChangeDeviceStatusCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, Status: devices.StatusSuspended,
		Reason: "Integration custody investigation", IdempotencyKey: "postgres-device-suspend-0001",
	})
	if err != nil || suspended.Status != devices.StatusSuspended {
		t.Fatalf("managed device suspension: %+v err=%v", suspended, err)
	}
	active, err := mobileService.ChangeDeviceStatus(ctx, mobile.ChangeDeviceStatusCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, Status: devices.StatusActive,
		Reason: "Integration custody verified", IdempotencyKey: "postgres-device-reactivate-01",
	})
	if err != nil || active.Status != devices.StatusActive {
		t.Fatalf("managed device reactivation: %+v err=%v", active, err)
	}
	syncCommand := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: "CASH",
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: clientID, ClientTimestamp: testTime, AppVersion: "1.0.0",
		MasterDataVersion: masterVersion, PriceVersion: priceVersion, CatalogSnapshotToken: snapshotToken, SyncAttempt: 1,
	}
	synced, err := mobileService.SyncSale(ctx, scope, userID, "", syncCommand)
	if err != nil || synced.IdempotentReplay || synced.FiscalStatus != sales.FiscalNotConfigured || synced.ReceiptReference == "" ||
		synced.Sale.ClientTimestamp == nil || !synced.Sale.ClientTimestamp.Equal(testTime) || synced.Sale.AppVersion != "1.0.0" ||
		synced.Sale.MasterDataVersion != masterVersion || synced.Sale.PriceVersion != priceVersion || synced.Sale.CatalogSnapshotToken != snapshotToken {
		t.Fatalf("mobile sync: %+v %v", synced, err)
	}
	syncCommand.SyncAttempt = 2
	replayed, err := mobileService.SyncSale(ctx, scope, userID, "", syncCommand)
	if err != nil || !replayed.IdempotentReplay || replayed.Sale.ID != synced.Sale.ID || replayed.ReceiptReference != synced.ReceiptReference {
		t.Fatalf("mobile replay: %+v %v", replayed, err)
	}
	// Tax policy changes are serialized with lease issuance. Prepare a
	// zero-rated governed snapshot, issue a lease, then prove that an operator
	// cannot schedule a transition inside that outstanding lease.
	if _, err := pool.Exec(ctx, `UPDATE `+pgx.Identifier{schema}.Sanitize()+`.tax_rules SET basis_points=0 WHERE id=$1`, taxID); err != nil {
		t.Fatal(err)
	}
	taxContext, err := readService.Context(ctx, scope, userID)
	if err != nil {
		t.Fatal(err)
	}
	taxMasterVersion := taxContext.MasterDataVersion
	taxSnapshotToken := taxContext.CatalogSnapshotToken
	if _, err := pool.Exec(ctx, `UPDATE `+pgx.Identifier{schema}.Sanitize()+`.mobile_devices
		SET offline_enabled=true, offline_transaction_limit_minor=1000000, offline_daily_limit_minor=1000000
		WHERE tenant_id=$1 AND id=$2`, tenantID, deviceID); err != nil {
		t.Fatal(err)
	}
	allocated, err := mobileService.ChangeDeviceAllocation(ctx, mobile.ChangeDeviceAllocationCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, ProductID: productID,
		AllocatedQuantity: 10, Reason: "Integration route allocation", IdempotencyKey: "postgres-device-allocation-001",
	})
	if err != nil || len(allocated.StockAllocations) != 1 || allocated.StockAllocations[0].AllocatedQuantity != 10 {
		t.Fatalf("managed device allocation: %+v err=%v", allocated, err)
	}
	leased, err := mobileService.Enroll(ctx, mobile.EnrollCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, DeviceName: "Integration POS", AppVersion: "1.0.0",
		InstalledMasterDataVersion: &taxMasterVersion, InstalledPriceVersion: &priceVersion, InstalledCatalogSnapshotToken: &taxSnapshotToken,
	})
	leaseStart := testTime.Add(time.Minute)
	if err != nil || !leased.OfflineSalesValidUntil.Equal(leaseStart.Add(devices.OfflineSalesLeaseDuration)) {
		t.Fatalf("offline lease: %+v err=%v", leased, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE `+pgx.Identifier{schema}.Sanitize()+`.mobile_device_offline_leases SET valid_until=valid_until-interval '1 second' WHERE tenant_id=$1 AND device_id=$2`, tenantID, deviceID); err == nil || !strings.Contains(err.Error(), "is append-only") {
		t.Fatalf("offline lease history mutation error=%v", err)
	}
	attemptedTransition := testTime.Add(2 * time.Hour)
	if _, err := pool.Exec(ctx, `INSERT INTO `+pgx.Identifier{schema}.Sanitize()+`.tax_rules
		(id,tenant_id,company_id,code,basis_points,effective_from) VALUES($1,$2,$3,'VAT',1800,$4)`,
		futureTaxID, tenantID, companyID, attemptedTransition); err == nil || !strings.Contains(err.Error(), "overlaps an outstanding offline-sales lease") {
		t.Fatalf("conflicting tax transition error=%v", err)
	}
	offlineCommand := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: offlineID, ClientTimestamp: attemptedTransition.Add(time.Minute), AppVersion: "1.0.0",
		MasterDataVersion: taxMasterVersion, PriceVersion: priceVersion, CatalogSnapshotToken: taxSnapshotToken, SyncAttempt: 1, Offline: true,
	}
	if _, err := mobileService.SyncSale(ctx, scope, userID, "", offlineCommand); err != nil {
		t.Fatalf("sale after rejected transition: %v", err)
	}
	finalPriceVersion := priceVersion + 2
	var finalSnapshotToken string
	if err := pool.QueryRow(ctx, `UPDATE `+pgx.Identifier{schema}.Sanitize()+`.legal_companies
		SET price_version=$3, catalog_snapshot_token=gen_random_uuid()
		WHERE tenant_id=$1 AND id=$2 RETURNING catalog_snapshot_token`, tenantID, companyID, finalPriceVersion).Scan(&finalSnapshotToken); err != nil {
		t.Fatal(err)
	}
	refreshed, err := mobileService.Enroll(ctx, mobile.EnrollCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, DeviceName: "Integration POS", AppVersion: "1.1.0",
	})
	if err != nil || refreshed.MasterDataVersion != taxMasterVersion || refreshed.PriceVersion != priceVersion || refreshed.CatalogSnapshotToken != taxSnapshotToken || refreshed.AvailableMasterDataVersion != taxMasterVersion || refreshed.AvailablePriceVersion != finalPriceVersion || refreshed.AvailableCatalogSnapshotToken != finalSnapshotToken || refreshed.AppVersion != "1.0.0" {
		t.Fatalf("re-enrollment changed unacknowledged installed versions: %+v err=%v", refreshed, err)
	}
	var persistedAppVersion string
	if err := pool.QueryRow(ctx, `SELECT app_version FROM `+pgx.Identifier{schema}.Sanitize()+`.mobile_devices WHERE tenant_id=$1 AND id=$2`, tenantID, deviceID).Scan(&persistedAppVersion); err != nil || persistedAppVersion != "1.0.0" {
		t.Fatalf("unacknowledged app upgrade persisted=%q err=%v", persistedAppVersion, err)
	}
	var acknowledgementAudits, acknowledgementEvents, leaseCount int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events WHERE tenant_id=$1 AND entity_id=$2 AND action='mobile.device.installation_acknowledged'),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.outbox_events WHERE tenant_id=$1 AND aggregate_id=$2 AND event_type='mobile.device.installation_acknowledged'),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.mobile_device_offline_leases WHERE tenant_id=$1 AND device_id=$2)`,
		tenantID, deviceID).Scan(&acknowledgementAudits, &acknowledgementEvents, &leaseCount); err != nil || acknowledgementAudits != 2 || acknowledgementEvents != 2 || leaseCount != 1 {
		t.Fatalf("no-ack check-in changed evidence audits=%d outbox=%d leases=%d err=%v", acknowledgementAudits, acknowledgementEvents, leaseCount, err)
	}
	staleCommand := syncCommand
	staleCommand.ClientTransactionID = "10000000-0000-4000-8000-000000000003"
	staleCommand.SyncAttempt = 1
	if _, err := mobileService.SyncSale(ctx, scope, userID, "", staleCommand); !errors.Is(err, devices.ErrStaleMasterData) {
		t.Fatalf("online stale version error=%v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		acknowledged, err := mobileService.Enroll(ctx, mobile.EnrollCommand{
			Scope: scope, ActorID: userID, DeviceID: deviceID, DeviceName: "Integration POS", AppVersion: "1.1.0",
			InstalledMasterDataVersion: &taxMasterVersion, InstalledPriceVersion: &finalPriceVersion, InstalledCatalogSnapshotToken: &finalSnapshotToken,
		})
		if err != nil || acknowledged.AppVersion != "1.1.0" || acknowledged.MasterDataVersion != taxMasterVersion || acknowledged.PriceVersion != finalPriceVersion || acknowledged.CatalogSnapshotToken != finalSnapshotToken {
			t.Fatalf("app/cache acknowledgement attempt %d: %+v err=%v", attempt, acknowledged, err)
		}
		if err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events WHERE tenant_id=$1 AND entity_id=$2 AND action='mobile.device.installation_acknowledged'),
			(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.outbox_events WHERE tenant_id=$1 AND aggregate_id=$2 AND event_type='mobile.device.installation_acknowledged'),
			(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.mobile_device_offline_leases WHERE tenant_id=$1 AND device_id=$2)`,
			tenantID, deviceID).Scan(&acknowledgementAudits, &acknowledgementEvents, &leaseCount); err != nil || acknowledgementAudits != 3 || acknowledgementEvents != 3 || leaseCount != 2 {
			t.Fatalf("acknowledgement attempt %d evidence audits=%d outbox=%d leases=%d err=%v", attempt, acknowledgementAudits, acknowledgementEvents, leaseCount, err)
		}
	}
	var auditedPayload, publishedPayload []byte
	if err := pool.QueryRow(ctx, `SELECT data FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events
		WHERE tenant_id=$1 AND entity_id=$2 AND action='mobile.device.installation_acknowledged' AND data->>'app_version'='1.1.0'`, tenantID, deviceID).Scan(&auditedPayload); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT payload FROM `+pgx.Identifier{schema}.Sanitize()+`.outbox_events
		WHERE tenant_id=$1 AND aggregate_id=$2 AND event_type='mobile.device.installation_acknowledged' AND payload->>'app_version'='1.1.0'`, tenantID, deviceID).Scan(&publishedPayload); err != nil {
		t.Fatal(err)
	}
	for label, payload := range map[string][]byte{"audit": auditedPayload, "outbox": publishedPayload} {
		var finalState devices.Device
		if err := json.Unmarshal(payload, &finalState); err != nil || finalState.AppVersion != "1.1.0" ||
			finalState.MasterDataVersion != taxMasterVersion || finalState.PriceVersion != finalPriceVersion || finalState.CatalogSnapshotToken != finalSnapshotToken || !finalState.OfflineSalesValidUntil.After(leaseStart) {
			t.Fatalf("%s acknowledgement payload=%+v err=%v", label, finalState, err)
		}
	}

	expiredAt := testTime.Add(time.Minute).Add(devices.OfflineSalesLeaseDuration).Add(time.Second)
	renewalSalesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: expiredAt})
	if err != nil {
		t.Fatal(err)
	}
	renewalService, err := mobile.NewService(store, renewalSalesService, identity.UUIDGenerator{}, clock.Fixed{Time: expiredAt})
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := renewalService.Enroll(ctx, mobile.EnrollCommand{
		Scope: scope, ActorID: userID, DeviceID: deviceID, DeviceName: "Integration POS", AppVersion: "1.1.0",
		InstalledMasterDataVersion: &taxMasterVersion, InstalledPriceVersion: &finalPriceVersion, InstalledCatalogSnapshotToken: &finalSnapshotToken,
	})
	if err != nil || !renewed.OfflineSalesValidUntil.Equal(expiredAt.Add(devices.OfflineSalesLeaseDuration)) {
		t.Fatalf("expired lease renewal: %+v err=%v", renewed, err)
	}
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events WHERE tenant_id=$1 AND entity_id=$2 AND action='mobile.device.installation_acknowledged'),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.outbox_events WHERE tenant_id=$1 AND aggregate_id=$2 AND event_type='mobile.device.installation_acknowledged'),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.mobile_device_offline_leases WHERE tenant_id=$1 AND device_id=$2)`,
		tenantID, deviceID).Scan(&acknowledgementAudits, &acknowledgementEvents, &leaseCount); err != nil || acknowledgementAudits != 4 || acknowledgementEvents != 4 || leaseCount != 3 {
		t.Fatalf("expired renewal evidence audits=%d outbox=%d leases=%d err=%v", acknowledgementAudits, acknowledgementEvents, leaseCount, err)
	}
	const (
		unpublishedProductID = "10000000-0000-4000-8000-000000000006"
		reconciliationTxnID  = "10000000-0000-4000-8000-000000000007"
	)
	if _, err := pool.Exec(ctx, `INSERT INTO `+pgx.Identifier{schema}.Sanitize()+`.products
		(id,tenant_id,company_id,sku,name,currency,list_price_minor,standard_cost_minor,tax_code,revenue_account_id,cogs_account_id,inventory_account_id)
		VALUES($1,$2,$3,'LATE','Unpublished Product','TZS',10000,6000,'VAT','revenue','cogs','inventory')`,
		unpublishedProductID, tenantID, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO `+pgx.Identifier{schema}.Sanitize()+`.mobile_device_stock_allocations
		(tenant_id,company_id,branch_id,warehouse_id,device_id,product_id,allocated_quantity)
		VALUES ($1,$2,$3,$4,$5,$6,10)`, tenantID, companyID, branchID, warehouseID, deviceID, unpublishedProductID); err != nil {
		t.Fatal(err)
	}
	reconciliationCommand := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: unpublishedProductID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: reconciliationTxnID, ClientTimestamp: expiredAt.Add(time.Minute), AppVersion: "1.1.0",
		MasterDataVersion: taxMasterVersion, PriceVersion: finalPriceVersion, CatalogSnapshotToken: finalSnapshotToken,
		SyncAttempt: 1, Offline: true,
	}
	if _, err := renewalService.SyncSale(ctx, scope, userID, "", reconciliationCommand); !errors.Is(err, sales.ErrOfflineReconciliation) {
		t.Fatalf("missing publication reconciliation error=%v", err)
	}
	reconciliationCommand.SyncAttempt = 2
	if _, err := renewalService.SyncSale(ctx, scope, userID, "", reconciliationCommand); !errors.Is(err, sales.ErrOfflineReconciliation) {
		t.Fatalf("missing publication reconciliation replay error=%v", err)
	}
	cases, err := renewalService.ReconciliationCases(ctx, scope, userID, "OPEN", "", 10)
	if err != nil || len(cases.Items) != 1 || cases.Items[0].ClientTransactionID != reconciliationTxnID {
		t.Fatalf("PostgreSQL reconciliation cases=%+v err=%v", cases, err)
	}
	caseID := cases.Items[0].ID
	resolved, err := renewalService.ResolveReconciliation(ctx, mobile.ResolveReconciliationCommand{
		Scope: scope, ActorID: userID, CaseID: caseID, Action: mobile.ResolutionPostedExternally,
		Reason: "Verified in the legacy register", ExternalReference: "LEGACY-001",
		IdempotencyKey: "postgres-reconciliation-0001",
	})
	if err != nil || resolved.Status != mobile.ReconciliationResolved || resolved.Resolution == nil {
		t.Fatalf("PostgreSQL reconciliation resolution=%+v err=%v", resolved, err)
	}
	resolvedReplay, err := renewalService.ResolveReconciliation(ctx, mobile.ResolveReconciliationCommand{
		Scope: scope, ActorID: userID, CaseID: caseID, Action: mobile.ResolutionPostedExternally,
		Reason: "Verified in the legacy register", ExternalReference: "LEGACY-001",
		IdempotencyKey: "postgres-reconciliation-0001",
	})
	if err != nil || resolvedReplay.Resolution == nil || resolvedReplay.Resolution.ID != resolved.Resolution.ID {
		t.Fatalf("PostgreSQL resolution replay=%+v err=%v", resolvedReplay, err)
	}
	var caseCount, resolutionCount, saleCount, openedCount, resolvedCount int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.mobile_reconciliation_cases WHERE tenant_id=$1 AND id=$2),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.mobile_reconciliation_resolutions WHERE tenant_id=$1 AND case_id=$2),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.sales WHERE tenant_id=$1 AND device_id=$3 AND client_transaction_id=$4),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events WHERE tenant_id=$1 AND entity_id=$2 AND action='mobile.reconciliation.opened'),
		(SELECT count(*) FROM `+pgx.Identifier{schema}.Sanitize()+`.audit_events WHERE tenant_id=$1 AND entity_id=$2 AND action='mobile.reconciliation.resolved')`,
		tenantID, caseID, deviceID, reconciliationTxnID).Scan(&caseCount, &resolutionCount, &saleCount, &openedCount, &resolvedCount); err != nil ||
		caseCount != 1 || resolutionCount != 1 || saleCount != 0 || openedCount != 1 || resolvedCount != 1 {
		t.Fatalf("reconciliation persistence cases=%d resolutions=%d sales=%d opened=%d resolved=%d err=%v",
			caseCount, resolutionCount, saleCount, openedCount, resolvedCount, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE `+pgx.Identifier{schema}.Sanitize()+`.mobile_reconciliation_resolutions SET reason='Changed evidence' WHERE tenant_id=$1 AND case_id=$2`, tenantID, caseID); err == nil || !strings.Contains(err.Error(), "is append-only") {
		t.Fatalf("reconciliation resolution mutation error=%v", err)
	}
	canonicalizedScope := tenancy.Scope{
		TenantID: " " + strings.ToUpper(tenantID) + " ", CompanyID: " " + strings.ToUpper(companyID) + " ",
		BranchID: " " + strings.ToUpper(branchID) + " ", WarehouseID: " " + strings.ToUpper(warehouseID) + " ",
	}
	commands := []sales.CompleteCommand{
		{Scope: canonicalizedScope, CustomerID: " " + strings.ToUpper(customerID) + " ", Kind: sales.KindCash, PaymentMethod: sales.PaymentCash, Lines: []sales.CommandLine{{ProductID: " " + strings.ToUpper(productID) + " ", Quantity: 98}}, ActorID: " " + strings.ToUpper(userID) + " ", IdempotencyKey: "canonical-concurrency-one"},
		{Scope: canonicalizedScope, CustomerID: " " + strings.ToUpper(customerID) + " ", Kind: sales.KindCash, PaymentMethod: sales.PaymentCash, Lines: []sales.CommandLine{{ProductID: " " + strings.ToUpper(productID) + " ", Quantity: 98}}, ActorID: " " + strings.ToUpper(userID) + " ", IdempotencyKey: "canonical-concurrency-two"},
	}
	results := make(chan error, len(commands))
	for _, concurrentCommand := range commands {
		go func() {
			_, completeErr := service.Complete(ctx, concurrentCommand)
			results <- completeErr
		}()
	}
	var successful, insufficient int
	for range commands {
		completeErr := <-results
		switch {
		case completeErr == nil:
			successful++
		case errors.Is(completeErr, sales.ErrInsufficientStock):
			insufficient++
		default:
			t.Fatalf("canonical concurrent sale error=%v", completeErr)
		}
	}
	if successful != 1 || insufficient != 1 {
		t.Fatalf("canonical concurrent outcomes success=%d insufficient=%d", successful, insufficient)
	}
	deliveryNow := time.Now().UTC().Add(time.Minute)
	tenants, err := store.TenantIDs(ctx)
	if err != nil || len(tenants) != 1 || tenants[0] != tenantID {
		t.Fatalf("list outbox tenants: %v %v", tenants, err)
	}
	events, err := store.Claim(ctx, tenantID, "integration-worker", 10, deliveryNow, deliveryNow.Add(time.Minute))
	if err != nil || len(events) != 10 {
		t.Fatalf("claim outbox: count=%d err=%v", len(events), err)
	}
	for _, event := range events {
		if event.CorrelationID == "" || event.CausationID == "" {
			t.Fatalf("event context missing: %+v", event)
		}
		if event.AggregateID == synced.Sale.ID {
			var eventSale sales.Sale
			if err := json.Unmarshal(event.Payload, &eventSale); err != nil || eventSale.ClientTimestamp == nil ||
				eventSale.AppVersion != "1.0.0" || eventSale.MasterDataVersion != masterVersion || eventSale.PriceVersion != priceVersion || eventSale.CatalogSnapshotToken != snapshotToken {
				t.Fatalf("mobile provenance missing from outbox payload: %+v err=%v", eventSale, err)
			}
		}
	}
	if err := store.MarkPublished(ctx, tenantID, events[0].ID, "integration-worker", deliveryNow); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	if err := store.MarkFailed(ctx, tenantID, events[1].ID, "integration-worker", deliveryNow.Add(time.Minute), "test retry"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	check, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = check.Rollback(context.Background()) }()
	_, _ = check.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()+", public")
	var stock, salesCount, journalCount, outboxCount, processedCount int64
	if err := check.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0) FROM inventory_stock_ledger`).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(ctx, `SELECT count(*) FROM sales`).Scan(&salesCount); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(ctx, `SELECT count(*) FROM journals`).Scan(&journalCount); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE processed_at IS NOT NULL`).Scan(&processedCount); err != nil {
		t.Fatal(err)
	}
	if stock != 1 || salesCount != 7 || journalCount != 28 || outboxCount != 105 || processedCount != 1 {
		t.Fatalf("stock=%d sales=%d journals=%d outbox=%d processed=%d", stock, salesCount, journalCount, outboxCount, processedCount)
	}
}

func TestRestrictedRuntimeRoleEnforcesTenantRLS(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clusterPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterPool.Close()
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	suffix = strings.ReplaceAll(suffix, ".", "")
	databaseName := "itembaz_caps_" + suffix
	if _, err := clusterPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		t.Fatal(err)
	}

	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.ConnConfig.Database = databaseName
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	apiUser, workerUser := "itembaz_api_"+suffix, "itembaz_worker_"+suffix
	defer func() {
		adminPool.Close()
		_, _ = clusterPool.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)")
		_, _ = clusterPool.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{apiUser}.Sanitize())
		_, _ = clusterPool.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{workerUser}.Sanitize())
	}()
	for _, name := range []string{"000001_core.up.sql", "000002_live_golden.up.sql", "000003_runtime_security.up.sql", "000004_runtime_capabilities.up.sql", "000005_offline_and_version_ack.up.sql", "000006_version_ack_serialization.up.sql", "000007_offline_sales_leases.up.sql", "000008_catalog_snapshot_tokens.up.sql", "000009_catalog_publications.up.sql", "000010_mobile_reconciliation.up.sql", "000011_offline_posting_policy.up.sql", "000012_mobile_device_governance.up.sql", "000013_customer_receivables.up.sql", "000014_commercial_operations.up.sql"} {
		applyTestMigration(t, ctx, adminPool, "itembaz", name)
	}
	const (
		tenantA  = "60000000-0000-4000-8000-000000000001"
		tenantB  = "60000000-0000-4000-8000-000000000002"
		companyA = "60000000-0000-4000-8000-000000000003"
		companyB = "60000000-0000-4000-8000-000000000004"
		eventA   = "60000000-0000-4000-8000-000000000005"
		eventB   = "60000000-0000-4000-8000-000000000006"
	)
	if _, err := adminPool.Exec(ctx, `INSERT INTO itembaz.tenants(id,name) VALUES($1,'Tenant A'),($2,'Tenant B')`, tenantA, tenantB); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `INSERT INTO itembaz.legal_companies(id,tenant_id,name) VALUES($1,$2,'Company A'),($3,$4,'Company B')`, companyA, tenantA, companyB, tenantB); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `INSERT INTO itembaz.outbox_events
		(id,tenant_id,company_id,aggregate_type,aggregate_id,event_type,version,correlation_id,causation_id,payload,occurred_at,available_at)
		VALUES ($1,$2,$3,'test',$1,'test.created',1,$1,'seed','{}',now()-interval '1 minute',now()-interval '1 minute'),
		       ($4,$5,$6,'test',$4,'test.created',1,$4,'seed','{}',now()-interval '1 minute',now()-interval '1 minute')`,
		eventA, tenantA, companyA, eventB, tenantB, companyB); err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `CREATE TABLE itembaz.future_private(id integer); CREATE FUNCTION itembaz.future_private_function() RETURNS integer LANGUAGE sql AS 'SELECT 1'`); err != nil {
		t.Fatal(err)
	}

	const apiPassword = "integration-api-password"
	const workerPassword = "integration-worker-password"
	if err := dbrole.Provision(ctx, adminPool, apiUser, apiPassword, dbrole.CapabilityAPI); err != nil {
		t.Fatalf("provision API login: %v", err)
	}
	if err := dbrole.Provision(ctx, adminPool, workerUser, workerPassword, dbrole.CapabilityWorker); err != nil {
		t.Fatalf("provision worker login: %v", err)
	}
	var apiLeaseSelect, apiLeaseInsert, apiLeaseUpdate, apiLeaseDelete, workerLeaseSelect, apiLeaseColumnUpdate bool
	if err := adminPool.QueryRow(ctx, `
		SELECT has_table_privilege($1, 'itembaz.mobile_device_offline_leases', 'SELECT'),
		       has_table_privilege($1, 'itembaz.mobile_device_offline_leases', 'INSERT'),
		       has_table_privilege($1, 'itembaz.mobile_device_offline_leases', 'UPDATE'),
		       has_table_privilege($1, 'itembaz.mobile_device_offline_leases', 'DELETE'),
		       has_table_privilege($2, 'itembaz.mobile_device_offline_leases', 'SELECT'),
		       has_column_privilege($1, 'itembaz.mobile_devices', 'offline_sales_valid_until', 'UPDATE')`, apiUser, workerUser).Scan(
		&apiLeaseSelect, &apiLeaseInsert, &apiLeaseUpdate, &apiLeaseDelete, &workerLeaseSelect, &apiLeaseColumnUpdate); err != nil {
		t.Fatal(err)
	}
	if !apiLeaseSelect || !apiLeaseInsert || !apiLeaseColumnUpdate || apiLeaseUpdate || apiLeaseDelete || workerLeaseSelect {
		t.Fatalf("unsafe lease capabilities api(select=%v insert=%v update=%v delete=%v column_update=%v) worker_select=%v",
			apiLeaseSelect, apiLeaseInsert, apiLeaseUpdate, apiLeaseDelete, apiLeaseColumnUpdate, workerLeaseSelect)
	}
	var apiPublicationSelect, apiPublicationInsert, apiPublicationUpdate, apiPublicationDelete, workerPublicationSelect bool
	if err := adminPool.QueryRow(ctx, `
		SELECT has_table_privilege($1, 'itembaz.catalog_publications', 'SELECT'),
		       has_table_privilege($1, 'itembaz.catalog_publications', 'INSERT'),
		       has_table_privilege($1, 'itembaz.catalog_publications', 'UPDATE'),
		       has_table_privilege($1, 'itembaz.catalog_publications', 'DELETE'),
		       has_table_privilege($2, 'itembaz.catalog_publications', 'SELECT')`, apiUser, workerUser).Scan(
		&apiPublicationSelect, &apiPublicationInsert, &apiPublicationUpdate, &apiPublicationDelete, &workerPublicationSelect); err != nil {
		t.Fatal(err)
	}
	if !apiPublicationSelect || !apiPublicationInsert || apiPublicationUpdate || apiPublicationDelete || workerPublicationSelect {
		t.Fatalf("unsafe publication capabilities api(select=%v insert=%v update=%v delete=%v) worker_select=%v",
			apiPublicationSelect, apiPublicationInsert, apiPublicationUpdate, apiPublicationDelete, workerPublicationSelect)
	}
	var apiCaseSelect, apiCaseInsert, apiCaseUpdate, apiCaseDelete, workerCaseSelect bool
	if err := adminPool.QueryRow(ctx, `
		SELECT has_table_privilege($1, 'itembaz.mobile_reconciliation_cases', 'SELECT'),
		       has_table_privilege($1, 'itembaz.mobile_reconciliation_cases', 'INSERT'),
		       has_table_privilege($1, 'itembaz.mobile_reconciliation_cases', 'UPDATE'),
		       has_table_privilege($1, 'itembaz.mobile_reconciliation_cases', 'DELETE'),
		       has_table_privilege($2, 'itembaz.mobile_reconciliation_cases', 'SELECT')`, apiUser, workerUser).Scan(
		&apiCaseSelect, &apiCaseInsert, &apiCaseUpdate, &apiCaseDelete, &workerCaseSelect); err != nil {
		t.Fatal(err)
	}
	if !apiCaseSelect || !apiCaseInsert || apiCaseUpdate || apiCaseDelete || workerCaseSelect {
		t.Fatalf("unsafe reconciliation capabilities api(select=%v insert=%v update=%v delete=%v) worker_select=%v",
			apiCaseSelect, apiCaseInsert, apiCaseUpdate, apiCaseDelete, workerCaseSelect)
	}
	var apiDeviceChangeSelect, apiDeviceChangeInsert, apiDeviceChangeUpdate, apiDeviceChangeDelete bool
	var workerDeviceChangeSelect, apiDeviceStatusUpdate, apiAllocationUpdate bool
	if err := adminPool.QueryRow(ctx, `
		SELECT has_table_privilege($1, 'itembaz.mobile_device_status_changes', 'SELECT'),
		       has_table_privilege($1, 'itembaz.mobile_device_status_changes', 'INSERT'),
		       has_table_privilege($1, 'itembaz.mobile_device_status_changes', 'UPDATE'),
		       has_table_privilege($1, 'itembaz.mobile_device_status_changes', 'DELETE'),
		       has_table_privilege($2, 'itembaz.mobile_device_status_changes', 'SELECT'),
		       has_column_privilege($1, 'itembaz.mobile_devices', 'status', 'UPDATE'),
		       has_column_privilege($1, 'itembaz.mobile_device_stock_allocations', 'allocated_quantity', 'UPDATE')`,
		apiUser, workerUser).Scan(&apiDeviceChangeSelect, &apiDeviceChangeInsert, &apiDeviceChangeUpdate,
		&apiDeviceChangeDelete, &workerDeviceChangeSelect, &apiDeviceStatusUpdate, &apiAllocationUpdate); err != nil {
		t.Fatal(err)
	}
	if !apiDeviceChangeSelect || !apiDeviceChangeInsert || !apiDeviceStatusUpdate || !apiAllocationUpdate ||
		apiDeviceChangeUpdate || apiDeviceChangeDelete || workerDeviceChangeSelect {
		t.Fatalf("unsafe device governance capabilities api(select=%v insert=%v update=%v delete=%v status_update=%v allocation_update=%v) worker_select=%v",
			apiDeviceChangeSelect, apiDeviceChangeInsert, apiDeviceChangeUpdate, apiDeviceChangeDelete,
			apiDeviceStatusUpdate, apiAllocationUpdate, workerDeviceChangeSelect)
	}
	var futureACL string
	var apiFutureExecute bool
	if err := adminPool.QueryRow(ctx, `SELECT COALESCE(p.proacl::text,''), has_function_privilege($1, p.oid, 'EXECUTE') FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='itembaz' AND p.proname='future_private_function'`, apiUser).Scan(&futureACL, &apiFutureExecute); err != nil {
		t.Fatal(err)
	}
	if apiFutureExecute {
		t.Fatalf("unsafe future function defaults: acl=%s", futureACL)
	}
	apiURL := databaseURLWithIdentity(t, databaseURL, databaseName, apiUser, apiPassword)
	workerURL := databaseURLWithIdentity(t, databaseURL, databaseName, workerUser, workerPassword)

	apiStore, err := postgres.Open(ctx, apiURL)
	if err != nil {
		t.Fatalf("open API store: %v", err)
	}
	defer apiStore.Close()
	if wrong, err := postgres.OpenWorker(ctx, apiURL); err == nil {
		wrong.Close()
		t.Fatal("API login was accepted as an outbox worker")
	}
	workerStore, err := postgres.OpenWorker(ctx, workerURL)
	if err != nil {
		t.Fatalf("open worker store: %v", err)
	}
	defer workerStore.Close()
	if wrong, err := postgres.Open(ctx, workerURL); err == nil {
		wrong.Close()
		t.Fatal("worker login was accepted as an API")
	}

	apiPool := apiStore.Pool()
	connection, err := apiPool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, "SET search_path TO itembaz, public"); err != nil {
		t.Fatal(err)
	}
	var superuser, bypassRLS, ownsTables bool
	if err := connection.QueryRow(ctx, `
		SELECT r.rolsuper, r.rolbypassrls, EXISTS(
			SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
			WHERE n.nspname=$1 AND c.relowner=r.oid AND c.relkind IN ('r','p')
		) FROM pg_roles r WHERE r.rolname=current_user`, "itembaz").Scan(&superuser, &bypassRLS, &ownsTables); err != nil {
		t.Fatal(err)
	}
	if superuser || bypassRLS || ownsTables {
		t.Fatalf("unsafe runtime identity: superuser=%v bypass_rls=%v owns_tables=%v", superuser, bypassRLS, ownsTables)
	}
	var count int
	if err := connection.QueryRow(ctx, `SELECT count(*) FROM legal_companies`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unset tenant context exposed %d companies: %v", count, err)
	}
	if _, err := connection.Exec(ctx, `SELECT set_config('app.tenant_id',$1,false)`, tenantA); err != nil {
		t.Fatal(err)
	}
	if err := connection.QueryRow(ctx, `SELECT count(*) FROM legal_companies WHERE tenant_id=$1`, tenantB).Scan(&count); err != nil || count != 0 {
		t.Fatalf("wrong tenant context exposed %d companies: %v", count, err)
	}
	if err := connection.QueryRow(ctx, `SELECT count(*) FROM legal_companies`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("correct tenant context returned %d companies: %v", count, err)
	}
	if _, err := connection.Exec(ctx, `INSERT INTO catalog_publications
		(tenant_id,company_id,catalog_snapshot_token,master_data_version,price_version)
		SELECT tenant_id,id,catalog_snapshot_token,master_data_version,price_version
		FROM legal_companies WHERE tenant_id=$1 AND id=$2`, tenantA, companyA); err != nil {
		t.Fatalf("API lost required publication insert capability: %v", err)
	}
	if _, err := connection.Exec(ctx, `UPDATE catalog_publications SET price_version=price_version+1 WHERE tenant_id=$1`, tenantA); err == nil {
		t.Fatal("API could mutate immutable catalog publication evidence")
	}
	if _, err := connection.Exec(ctx, `INSERT INTO legal_companies(id,tenant_id,name) VALUES($1,$2,'Forbidden')`,
		"60000000-0000-4000-8000-000000000005", tenantB); err == nil {
		t.Fatal("RLS accepted a cross-tenant insert")
	}
	for name, query := range map[string]string{
		"global tenants":          `SELECT count(*) FROM tenants`,
		"worker discovery":        `SELECT count(*) FROM pending_outbox_tenant_ids()`,
		"outbox delivery state":   `SELECT count(*) FROM outbox_events`,
		"future table default":    `SELECT count(*) FROM future_private`,
		"future function default": `SELECT future_private_function()`,
	} {
		if err := connection.QueryRow(ctx, query).Scan(&count); err == nil {
			t.Fatalf("API could access %s", name)
		}
	}
	if _, err := connection.Exec(ctx, `UPDATE outbox_events SET processed_at=now()`); err == nil {
		t.Fatal("API could mutate outbox delivery state")
	}
	if _, err := connection.Exec(ctx, `INSERT INTO outbox_events
		(id,tenant_id,company_id,aggregate_type,aggregate_id,event_type,version,correlation_id,causation_id,payload,occurred_at)
		VALUES ('60000000-0000-4000-8000-000000000007',$1,$2,'test','60000000-0000-4000-8000-000000000007','api.created',1,'60000000-0000-4000-8000-000000000007','api','{}',now())`, tenantA, companyA); err != nil {
		t.Fatalf("API lost required outbox insert capability: %v", err)
	}

	workerConnection, err := workerStore.Pool().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer workerConnection.Release()
	if _, err := workerConnection.Exec(ctx, `SET search_path TO itembaz, public`); err != nil {
		t.Fatal(err)
	}
	if err := workerConnection.QueryRow(ctx, `SELECT count(*) FROM tenants`).Scan(&count); err == nil {
		t.Fatal("worker could read global tenant metadata directly")
	}
	if err := workerConnection.QueryRow(ctx, `SELECT count(*) FROM legal_companies`).Scan(&count); err == nil {
		t.Fatal("worker could read API business tables")
	}
	if _, err := workerConnection.Exec(ctx, `INSERT INTO outbox_events
		(id,tenant_id,company_id,aggregate_type,aggregate_id,event_type,version,correlation_id,causation_id,payload,occurred_at)
		VALUES ('60000000-0000-4000-8000-000000000008',$1,$2,'test','60000000-0000-4000-8000-000000000008','worker.created',1,'60000000-0000-4000-8000-000000000008','worker','{}',now())`, tenantA, companyA); err == nil {
		t.Fatal("worker could insert API-owned outbox events")
	}
	tenants, err := workerStore.TenantIDs(ctx)
	if err != nil || len(tenants) != 2 || tenants[0] != tenantA || tenants[1] != tenantB {
		t.Fatalf("worker pending tenant discovery=%v err=%v", tenants, err)
	}
	now := time.Now().UTC()
	events, err := workerStore.Claim(ctx, tenantA, "capability-test", 10, now, now.Add(time.Minute))
	if err != nil || len(events) != 2 {
		t.Fatalf("worker claim count=%d err=%v", len(events), err)
	}
	if err := workerStore.MarkPublished(ctx, tenantA, events[0].ID, "capability-test", now); err != nil {
		t.Fatalf("worker mark published: %v", err)
	}
}

func TestLiveGoldenMigrationUpgradesExistingPostedSale(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "itembaz_upgrade_" + time.Now().UTC().Format("20060102150405")
	applyTestMigration(t, ctx, pool, schema, "000001_core.up.sql")
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	}()
	seed, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer seed.Rollback(context.Background())
	if _, err := seed.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()+", public"); err != nil {
		t.Fatal(err)
	}
	const (
		tenantID    = "50000000-0000-4000-8000-000000000001"
		companyID   = "50000000-0000-4000-8000-000000000002"
		branchID    = "50000000-0000-4000-8000-000000000003"
		warehouseID = "50000000-0000-4000-8000-000000000004"
		userID      = "50000000-0000-4000-8000-000000000005"
		customerID  = "50000000-0000-4000-8000-000000000006"
		saleID      = "50000000-0000-4000-8000-000000000007"
	)
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenants(id,name) VALUES($1,'Tenant')`, []any{tenantID}},
		{`INSERT INTO legal_companies(id,tenant_id,name) VALUES($1,$2,'Company')`, []any{companyID, tenantID}},
		{`INSERT INTO branches(id,tenant_id,company_id,name) VALUES($1,$2,$3,'Branch')`, []any{branchID, tenantID, companyID}},
		{`INSERT INTO warehouses(id,tenant_id,company_id,branch_id,name) VALUES($1,$2,$3,$4,'Warehouse')`, []any{warehouseID, tenantID, companyID, branchID}},
		{`INSERT INTO users(id,tenant_id,email) VALUES($1,$2,'upgrade@example.test')`, []any{userID, tenantID}},
		{`INSERT INTO customer_accounts(id,tenant_id,company_id,name,active,is_general) VALUES($1,$2,$3,'General Customer',true,true)`, []any{customerID, tenantID, companyID}},
		{`INSERT INTO sales(id,tenant_id,company_id,branch_id,warehouse_id,record_type,sale_kind,status,customer_id,currency,subtotal_minor,tax_minor,total_minor,cogs_minor,payment_method,created_by,correlation_id,created_at) VALUES($1,$2,$3,$4,$5,'SALE','CASH','POSTED',$6,'TZS',100,0,100,0,'CASH',$7,$1,now())`, []any{saleID, tenantID, companyID, branchID, warehouseID, customerID, userID}},
	}
	for _, statement := range statements {
		if _, err := seed.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed upgrade fixture: %v", err)
		}
	}
	if err := seed.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	applyTestMigration(t, ctx, pool, schema, "000002_live_golden.up.sql")
	check, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Rollback(context.Background())
	if _, err := check.Exec(ctx, "SET LOCAL search_path TO "+pgx.Identifier{schema}.Sanitize()+", public"); err != nil {
		t.Fatal(err)
	}
	var receiptReference, fiscalStatus, triggerEnabled string
	if err := check.QueryRow(ctx, `SELECT receipt_reference, fiscal_status FROM sales WHERE id=$1`, saleID).Scan(&receiptReference, &fiscalStatus); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(ctx, `SELECT tgenabled::text FROM pg_trigger WHERE tgrelid='sales'::regclass AND tgname='sales_immutable'`).Scan(&triggerEnabled); err != nil {
		t.Fatal(err)
	}
	if receiptReference != saleID || fiscalStatus != "NOT_CONFIGURED" || triggerEnabled != "O" {
		t.Fatalf("receipt=%q fiscal=%q trigger=%q", receiptReference, fiscalStatus, triggerEnabled)
	}
	if _, err := check.Exec(ctx, `UPDATE sales SET receipt_reference='changed' WHERE id=$1`, saleID); err == nil {
		t.Fatal("posted-sale immutability trigger was not restored")
	}
}

func applyTestMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, name string) {
	t.Helper()
	migrationPath := filepath.Join("..", "..", "..", "..", "migrations", name)
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	script := regexp.MustCompile(`\bitembaz\b`).ReplaceAllString(string(migration), schema)
	if _, err := pool.Exec(ctx, script); err != nil {
		t.Fatalf("apply migration %s: %v", name, err)
	}
}

func databaseURLWithIdentity(t *testing.T, baseURL, database, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(username, password)
	parsed.Path = "/" + database
	return parsed.String()
}
