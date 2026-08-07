package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ReceiveCustomerCollection(ctx context.Context, collection receivables.Collection, ids receivables.CollectionEffectIDs, event audit.Event, message outbox.Event, requestHash string) (receivables.Collection, error) {
	var result receivables.Collection
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, collection.Scope, collection.CreatedBy, "customers.collections.post")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, collection.Scope, "customers.collection.receive.v1", collection.IdempotencyKey, requestHash)
		if err != nil {
			return err
		}
		if !acquired {
			err = tx.tx.QueryRow(ctx, `SELECT id::text,customer_id::text,invoice_sale_id::text,account_id,method,amount_minor,currency,occurred_at,created_by::text,correlation_id::text FROM customer_collections WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, collection.Scope.TenantID, collection.Scope.CompanyID, resultID).Scan(&result.ID, &result.CustomerID, &result.InvoiceSaleID, &result.AccountID, &result.Method, &result.AmountMinor, &result.Currency, &result.OccurredAt, &result.CreatedBy, &result.CorrelationID)
			result.Scope = collection.Scope
			return normalizeError(err)
		}
		var active bool
		if err := tx.tx.QueryRow(ctx, `SELECT active FROM customer_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, collection.Scope.TenantID, collection.Scope.CompanyID, collection.CustomerID).Scan(&active); err != nil {
			return normalizeError(err)
		}
		if !active {
			return sales.ErrCustomerInactive
		}
		var invoiceItemID, currency string
		var invoiceAmount, allocated int64
		err = tx.tx.QueryRow(ctx, `SELECT id::text,amount_minor,currency FROM customer_receivable_items WHERE tenant_id=$1 AND company_id=$2 AND customer_id=$3 AND kind='INVOICE' AND source_type='SALE' AND source_id=$4 FOR UPDATE`, collection.Scope.TenantID, collection.Scope.CompanyID, collection.CustomerID, collection.InvoiceSaleID).Scan(&invoiceItemID, &invoiceAmount, &currency)
		if errors.Is(err, pgx.ErrNoRows) {
			return sales.ErrNotFound
		}
		if err != nil {
			return normalizeError(err)
		}
		if currency != collection.Currency {
			return customers.ErrReceivablesUnbalanced
		}
		if err := tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0)::bigint FROM customer_receivable_allocations WHERE tenant_id=$1 AND company_id=$2 AND customer_id=$3 AND debit_item_id=$4`, collection.Scope.TenantID, collection.Scope.CompanyID, collection.CustomerID, invoiceItemID).Scan(&allocated); err != nil {
			return normalizeError(err)
		}
		if collection.AmountMinor > invoiceAmount-allocated {
			return customers.ErrReceivablesUnbalanced
		}
		posting, err := tx.SalesPostingConfig(ctx, collection.Scope)
		if err != nil {
			return err
		}
		accountID := posting.CashAccounts[collection.Method]
		if accountID == "" {
			return sales.ErrUnsupportedPayment
		}
		collection.AccountID = accountID
		payload, _ := json.Marshal(collection)
		event.Data, message.Payload = payload, payload
		if err := tx.AppendCustomerLedgerEntry(ctx, customers.LedgerEntry{ID: ids.LedgerID, TenantID: collection.Scope.TenantID, CompanyID: collection.Scope.CompanyID, CustomerID: collection.CustomerID, SourceType: "CUSTOMER_COLLECTION", SourceID: collection.ID, AmountMinor: -collection.AmountMinor, Currency: collection.Currency, OccurredAt: collection.OccurredAt}); err != nil {
			return err
		}
		if err := tx.AppendReceivableItem(ctx, customers.ReceivableItem{ID: ids.ItemID, TenantID: collection.Scope.TenantID, CompanyID: collection.Scope.CompanyID, CustomerID: collection.CustomerID, Kind: customers.ReceivableReceipt, SourceType: "CUSTOMER_COLLECTION", SourceID: collection.ID, AmountMinor: collection.AmountMinor, Currency: collection.Currency, DocumentAt: collection.OccurredAt, OccurredAt: collection.OccurredAt}); err != nil {
			return err
		}
		if err := tx.AppendReceivableAllocation(ctx, customers.ReceivableAllocation{ID: ids.AllocationID, TenantID: collection.Scope.TenantID, CompanyID: collection.Scope.CompanyID, CustomerID: collection.CustomerID, DebitItemID: invoiceItemID, CreditItemID: ids.ItemID, AmountMinor: collection.AmountMinor, OccurredAt: collection.OccurredAt}); err != nil {
			return err
		}
		if _, err := tx.tx.Exec(ctx, `INSERT INTO customer_collections(id,tenant_id,company_id,customer_id,invoice_sale_id,account_id,method,amount_minor,currency,occurred_at,created_by,correlation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, collection.ID, collection.Scope.TenantID, collection.Scope.CompanyID, collection.CustomerID, collection.InvoiceSaleID, accountID, collection.Method, collection.AmountMinor, collection.Currency, collection.OccurredAt, collection.CreatedBy, collection.CorrelationID); err != nil {
			return normalizeError(err)
		}
		if err := tx.CreateJournal(ctx, finance.Journal{ID: ids.JournalID, TenantID: collection.Scope.TenantID, CompanyID: collection.Scope.CompanyID, SourceType: "CUSTOMER_COLLECTION", SourceID: collection.ID, Currency: collection.Currency, OccurredAt: collection.OccurredAt, Entries: []finance.JournalEntry{{AccountID: accountID, DebitMinor: collection.AmountMinor, Memo: "Customer collection"}, {AccountID: posting.ReceivableAccountID, CreditMinor: collection.AmountMinor, Memo: "Trade receivable settled"}}}); err != nil {
			return err
		}
		if err := tx.AppendAuditEvent(ctx, event); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, message); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, collection.Scope, "customers.collection.receive.v1", collection.IdempotencyKey, collection.ID); err != nil {
			return err
		}
		result = collection
		return nil
	})
	return result, err
}
