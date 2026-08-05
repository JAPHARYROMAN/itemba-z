// Command devseed installs a deterministic, non-production golden-sale data set.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	tenantID               = "00000000-0000-4000-8000-000000000001"
	companyID              = "00000000-0000-4000-8000-000000000002"
	branchID               = "00000000-0000-4000-8000-000000000003"
	warehouseID            = "00000000-0000-4000-8000-000000000004"
	operatorID             = "00000000-0000-4000-8000-000000000005"
	roleID                 = "00000000-0000-4000-8000-000000000006"
	generalID              = "00000000-0000-4000-8000-000000000007"
	creditID               = "00000000-0000-4000-8000-000000000008"
	productOneID           = "00000000-0000-4000-8000-000000000009"
	productTwoID           = "00000000-0000-4000-8000-000000000010"
	deviceID               = "00000000-0000-4000-8000-000000000011"
	roleScopeID            = "00000000-0000-4000-8000-000000000012"
	taxRuleID              = "00000000-0000-4000-8000-000000000013"
	fiscalPeriodID         = "00000000-0000-4000-8000-000000000014"
	offlinePostingPolicyID = "00000000-0000-4000-8000-000000000018"
	stockOneID             = "00000000-0000-4000-8000-000000000015"
	stockTwoID             = "00000000-0000-4000-8000-000000000016"
	seedSourceID           = "00000000-0000-4000-8000-000000000017"
	supplierID             = "00000000-0000-4000-8000-000000000019"

	offlineTransactionLimitMinor int64 = 20_000_000
	offlineDailyLimitMinor       int64 = 50_000_000
	offlineProductAllocation     int64 = 25
)

func main() {
	if err := run(context.Background(), os.Getenv("ITEMBA_ENV"), os.Getenv("DATABASE_URL")); err != nil {
		fmt.Fprintln(os.Stderr, "devseed:", err)
		os.Exit(1)
	}
	fmt.Println("ITEMBA-Z development seed is ready")
}

