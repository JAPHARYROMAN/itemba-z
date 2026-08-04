package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (t *transaction) Authorize(ctx context.Context, scope tenancy.Scope, actorID, permission string) (bool, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return false, err
	}
	var authorized bool
	err := t.tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN user_role_scopes urs ON urs.tenant_id = u.tenant_id AND urs.user_id = u.id
			JOIN role_permissions rp ON rp.tenant_id = urs.tenant_id AND rp.role_id = urs.role_id
			WHERE u.tenant_id = $1 AND u.id = $2 AND u.active
			  AND urs.company_id = $3 AND urs.branch_id = $4 AND urs.warehouse_id = $5
			  AND rp.permission_code = $6
		)`, scope.TenantID, actorID, scope.CompanyID, scope.BranchID, scope.WarehouseID, permission).Scan(&authorized)
	return authorized, normalizeError(err)
}

func (t *transaction) ClaimIdempotency(ctx context.Context, scope tenancy.Scope, operation, key, requestHash string) (bool, string, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return false, "", err
	}
	command, err := t.tx.Exec(ctx, `
		INSERT INTO idempotency_keys (tenant_id, company_id, operation, idempotency_key, request_hash)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, company_id, operation, idempotency_key) DO NOTHING`,
		scope.TenantID, scope.CompanyID, operation, key, requestHash)
	if err != nil {
		return false, "", normalizeError(err)
	}
	if command.RowsAffected() == 1 {
		return true, "", nil
	}
	var storedHash string
	var resultID pgtype.UUID
	err = t.tx.QueryRow(ctx, `
		SELECT request_hash, result_id
		FROM idempotency_keys
		WHERE tenant_id = $1 AND company_id = $2 AND operation = $3 AND idempotency_key = $4`,
		scope.TenantID, scope.CompanyID, operation, key).Scan(&storedHash, &resultID)
	if err != nil {
		return false, "", normalizeError(err)
	}
	if storedHash != requestHash {
		return false, "", sales.ErrIdempotencyConflict
	}
	if !resultID.Valid {
		return false, "", errors.New("idempotency key has no committed result")
	}
	return false, resultID.String(), nil
}

func (t *transaction) CompleteIdempotency(ctx context.Context, scope tenancy.Scope, operation, key, resultID string) error {
	if err := t.ensureScope(ctx, scope); err != nil {
		return err
	}
	command, err := t.tx.Exec(ctx, `
		UPDATE idempotency_keys SET result_id = $5, completed_at = now()
		WHERE tenant_id = $1 AND company_id = $2 AND operation = $3 AND idempotency_key = $4 AND result_id IS NULL`,
		scope.TenantID, scope.CompanyID, operation, key, resultID)
	if err != nil {
		return normalizeError(err)
	}
	if command.RowsAffected() != 1 {
		return errors.New("invalid idempotency completion")
	}
	return nil
}

func (t *transaction) Customer(ctx context.Context, scope tenancy.Scope, customerID string) (customers.Account, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return customers.Account{}, err
	}
	var value customers.Account
	err := t.tx.QueryRow(ctx, `
		SELECT id, code, tenant_id, company_id, name, active, is_general, credit_enabled, credit_limit_minor
		FROM customer_accounts WHERE tenant_id = $1 AND company_id = $2 AND id = $3`,
		scope.TenantID, scope.CompanyID, customerID).Scan(
		&value.ID, &value.Code, &value.TenantID, &value.CompanyID, &value.Name, &value.Active,
		&value.General, &value.CreditEnabled, &value.CreditLimitMinor)
	return value, normalizeError(err)
}

func (t *transaction) LockCustomerCredit(ctx context.Context, scope tenancy.Scope, customerID string) error {
	if err := t.ensureScope(ctx, scope); err != nil {
		return err
	}
	lockKey := scope.TenantID + ":" + scope.CompanyID + ":customer-credit:" + customerID
	if _, err := t.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return normalizeError(err)
	}
	var exists bool
	if err := t.tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM customer_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3
	)`, scope.TenantID, scope.CompanyID, customerID).Scan(&exists); err != nil {
		return normalizeError(err)
	}
	if !exists {
		return sales.ErrNotFound
	}
	return nil
}

