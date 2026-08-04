package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
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
	migrationPath := filepath.Join("..", "..", "..", "..", "migrations", "000001_core.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(migration), "itembaz", schema)
	if _, err := pool.Exec(ctx, script); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	}()
	store, err := postgres.New(pool, schema)
	if err != nil {
		t.Fatal(err)
	}

	const (
		tenantID    = "00000000-0000-0000-0000-000000000001"
		companyID   = "00000000-0000-0000-0000-000000000002"
		branchID    = "00000000-0000-0000-0000-000000000003"
		warehouseID = "00000000-0000-0000-0000-000000000004"
		userID      = "00000000-0000-0000-0000-000000000005"
		roleID      = "00000000-0000-0000-0000-000000000006"
		scopeID     = "00000000-0000-0000-0000-000000000007"
		customerID  = "00000000-0000-0000-0000-000000000008"
		productID   = "00000000-0000-0000-0000-000000000009"
		taxID       = "00000000-0000-0000-0000-00000000000a"
		periodID    = "00000000-0000-0000-0000-00000000000b"
		stockID     = "00000000-0000-0000-0000-00000000000c"
		openingID   = "00000000-0000-0000-0000-00000000000d"
	)
	testTime := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
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
		{`INSERT INTO branches(id,tenant_id,company_id,name) VALUES($1,$2,$3,'Branch')`, []any{branchID, tenantID, companyID}},
		{`INSERT INTO warehouses(id,tenant_id,company_id,branch_id,name) VALUES($1,$2,$3,$4,'Warehouse')`, []any{warehouseID, tenantID, companyID, branchID}},
		{`INSERT INTO users(id,tenant_id,email) VALUES($1,$2,'user@example.test')`, []any{userID, tenantID}},
		{`INSERT INTO roles(id,tenant_id,name) VALUES($1,$2,'Manager')`, []any{roleID, tenantID}},
		{`INSERT INTO role_permissions(tenant_id,role_id,permission_code) SELECT $1,$2,code FROM permissions`, []any{tenantID, roleID}},
		{`INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, []any{scopeID, tenantID, userID, roleID, companyID, branchID, warehouseID}},
		{`INSERT INTO customer_accounts(id,tenant_id,company_id,name,active,is_general) VALUES($1,$2,$3,'General Customer',true,true)`, []any{customerID, tenantID, companyID}},
		{`INSERT INTO products(id,tenant_id,company_id,sku,name,currency,list_price_minor,standard_cost_minor,tax_code,revenue_account_id,cogs_account_id,inventory_account_id) VALUES($1,$2,$3,'SKU','Product','TZS',10000,6000,'VAT','revenue','cogs','inventory')`, []any{productID, tenantID, companyID}},
		{`INSERT INTO tax_rules(id,tenant_id,company_id,code,basis_points,effective_from) VALUES($1,$2,$3,'VAT',1800,$4)`, []any{taxID, tenantID, companyID, testTime.AddDate(-1, 0, 0)}},
		{`INSERT INTO fiscal_periods(id,tenant_id,company_id,starts_at,ends_at,is_open) VALUES($1,$2,$3,$4,$5,true)`, []any{periodID, tenantID, companyID, testTime.AddDate(0, -1, 0), testTime.AddDate(0, 1, 0)}},
		{`INSERT INTO sales_posting_config(tenant_id,company_id,receivable_account_id,tax_payable_account_id,cash_accounts) VALUES($1,$2,'receivable','tax-payable','{"CASH":"cash"}')`, []any{tenantID, companyID}},
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
	reversal, err := service.Reverse(ctx, sales.ReverseCommand{Scope: scope, SaleID: created.ID, Reason: "Integration return", ActorID: userID, IdempotencyKey: "integration-reversal-key"})
	if err != nil || reversal.ReversalOf != created.ID {
		t.Fatalf("reverse: %+v %v", reversal, err)
	}
	deliveryNow := time.Now().UTC().Add(time.Minute)
	tenants, err := store.TenantIDs(ctx)
	if err != nil || len(tenants) != 1 || tenants[0] != tenantID {
		t.Fatalf("list outbox tenants: %v %v", tenants, err)
	}
	events, err := store.Claim(ctx, tenantID, "integration-worker", 10, deliveryNow, deliveryNow.Add(time.Minute))
	if err != nil || len(events) != 2 {
		t.Fatalf("claim outbox: count=%d err=%v", len(events), err)
	}
	for _, event := range events {
		if event.CorrelationID == "" || event.CausationID == "" {
			t.Fatalf("event context missing: %+v", event)
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
	if stock != 100 || salesCount != 2 || journalCount != 2 || outboxCount != 2 || processedCount != 1 {
		t.Fatalf("stock=%d sales=%d journals=%d outbox=%d processed=%d", stock, salesCount, journalCount, outboxCount, processedCount)
	}
}