func run(ctx context.Context, environment, databaseURL string) error {
	if !safeEnvironment(environment) {
		return fmt.Errorf("refusing to seed ITEMBA_ENV=%q; use development, dev, local, or test", environment)
	}
	if strings.TrimSpace(databaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer connection.Close(context.Background())
	tx, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SET LOCAL search_path TO itembaz, public`); err != nil {
		return fmt.Errorf("select schema: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}
	if err := seed(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

func safeEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

func seed(ctx context.Context, tx pgx.Tx) error {
	statements := []struct {
		name string
		sql  string
		args []any
	}{
		{"tenant", `INSERT INTO tenants(id,name) VALUES($1,'Itemba Group DEV') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name`, []any{tenantID}},
		{"company", `INSERT INTO legal_companies(id,tenant_id,name,base_currency,master_data_version,price_version,business_timezone) VALUES($1,$2,'Itemba Trading DEV','TZS',1,1,'Africa/Dar_es_Salaam') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,base_currency=EXCLUDED.base_currency,business_timezone=EXCLUDED.business_timezone`, []any{companyID, tenantID}},
		{"branch", `INSERT INTO branches(id,tenant_id,company_id,name) VALUES($1,$2,$3,'Dar es Salaam DEV') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name`, []any{branchID, tenantID, companyID}},
		{"warehouse", `INSERT INTO warehouses(id,tenant_id,company_id,branch_id,name) VALUES($1,$2,$3,$4,'Main Warehouse DEV') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name`, []any{warehouseID, tenantID, companyID, branchID}},
		{"operator", `INSERT INTO users(id,tenant_id,email,display_name,active) VALUES($1,$2,'operator@itemba.invalid','Development Operator',true) ON CONFLICT(id) DO UPDATE SET display_name=EXCLUDED.display_name,active=true`, []any{operatorID, tenantID}},
		{"role", `INSERT INTO roles(id,tenant_id,name) VALUES($1,$2,'Golden Sale Operator') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name`, []any{roleID, tenantID}},
		{"permissions", `INSERT INTO role_permissions(tenant_id,role_id,permission_code) SELECT $1,$2,code FROM permissions WHERE code IN ('sales.read','sales.complete','sales.reverse','customers.read','customers.accounts.read','customers.credit.manage','customers.collections.post','products.read','mobile.devices.enroll','mobile.devices.read','mobile.devices.manage','mobile.sales.sync','mobile.reconciliation.read','mobile.reconciliation.resolve','operations.read','sales.orders.manage','purchases.requests.manage','purchases.orders.manage','purchases.receive','purchases.invoices.post','purchases.payments.post','inventory.transfers.manage','inventory.counts.manage','inventory.adjustments.post') ON CONFLICT DO NOTHING`, []any{tenantID, roleID}},
		{"role scope", `INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,user_id,role_id,company_id,branch_id,warehouse_id) DO NOTHING`, []any{roleScopeID, tenantID, operatorID, roleID, companyID, branchID, warehouseID}},
		{"general customer", `INSERT INTO customer_accounts(id,tenant_id,company_id,code,name,active,is_general,credit_enabled,credit_limit_minor) VALUES($1,$2,$3,'GENERAL','General Customer',true,true,false,0) ON CONFLICT(id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,active=true`, []any{generalID, tenantID, companyID}},
		{"credit customer", `INSERT INTO customer_accounts(id,tenant_id,company_id,code,name,active,is_general,credit_enabled,credit_limit_minor) VALUES($1,$2,$3,'CREDIT-001','Amani Stores DEV',true,false,true,50000000) ON CONFLICT(id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,active=true`, []any{creditID, tenantID, companyID}},
		{"supplier", `INSERT INTO suppliers(id,tenant_id,company_id,code,name,active,payment_terms_days) VALUES($1,$2,$3,'SUP-001','Mwanza Packaging Works DEV',true,30) ON CONFLICT(id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,active=true,payment_terms_days=30`, []any{supplierID, tenantID, companyID}},
		{"zero tax", `INSERT INTO tax_rules(id,tenant_id,company_id,code,basis_points,effective_from) VALUES($1,$2,$3,'DEV_ZERO',0,'2020-01-01T00:00:00Z') ON CONFLICT(tenant_id,company_id,code,effective_from) DO UPDATE SET basis_points=0,effective_to=NULL`, []any{taxRuleID, tenantID, companyID}},
		{"product one", `INSERT INTO products(id,tenant_id,company_id,sku,name,base_unit_code,active,currency,list_price_minor,standard_cost_minor,tax_code,revenue_account_id,cogs_account_id,inventory_account_id,price_version,master_data_version) VALUES($1,$2,$3,'DEV-RICE-25','Rice 25kg DEV','BAG',true,'TZS',8500000,7000000,'DEV_ZERO','revenue','cogs','inventory',1,1) ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,active=true,list_price_minor=EXCLUDED.list_price_minor,standard_cost_minor=EXCLUDED.standard_cost_minor,tax_code=EXCLUDED.tax_code`, []any{productOneID, tenantID, companyID}},
		{"product two", `INSERT INTO products(id,tenant_id,company_id,sku,name,base_unit_code,active,currency,list_price_minor,standard_cost_minor,tax_code,revenue_account_id,cogs_account_id,inventory_account_id,price_version,master_data_version) VALUES($1,$2,$3,'DEV-OIL-5','Cooking Oil 5L DEV','JAR',true,'TZS',2500000,1900000,'DEV_ZERO','revenue','cogs','inventory',1,1) ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,active=true,list_price_minor=EXCLUDED.list_price_minor,standard_cost_minor=EXCLUDED.standard_cost_minor,tax_code=EXCLUDED.tax_code`, []any{productTwoID, tenantID, companyID}},
		{"offline device", `INSERT INTO mobile_devices(id,tenant_id,company_id,branch_id,warehouse_id,actor_id,status,device_name,app_version,master_data_version,price_version,offline_enabled,offline_transaction_limit_minor,offline_daily_limit_minor,enrolled_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$6,'ACTIVE','ITEMBA-Z DEV POS','devseed',1,1,true,$7,$8,'2020-01-01T00:00:00Z','2020-01-01T00:00:00Z') ON CONFLICT(tenant_id,id) DO UPDATE SET company_id=EXCLUDED.company_id,branch_id=EXCLUDED.branch_id,warehouse_id=EXCLUDED.warehouse_id,actor_id=EXCLUDED.actor_id,status='ACTIVE',device_name=EXCLUDED.device_name,app_version=EXCLUDED.app_version,master_data_version=EXCLUDED.master_data_version,price_version=EXCLUDED.price_version,offline_enabled=true,offline_transaction_limit_minor=EXCLUDED.offline_transaction_limit_minor,offline_daily_limit_minor=EXCLUDED.offline_daily_limit_minor,last_seen_at=GREATEST(mobile_devices.last_seen_at,EXCLUDED.last_seen_at)`, []any{deviceID, tenantID, companyID, branchID, warehouseID, operatorID, offlineTransactionLimitMinor, offlineDailyLimitMinor}},
		{"authoritative product cache versions", `UPDATE products AS p SET master_data_version=c.master_data_version,price_version=c.price_version FROM legal_companies AS c WHERE p.tenant_id=$1 AND p.company_id=$2 AND c.tenant_id=p.tenant_id AND c.id=p.company_id`, []any{tenantID, companyID}},
		{"authoritative device cache versions", `UPDATE mobile_devices AS d SET master_data_version=c.master_data_version,price_version=c.price_version,catalog_snapshot_token=c.catalog_snapshot_token FROM legal_companies AS c WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.id=$3 AND c.tenant_id=d.tenant_id AND c.id=d.company_id`, []any{tenantID, companyID, deviceID}},
		{"product one offline allocation", `INSERT INTO mobile_device_stock_allocations(tenant_id,company_id,branch_id,warehouse_id,device_id,product_id,allocated_quantity) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,device_id,product_id) DO UPDATE SET company_id=EXCLUDED.company_id,branch_id=EXCLUDED.branch_id,warehouse_id=EXCLUDED.warehouse_id,allocated_quantity=EXCLUDED.allocated_quantity,updated_at=now()`, []any{tenantID, companyID, branchID, warehouseID, deviceID, productOneID, offlineProductAllocation}},
		{"product two offline allocation", `INSERT INTO mobile_device_stock_allocations(tenant_id,company_id,branch_id,warehouse_id,device_id,product_id,allocated_quantity) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,device_id,product_id) DO UPDATE SET company_id=EXCLUDED.company_id,branch_id=EXCLUDED.branch_id,warehouse_id=EXCLUDED.warehouse_id,allocated_quantity=EXCLUDED.allocated_quantity,updated_at=now()`, []any{tenantID, companyID, branchID, warehouseID, deviceID, productTwoID, offlineProductAllocation}},
		{"period", `INSERT INTO fiscal_periods(id,tenant_id,company_id,starts_at,ends_at,is_open) VALUES($1,$2,$3,'2020-01-01T00:00:00Z','2100-01-01T00:00:00Z',true) ON CONFLICT(tenant_id,company_id,starts_at) DO UPDATE SET ends_at=EXCLUDED.ends_at,is_open=true`, []any{fiscalPeriodID, tenantID, companyID}},
		{"offline posting policy", `INSERT INTO offline_posting_policies(id,tenant_id,company_id,accounting_time_basis,maximum_future_skew_seconds,require_same_fiscal_period,effective_from,created_by,created_at) VALUES($1,$2,$3,'SERVER_RECEIPT',300,true,'2020-01-01T00:00:00Z',$4,'2020-01-01T00:00:00Z') ON CONFLICT(tenant_id,company_id,effective_from) DO NOTHING`, []any{offlinePostingPolicyID, tenantID, companyID, operatorID}},
		{"posting", `INSERT INTO sales_posting_config(tenant_id,company_id,receivable_account_id,tax_payable_account_id,cash_accounts) VALUES($1,$2,'receivable','tax-payable','{"CASH":"cash-on-hand","MOBILE_MONEY":"mobile-money-clearing","BANK_CARD":"bank-card-clearing","BANK_TRANSFER":"bank-current"}'::jsonb) ON CONFLICT(tenant_id,company_id) DO UPDATE SET receivable_account_id=EXCLUDED.receivable_account_id,tax_payable_account_id=EXCLUDED.tax_payable_account_id,cash_accounts=EXCLUDED.cash_accounts`, []any{tenantID, companyID}},
		{"procurement posting", `INSERT INTO procurement_posting_config(tenant_id,company_id,grni_account_id,payable_account_id,inventory_adjustment_account_id,stock_in_transit_account_id,cash_accounts) VALUES($1,$2,'grni','payable','inventory-adjustment','stock-in-transit','{"CASH":"cash-on-hand","MOBILE_MONEY":"mobile-money-clearing","BANK_CARD":"bank-card-clearing","BANK_TRANSFER":"bank-current"}'::jsonb) ON CONFLICT(tenant_id,company_id) DO UPDATE SET grni_account_id=EXCLUDED.grni_account_id,payable_account_id=EXCLUDED.payable_account_id,inventory_adjustment_account_id=EXCLUDED.inventory_adjustment_account_id,stock_in_transit_account_id=EXCLUDED.stock_in_transit_account_id,cash_accounts=EXCLUDED.cash_accounts`, []any{tenantID, companyID}},
		{"stock one", `INSERT INTO inventory_stock_ledger(id,tenant_id,company_id,branch_id,warehouse_id,product_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,'DEV_SEED',$7,100,'2020-01-01T00:00:00Z') ON CONFLICT(id) DO NOTHING`, []any{stockOneID, tenantID, companyID, branchID, warehouseID, productOneID, seedSourceID}},
		{"stock two", `INSERT INTO inventory_stock_ledger(id,tenant_id,company_id,branch_id,warehouse_id,product_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,'DEV_SEED',$7,100,'2020-01-01T00:00:00Z') ON CONFLICT(id) DO NOTHING`, []any{stockTwoID, tenantID, companyID, branchID, warehouseID, productTwoID, seedSourceID}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.sql, statement.args...); err != nil {
			return fmt.Errorf("seed %s: %w", statement.name, err)
		}
	}
	return verifySeed(ctx, tx)
}

func verifySeed(ctx context.Context, tx pgx.Tx) error {
	var status string
	var offlineEnabled bool
	var transactionLimit, dailyLimit, installedMaster, installedPrice, availableMaster, availablePrice int64
	if err := tx.QueryRow(ctx, `
		SELECT d.status, d.offline_enabled, d.offline_transaction_limit_minor, d.offline_daily_limit_minor,
		       d.master_data_version, d.price_version, c.master_data_version, c.price_version
		FROM mobile_devices d
		JOIN legal_companies c ON c.tenant_id=d.tenant_id AND c.id=d.company_id
		WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.branch_id=$3 AND d.warehouse_id=$4
		  AND d.actor_id=$5 AND d.id=$6`, tenantID, companyID, branchID, warehouseID, operatorID, deviceID).Scan(
		&status, &offlineEnabled, &transactionLimit, &dailyLimit, &installedMaster, &installedPrice, &availableMaster, &availablePrice); err != nil {
		return fmt.Errorf("verify offline device: %w", err)
	}
	if status != "ACTIVE" || !offlineEnabled || transactionLimit != offlineTransactionLimitMinor || dailyLimit != offlineDailyLimitMinor || installedMaster != availableMaster || installedPrice != availablePrice {
		return errors.New("verify offline device: policy does not match deterministic development fixture")
	}
	var allocationCount int
	var minimumAllocation int64
	if err := tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(min(allocated_quantity),0)::bigint
		FROM mobile_device_stock_allocations
		WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4
		  AND device_id=$5 AND product_id IN ($6,$7)`,
		tenantID, companyID, branchID, warehouseID, deviceID, productOneID, productTwoID).Scan(
		&allocationCount, &minimumAllocation); err != nil {
		return fmt.Errorf("verify offline allocations: %w", err)
	}
	if allocationCount != 2 || minimumAllocation != offlineProductAllocation {
		return errors.New("verify offline allocations: both development products require the configured positive allocation")
	}
	var encodedAccounts []byte
	if err := tx.QueryRow(ctx, `SELECT cash_accounts FROM sales_posting_config WHERE tenant_id=$1 AND company_id=$2`, tenantID, companyID).Scan(&encodedAccounts); err != nil {
		return fmt.Errorf("verify payment posting accounts: %w", err)
	}
	var accounts map[string]string
	if err := json.Unmarshal(encodedAccounts, &accounts); err != nil {
		return fmt.Errorf("verify payment posting accounts: %w", err)
	}
	canonicalMethods := []string{"CASH", "MOBILE_MONEY", "BANK_CARD", "BANK_TRANSFER"}
	uniqueAccounts := make(map[string]struct{}, len(canonicalMethods))
	for _, method := range canonicalMethods {
		account := strings.TrimSpace(accounts[method])
		if account == "" {
			return fmt.Errorf("verify payment posting accounts: %s is not configured", method)
		}
		if _, duplicate := uniqueAccounts[account]; duplicate {
			return fmt.Errorf("verify payment posting accounts: %s does not use a distinct account", method)
		}
		uniqueAccounts[account] = struct{}{}
	}
	return nil
}