func (t *transaction) Product(ctx context.Context, scope tenancy.Scope, productID string) (catalog.Product, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return catalog.Product{}, err
	}
	var value catalog.Product
	err := t.tx.QueryRow(ctx, `
		SELECT id, tenant_id, company_id, sku, name, base_unit_code, active, currency, list_price_minor,
		       standard_cost_minor, tax_code, revenue_account_id, cogs_account_id, inventory_account_id,
		       price_version, master_data_version
		FROM products WHERE tenant_id = $1 AND company_id = $2 AND id = $3`,
		scope.TenantID, scope.CompanyID, productID).Scan(
		&value.ID, &value.TenantID, &value.CompanyID, &value.SKU, &value.Name, &value.BaseUnitCode, &value.Active,
		&value.Currency, &value.ListPriceMinor, &value.StandardCostMinor, &value.TaxCode,
		&value.RevenueAccountID, &value.COGSAccountID, &value.InventoryAccountID,
		&value.PriceVersion, &value.MasterDataVersion)
	return value, normalizeError(err)
}

func (t *transaction) TaxRateBasisPoints(ctx context.Context, scope tenancy.Scope, taxCode string, at time.Time) (int64, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return 0, err
	}
	var rate int64
	err := t.tx.QueryRow(ctx, `
		SELECT basis_points FROM tax_rules
		WHERE tenant_id = $1 AND company_id = $2 AND code = $3
		  AND effective_from <= $4 AND (effective_to IS NULL OR effective_to > $4)
		ORDER BY effective_from DESC LIMIT 1`, scope.TenantID, scope.CompanyID, taxCode, at).Scan(&rate)
	return rate, normalizeError(err)
}

func (t *transaction) AvailableStock(ctx context.Context, scope tenancy.Scope, productID string) (int64, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return 0, err
	}
	lockKey := scope.TenantID + ":" + scope.CompanyID + ":" + scope.WarehouseID + ":" + productID
	if _, err := t.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return 0, normalizeError(err)
	}
	var quantity int64
	err := t.tx.QueryRow(ctx, `
		SELECT COALESCE(sum(quantity), 0)::bigint FROM inventory_stock_ledger
		WHERE tenant_id = $1 AND company_id = $2 AND branch_id = $3 AND warehouse_id = $4 AND product_id = $5`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, productID).Scan(&quantity)
	return quantity, normalizeError(err)
}

func (t *transaction) CreditExposure(ctx context.Context, scope tenancy.Scope, customerID string) (int64, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return 0, err
	}
	var amount int64
	err := t.tx.QueryRow(ctx, `
		SELECT COALESCE(sum(amount_minor), 0)::bigint FROM customer_ledger
		WHERE tenant_id = $1 AND company_id = $2 AND customer_id = $3`,
		scope.TenantID, scope.CompanyID, customerID).Scan(&amount)
	return amount, normalizeError(err)
}

func (t *transaction) FiscalPeriodOpen(ctx context.Context, scope tenancy.Scope, at time.Time) (bool, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return false, err
	}
	var open bool
	err := t.tx.QueryRow(ctx, `
		SELECT is_open FROM fiscal_periods
		WHERE tenant_id = $1 AND company_id = $2 AND starts_at <= $3 AND ends_at > $3
		ORDER BY starts_at DESC LIMIT 1`, scope.TenantID, scope.CompanyID, at).Scan(&open)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return open, normalizeError(err)
}

func (t *transaction) SalesPostingConfig(ctx context.Context, scope tenancy.Scope) (finance.SalesPostingConfig, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return finance.SalesPostingConfig{}, err
	}
	var value finance.SalesPostingConfig
	var cashJSON []byte
	err := t.tx.QueryRow(ctx, `
		SELECT receivable_account_id, tax_payable_account_id, cash_accounts
		FROM sales_posting_config WHERE tenant_id = $1 AND company_id = $2`,
		scope.TenantID, scope.CompanyID).Scan(&value.ReceivableAccountID, &value.TaxPayableAccountID, &cashJSON)
	if err != nil {
		return finance.SalesPostingConfig{}, normalizeError(err)
	}
	if err := json.Unmarshal(cashJSON, &value.CashAccounts); err != nil {
		return finance.SalesPostingConfig{}, fmt.Errorf("decode cash account mapping: %w", err)
	}
	return value, nil
}

func (t *transaction) Sale(ctx context.Context, scope tenancy.Scope, saleID string) (sales.Sale, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return sales.Sale{}, err
	}
	var value sales.Sale
	var paymentMethod, reversalReason pgtype.Text
	var reversalOf pgtype.UUID
	var deviceID, clientTransactionID pgtype.UUID
	var catalogSnapshotToken pgtype.UUID
	var clientTimestamp, reversedAt pgtype.Timestamptz
	var clientAppVersion pgtype.Text
	var clientMasterDataVersion, clientPriceVersion pgtype.Int8
	err := t.tx.QueryRow(ctx, `
		SELECT id, tenant_id, company_id, branch_id, warehouse_id, record_type, sale_kind, status,
		       customer_id, currency, subtotal_minor, tax_minor, total_minor, cogs_minor,
		       payment_method, reversal_of, reversal_reason, device_id, client_transaction_id,
		       client_timestamp, client_app_version, client_master_data_version, client_price_version, client_catalog_snapshot_token, offline,
		       receipt_reference, fiscal_status, created_by, correlation_id, created_at, reversed_at
		FROM sales WHERE tenant_id = $1 AND company_id = $2 AND branch_id = $3 AND warehouse_id = $4 AND id = $5 FOR UPDATE`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, saleID).Scan(
		&value.ID, &value.Scope.TenantID, &value.Scope.CompanyID, &value.Scope.BranchID, &value.Scope.WarehouseID,
		&value.RecordType, &value.Kind, &value.Status, &value.CustomerID, &value.Currency,
		&value.SubtotalMinor, &value.TaxMinor, &value.TotalMinor, &value.COGSMinor,
		&paymentMethod, &reversalOf, &reversalReason, &deviceID, &clientTransactionID,
		&clientTimestamp, &clientAppVersion, &clientMasterDataVersion, &clientPriceVersion, &catalogSnapshotToken, &value.Offline,
		&value.ReceiptReference, &value.FiscalStatus, &value.CreatedBy, &value.CorrelationID, &value.CreatedAt, &reversedAt)
	if err != nil {
		return sales.Sale{}, normalizeError(err)
	}
	if paymentMethod.Valid {
		value.PaymentMethod = paymentMethod.String
	}
	if reversalOf.Valid {
		value.ReversalOf = reversalOf.String()
	}
	if reversalReason.Valid {
		value.ReversalReason = reversalReason.String
	}
	if deviceID.Valid {
		value.DeviceID = deviceID.String()
	}
	if clientTransactionID.Valid {
		value.ClientTransactionID = clientTransactionID.String()
	}
	if clientTimestamp.Valid {
		parsed := clientTimestamp.Time
		value.ClientTimestamp = &parsed
	}
	if clientAppVersion.Valid {
		value.AppVersion = clientAppVersion.String
	}
	if clientMasterDataVersion.Valid {
		value.MasterDataVersion = clientMasterDataVersion.Int64
	}
	if clientPriceVersion.Valid {
		value.PriceVersion = clientPriceVersion.Int64
	}
	if catalogSnapshotToken.Valid {
		value.CatalogSnapshotToken = catalogSnapshotToken.String()
	}
	if reversedAt.Valid {
		parsed := reversedAt.Time
		value.ReversedAt = &parsed
	}
	rows, err := t.tx.Query(ctx, `
		SELECT id, product_id, quantity, unit_price_minor, subtotal_minor, tax_minor,
		       total_minor, unit_cost_minor, cogs_minor
		FROM sale_lines WHERE tenant_id = $1 AND company_id = $2 AND sale_id = $3 ORDER BY id`,
		scope.TenantID, scope.CompanyID, saleID)
	if err != nil {
		return sales.Sale{}, normalizeError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var line sales.Line
		if err := rows.Scan(&line.ID, &line.ProductID, &line.Quantity, &line.UnitPriceMinor, &line.SubtotalMinor, &line.TaxMinor, &line.TotalMinor, &line.UnitCostMinor, &line.COGSMinor); err != nil {
			return sales.Sale{}, normalizeError(err)
		}
		value.Lines = append(value.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return sales.Sale{}, normalizeError(err)
	}
	return value, nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullablePositiveInt64(value int64) any {
	if value < 1 {
		return nil
	}
	return value
}

func (t *transaction) CreateSale(ctx context.Context, value sales.Sale) error {
	if err := t.ensureScope(ctx, value.Scope); err != nil {
		return err
	}
	_, err := t.tx.Exec(ctx, `
		INSERT INTO sales (
			id, tenant_id, company_id, branch_id, warehouse_id, record_type, sale_kind, status,
			customer_id, currency, subtotal_minor, tax_minor, total_minor, cogs_minor,
			payment_method, reversal_of, reversal_reason, device_id, client_transaction_id,
			client_timestamp, client_app_version, client_master_data_version, client_price_version, client_catalog_snapshot_token, offline,
			receipt_reference, fiscal_status, created_by, correlation_id, created_at, reversed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31)`,
		value.ID, value.Scope.TenantID, value.Scope.CompanyID, value.Scope.BranchID, value.Scope.WarehouseID,
		value.RecordType, value.Kind, value.Status, value.CustomerID, value.Currency,
		value.SubtotalMinor, value.TaxMinor, value.TotalMinor, value.COGSMinor,
		nullableText(value.PaymentMethod), nullableText(value.ReversalOf), nullableText(value.ReversalReason),
		nullableText(value.DeviceID), nullableText(value.ClientTransactionID), nullableTime(value.ClientTimestamp),
		nullableText(value.AppVersion), nullablePositiveInt64(value.MasterDataVersion), nullablePositiveInt64(value.PriceVersion),
		nullableText(value.CatalogSnapshotToken), value.Offline, value.ReceiptReference, value.FiscalStatus, value.CreatedBy, value.CorrelationID, value.CreatedAt, value.ReversedAt)
	if err != nil {
		return normalizeError(err)
	}
	for _, line := range value.Lines {
		_, err := t.tx.Exec(ctx, `
			INSERT INTO sale_lines (
				id, tenant_id, company_id, sale_id, product_id, quantity, unit_price_minor,
				subtotal_minor, tax_minor, total_minor, unit_cost_minor, cogs_minor
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			line.ID, value.Scope.TenantID, value.Scope.CompanyID, value.ID, line.ProductID,
			line.Quantity, line.UnitPriceMinor, line.SubtotalMinor, line.TaxMinor,
			line.TotalMinor, line.UnitCostMinor, line.COGSMinor)
		if err != nil {
			return normalizeError(err)
		}
	}
	return nil
}

func (t *transaction) MarkSaleReversed(ctx context.Context, scope tenancy.Scope, saleID string, reversedAt time.Time) error {
	if err := t.ensureScope(ctx, scope); err != nil {
		return err
	}
	command, err := t.tx.Exec(ctx, `UPDATE sales SET status = 'REVERSED', reversed_at = $6
		WHERE tenant_id = $1 AND company_id = $2 AND branch_id = $3 AND warehouse_id = $4 AND id = $5 AND status = 'POSTED'`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, saleID, reversedAt)
	if err != nil {
		return normalizeError(err)
	}
	if command.RowsAffected() != 1 {
		return sales.ErrAlreadyReversed
	}
	return nil
}

func (t *transaction) AppendStockMovement(ctx context.Context, value inventory.Movement) error {
	if t.tenantID == "" {
		return errors.New("stock movement requires an established sale scope")
	}
	if t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO inventory_stock_ledger (
		id, tenant_id, company_id, branch_id, warehouse_id, product_id, source_type, source_id, quantity, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.ID, value.TenantID, value.CompanyID,
		value.BranchID, value.WarehouseID, value.ProductID, value.SourceType, value.SourceID, value.Quantity, value.OccurredAt)
	return normalizeError(err)
}

func (t *transaction) StockMovementsBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]inventory.Movement, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return nil, err
	}
	if err := t.ensureSaleSourceScope(ctx, scope, sourceType, sourceID); err != nil {
		return nil, err
	}
	rows, err := t.tx.Query(ctx, `SELECT id, tenant_id, company_id, branch_id, warehouse_id,
		product_id, source_type, source_id, quantity, occurred_at FROM inventory_stock_ledger
		WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND source_type=$5 AND source_id=$6 ORDER BY id`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, sourceType, sourceID)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	var result []inventory.Movement
	for rows.Next() {
		var value inventory.Movement
		if err := rows.Scan(&value.ID, &value.TenantID, &value.CompanyID, &value.BranchID, &value.WarehouseID, &value.ProductID, &value.SourceType, &value.SourceID, &value.Quantity, &value.OccurredAt); err != nil {
			return nil, normalizeError(err)
		}
		result = append(result, value)
	}
	return result, normalizeError(rows.Err())
}

func (t *transaction) AppendCustomerLedgerEntry(ctx context.Context, value customers.LedgerEntry) error {
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO customer_ledger (
		id, tenant_id, company_id, customer_id, source_type, source_id, amount_minor, currency, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.ID, value.TenantID, value.CompanyID,
		value.CustomerID, value.SourceType, value.SourceID, value.AmountMinor, value.Currency, value.OccurredAt)
	return normalizeError(err)
}

func (t *transaction) CustomerLedgerBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]customers.LedgerEntry, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return nil, err
	}
	if err := t.ensureSaleSourceScope(ctx, scope, sourceType, sourceID); err != nil {
		return nil, err
	}
	rows, err := t.tx.Query(ctx, `SELECT id, tenant_id, company_id, customer_id, source_type,
		source_id, amount_minor, currency, occurred_at FROM customer_ledger
		WHERE tenant_id=$1 AND company_id=$2 AND source_type=$3 AND source_id=$4 ORDER BY id`,
		scope.TenantID, scope.CompanyID, sourceType, sourceID)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	var result []customers.LedgerEntry
	for rows.Next() {
		var value customers.LedgerEntry
		if err := rows.Scan(&value.ID, &value.TenantID, &value.CompanyID, &value.CustomerID, &value.SourceType, &value.SourceID, &value.AmountMinor, &value.Currency, &value.OccurredAt); err != nil {
			return nil, normalizeError(err)
		}
		result = append(result, value)
	}
	return result, normalizeError(rows.Err())
}

func (t *transaction) CreatePayment(ctx context.Context, value sales.Payment) error {
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO payments (
		id, tenant_id, company_id, sale_id, account_id, method, amount_minor, currency, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, value.ID, value.TenantID, value.CompanyID,
		value.SaleID, value.AccountID, value.Method, value.AmountMinor, value.Currency, value.OccurredAt)
	return normalizeError(err)
}

func (t *transaction) PaymentsBySale(ctx context.Context, scope tenancy.Scope, saleID string) ([]sales.Payment, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return nil, err
	}
	if err := t.ensureSaleSourceScope(ctx, scope, string(sales.RecordSale), saleID); err != nil {
		return nil, err
	}
	rows, err := t.tx.Query(ctx, `SELECT id, tenant_id, company_id, sale_id, account_id, method,
		amount_minor, currency, occurred_at FROM payments
		WHERE tenant_id=$1 AND company_id=$2 AND sale_id=$3 ORDER BY id`, scope.TenantID, scope.CompanyID, saleID)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	var result []sales.Payment
	for rows.Next() {
		var value sales.Payment
		if err := rows.Scan(&value.ID, &value.TenantID, &value.CompanyID, &value.SaleID, &value.AccountID, &value.Method, &value.AmountMinor, &value.Currency, &value.OccurredAt); err != nil {
			return nil, normalizeError(err)
		}
		result = append(result, value)
	}
	return result, normalizeError(rows.Err())
}

func (t *transaction) CreateJournal(ctx context.Context, value finance.Journal) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO journals (
		id, tenant_id, company_id, source_type, source_id, currency, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`, value.ID, value.TenantID, value.CompanyID,
		value.SourceType, value.SourceID, value.Currency, value.OccurredAt)
	if err != nil {
		return normalizeError(err)
	}
	for _, entry := range value.Entries {
		_, err := t.tx.Exec(ctx, `INSERT INTO journal_lines (
			journal_id, tenant_id, company_id, account_id, debit_minor, credit_minor, memo
			) VALUES ($1,$2,$3,$4,$5,$6,$7)`, value.ID, value.TenantID, value.CompanyID,
			entry.AccountID, entry.DebitMinor, entry.CreditMinor, entry.Memo)
		if err != nil {
			return normalizeError(err)
		}
	}
	_, err = t.tx.Exec(ctx, `SELECT enforce_balanced_journal($1)`, value.ID)
	return normalizeError(err)
}

func (t *transaction) JournalBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) (finance.Journal, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return finance.Journal{}, err
	}
	if err := t.ensureSaleSourceScope(ctx, scope, sourceType, sourceID); err != nil {
		return finance.Journal{}, err
	}
	var value finance.Journal
	err := t.tx.QueryRow(ctx, `SELECT id, tenant_id, company_id, source_type, source_id, currency,
		occurred_at FROM journals WHERE tenant_id=$1 AND company_id=$2 AND source_type=$3 AND source_id=$4`,
		scope.TenantID, scope.CompanyID, sourceType, sourceID).Scan(&value.ID, &value.TenantID, &value.CompanyID,
		&value.SourceType, &value.SourceID, &value.Currency, &value.OccurredAt)
	if err != nil {
		return finance.Journal{}, normalizeError(err)
	}
	rows, err := t.tx.Query(ctx, `SELECT account_id, debit_minor, credit_minor, memo FROM journal_lines
		WHERE tenant_id=$1 AND company_id=$2 AND journal_id=$3 ORDER BY id`, scope.TenantID, scope.CompanyID, value.ID)
	if err != nil {
		return finance.Journal{}, normalizeError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry finance.JournalEntry
		if err := rows.Scan(&entry.AccountID, &entry.DebitMinor, &entry.CreditMinor, &entry.Memo); err != nil {
			return finance.Journal{}, normalizeError(err)
		}
		value.Entries = append(value.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return finance.Journal{}, normalizeError(err)
	}
	return value, nil
}

func (t *transaction) ensureSaleSourceScope(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) error {
	if sourceType != string(sales.RecordSale) {
		return sales.ErrNotFound
	}
	var exists bool
	if err := t.tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sales
			WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4
			  AND id=$5 AND record_type='SALE'
		)`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, sourceID).Scan(&exists); err != nil {
		return normalizeError(err)
	}
	if !exists {
		return sales.ErrNotFound
	}
	return nil
}

func (t *transaction) AppendAuditEvent(ctx context.Context, value audit.Event) error {
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO audit_events (
		id, tenant_id, company_id, actor_id, action, entity_type, entity_id, correlation_id, causation_id, data, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.ID, value.TenantID, value.CompanyID,
		value.ActorID, value.Action, value.EntityType, value.EntityID, value.CorrelationID, value.CausationID, []byte(value.Data), value.OccurredAt)
	return normalizeError(err)
}

func (t *transaction) AppendOutboxEvent(ctx context.Context, value outbox.Event) error {
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO outbox_events (
		id, tenant_id, company_id, aggregate_type, aggregate_id, event_type, version, correlation_id, causation_id, payload, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, value.ID, value.TenantID, value.CompanyID,
		value.AggregateType, value.AggregateID, value.EventType, value.Version, value.CorrelationID, value.CausationID, []byte(value.Payload), value.OccurredAt)
	return normalizeError(err)
}
