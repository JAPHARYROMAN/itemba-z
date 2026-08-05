package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (t *transaction) CustomerCreditPolicy(ctx context.Context, scope tenancy.Scope, customerID string, at time.Time) (customers.CreditPolicy, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return customers.CreditPolicy{}, err
	}
	var value customers.CreditPolicy
	value.Scope = scope
	err := t.tx.QueryRow(ctx, `
		SELECT id,customer_id,credit_enabled,credit_limit_minor,payment_terms_days,max_overdue_days,
		       risk_status,reason,effective_from,COALESCE(approved_by::text,'SYSTEM'),created_at,correlation_id
		FROM customer_credit_policies
		WHERE tenant_id=$1 AND company_id=$2 AND customer_id=$3 AND effective_from <= $4
		ORDER BY effective_from DESC LIMIT 1`, scope.TenantID, scope.CompanyID, customerID, at).Scan(
		&value.ID, &value.CustomerID, &value.CreditEnabled, &value.CreditLimitMinor,
		&value.PaymentTermsDays, &value.MaxOverdueDays, &value.RiskStatus, &value.Reason,
		&value.EffectiveFrom, &value.ApprovedBy, &value.CreatedAt, &value.CorrelationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return customers.CreditPolicy{}, customers.ErrCreditPolicyMissing
	}
	return value, normalizeError(err)
}

func (t *transaction) CustomerReceivableAging(ctx context.Context, scope tenancy.Scope, customerID string, at time.Time) (customers.ReceivableAging, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return customers.ReceivableAging{}, err
	}
	var result customers.ReceivableAging
	if err := t.tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0)::bigint FROM customer_ledger
		WHERE tenant_id=$1 AND company_id=$2 AND customer_id=$3`, scope.TenantID, scope.CompanyID, customerID).Scan(&result.LedgerBalanceMinor); err != nil {
		return customers.ReceivableAging{}, normalizeError(err)
	}
	items, err := t.customerReceivableItems(ctx, scope, customerID, false)
	if err != nil {
		return customers.ReceivableAging{}, err
	}
	var zoneName string
	if err := t.tx.QueryRow(ctx, `SELECT business_timezone FROM legal_companies WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.CompanyID).Scan(&zoneName); err != nil {
		return customers.ReceivableAging{}, normalizeError(err)
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return customers.ReceivableAging{}, err
	}
	today := localDate(at, zone)
	for _, item := range items {
		if item.Kind != customers.ReceivableInvoice {
			result.UnappliedCreditMinor += item.OutstandingMinor
			continue
		}
		result.OpenInvoiceMinor += item.OutstandingMinor
		if item.DueAt == nil || item.OutstandingMinor == 0 {
			result.Buckets.CurrentMinor += item.OutstandingMinor
			continue
		}
		dueDate := localDate(*item.DueAt, zone)
		days := int64(today.Sub(dueDate) / (24 * time.Hour))
		if days <= 0 {
			result.Buckets.CurrentMinor += item.OutstandingMinor
			continue
		}
		result.OverdueMinor += item.OutstandingMinor
		if days > result.OldestOverdueDays {
			result.OldestOverdueDays = days
		}
		switch {
		case days <= 30:
			result.Buckets.Days1To30 += item.OutstandingMinor
		case days <= 60:
			result.Buckets.Days31To60 += item.OutstandingMinor
		case days <= 90:
			result.Buckets.Days61To90 += item.OutstandingMinor
		default:
			result.Buckets.DaysOver90 += item.OutstandingMinor
		}
	}
	result.CalculatedExposure = result.OpenInvoiceMinor - result.UnappliedCreditMinor
	result.Reconciled = result.CalculatedExposure == result.LedgerBalanceMinor
	return result, nil
}

func localDate(value time.Time, zone *time.Location) time.Time {
	local := value.In(zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func (t *transaction) AppendReceivableItem(ctx context.Context, value customers.ReceivableItem) error {
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO customer_receivable_items (
		id,tenant_id,company_id,customer_id,kind,source_type,source_id,amount_minor,currency,document_at,due_at,occurred_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, value.ID, value.TenantID, value.CompanyID,
		value.CustomerID, value.Kind, value.SourceType, value.SourceID, value.AmountMinor, value.Currency,
		value.DocumentAt, value.DueAt, value.OccurredAt)
	return normalizeError(err)
}

func (t *transaction) ReceivableItemBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) (customers.ReceivableItem, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return customers.ReceivableItem{}, err
	}
	items, err := t.customerReceivableItemsBySource(ctx, scope, sourceType, sourceID)
	if err != nil {
		return customers.ReceivableItem{}, err
	}
	if len(items) != 1 {
		return customers.ReceivableItem{}, sales.ErrNotFound
	}
	return items[0], nil
}

func (t *transaction) AppendReceivableAllocation(ctx context.Context, value customers.ReceivableAllocation) error {
	if t.tenantID == "" || t.tenantID != value.TenantID {
		return sales.ErrForbidden
	}
	_, err := t.tx.Exec(ctx, `INSERT INTO customer_receivable_allocations (
		id,tenant_id,company_id,customer_id,debit_item_id,credit_item_id,amount_minor,occurred_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, value.ID, value.TenantID, value.CompanyID, value.CustomerID,
		value.DebitItemID, value.CreditItemID, value.AmountMinor, value.OccurredAt)
	return normalizeError(err)
}

func (t *transaction) customerReceivableItems(ctx context.Context, scope tenancy.Scope, customerID string, onlyOpen bool) ([]customers.ReceivableItem, error) {
	query := `SELECT item.id,item.tenant_id,item.company_id,item.customer_id,item.kind,item.source_type,item.source_id,
		item.amount_minor,item.amount_minor-COALESCE(allocated.amount_minor,0),item.currency,item.document_at,item.due_at,item.occurred_at
	FROM customer_receivable_items item
	LEFT JOIN LATERAL (
		SELECT sum(a.amount_minor)::bigint AS amount_minor FROM customer_receivable_allocations a
		WHERE a.tenant_id=item.tenant_id AND a.company_id=item.company_id AND
		      (CASE WHEN item.kind='INVOICE' THEN a.debit_item_id=item.id ELSE a.credit_item_id=item.id END)
	) allocated ON true
	WHERE item.tenant_id=$1 AND item.company_id=$2 AND item.customer_id=$3`
	if onlyOpen {
		query += ` AND item.amount_minor-COALESCE(allocated.amount_minor,0) > 0`
	}
	query += ` ORDER BY item.document_at DESC,item.id`
	rows, err := t.tx.Query(ctx, query, scope.TenantID, scope.CompanyID, customerID)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	return scanReceivableItems(rows)
}

func (t *transaction) customerReceivableItemsBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]customers.ReceivableItem, error) {
	rows, err := t.tx.Query(ctx, `SELECT item.id,item.tenant_id,item.company_id,item.customer_id,item.kind,item.source_type,item.source_id,
		item.amount_minor,item.amount_minor-COALESCE(allocated.amount_minor,0),item.currency,item.document_at,item.due_at,item.occurred_at
	FROM customer_receivable_items item
	LEFT JOIN LATERAL (
		SELECT sum(a.amount_minor)::bigint AS amount_minor FROM customer_receivable_allocations a
		WHERE a.tenant_id=item.tenant_id AND a.company_id=item.company_id AND
		      (CASE WHEN item.kind='INVOICE' THEN a.debit_item_id=item.id ELSE a.credit_item_id=item.id END)
	) allocated ON true
	WHERE item.tenant_id=$1 AND item.company_id=$2 AND item.source_type=$3 AND item.source_id=$4`,
		scope.TenantID, scope.CompanyID, sourceType, sourceID)
	if err != nil {
		return nil, normalizeError(err)
	}
	defer rows.Close()
	return scanReceivableItems(rows)
}

type receivableRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanReceivableItems(rows receivableRows) ([]customers.ReceivableItem, error) {
	items := make([]customers.ReceivableItem, 0)
	for rows.Next() {
		var item customers.ReceivableItem
		if err := rows.Scan(&item.ID, &item.TenantID, &item.CompanyID, &item.CustomerID, &item.Kind,
			&item.SourceType, &item.SourceID, &item.AmountMinor, &item.OutstandingMinor, &item.Currency,
			&item.DocumentAt, &item.DueAt, &item.OccurredAt); err != nil {
			return nil, normalizeError(err)
		}
		items = append(items, item)
	}
	return items, normalizeError(rows.Err())
}

func (s *Store) CustomerAccountDetail(ctx context.Context, scope tenancy.Scope, actorID, customerID string, at time.Time) (customers.AccountDetail, error) {
	var result customers.AccountDetail
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "customers.accounts.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		result.Customer, err = tx.Customer(ctx, scope, customerID)
		if err != nil {
			return err
		}
		result.ActivePolicy, err = tx.CustomerCreditPolicy(ctx, scope, customerID, at)
		if err != nil {
			return err
		}
		result.Aging, err = tx.CustomerReceivableAging(ctx, scope, customerID, at)
		if err != nil {
			return err
		}
		result.OpenItems, err = tx.customerReceivableItems(ctx, scope, customerID, true)
		if err != nil {
			return err
		}
		rows, err := tx.tx.Query(ctx, `SELECT id,customer_id,credit_enabled,credit_limit_minor,payment_terms_days,
			max_overdue_days,risk_status,reason,effective_from,COALESCE(approved_by::text,'SYSTEM'),created_at,correlation_id
			FROM customer_credit_policies WHERE tenant_id=$1 AND company_id=$2 AND customer_id=$3 AND effective_from>$4
			ORDER BY effective_from`, scope.TenantID, scope.CompanyID, customerID, at)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var policy customers.CreditPolicy
			policy.Scope = scope
			if err := rows.Scan(&policy.ID, &policy.CustomerID, &policy.CreditEnabled, &policy.CreditLimitMinor,
				&policy.PaymentTermsDays, &policy.MaxOverdueDays, &policy.RiskStatus, &policy.Reason,
				&policy.EffectiveFrom, &policy.ApprovedBy, &policy.CreatedAt, &policy.CorrelationID); err != nil {
				return normalizeError(err)
			}
			result.ScheduledPolicies = append(result.ScheduledPolicies, policy)
		}
		return normalizeError(rows.Err())
	})
	return result, err
}

func (s *Store) ScheduleCustomerCreditPolicy(ctx context.Context, policy customers.CreditPolicy, policyAudit audit.Event, policyEvent outbox.Event, commandAt time.Time) (customers.CreditPolicy, error) {
	var result customers.CreditPolicy
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, policy.Scope, policy.ApprovedBy, "customers.credit.manage")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, policy.Scope, receivables.OperationName(), policy.IdempotencyKey, policy.RequestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.creditPolicyByID(ctx, policy.Scope, resultID)
			return err
		}
		if err := tx.LockCustomerCredit(ctx, policy.Scope, policy.CustomerID); err != nil {
			return err
		}
		account, err := tx.Customer(ctx, policy.Scope, policy.CustomerID)
		if err != nil {
			return err
		}
		if account.General && (policy.CreditEnabled || policy.CreditLimitMinor != 0) {
			return sales.ErrGeneralCustomerCredit
		}
		if policy.EffectiveFrom.Before(commandAt.Add(-5 * time.Minute)) {
			return customers.ErrCreditPolicyBackdated
		}
		var latest time.Time
		if err := tx.tx.QueryRow(ctx, `SELECT COALESCE(max(effective_from),'epoch'::timestamptz) FROM customer_credit_policies
			WHERE tenant_id=$1 AND company_id=$2 AND customer_id=$3`, policy.Scope.TenantID, policy.Scope.CompanyID, policy.CustomerID).Scan(&latest); err != nil {
			return normalizeError(err)
		}
		if !policy.EffectiveFrom.After(latest) {
			return customers.ErrCreditPolicySequence
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO customer_credit_policies (
			id,tenant_id,company_id,customer_id,credit_enabled,credit_limit_minor,payment_terms_days,
			max_overdue_days,risk_status,reason,effective_from,approved_by,created_at,correlation_id,idempotency_key,request_hash
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, policy.ID, policy.Scope.TenantID,
			policy.Scope.CompanyID, policy.CustomerID, policy.CreditEnabled, policy.CreditLimitMinor,
			policy.PaymentTermsDays, policy.MaxOverdueDays, policy.RiskStatus, policy.Reason, policy.EffectiveFrom,
			policy.ApprovedBy, policy.CreatedAt, policy.CorrelationID, policy.IdempotencyKey, policy.RequestHash)
		if err != nil {
			return normalizeError(err)
		}
		payload, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		policyAudit.Data, policyEvent.Payload = payload, payload
		if err := tx.AppendAuditEvent(ctx, policyAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, policyEvent); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, policy.Scope, receivables.OperationName(), policy.IdempotencyKey, policy.ID); err != nil {
			return err
		}
		result = policy
		return nil
	})
	return result, err
}

func (t *transaction) creditPolicyByID(ctx context.Context, scope tenancy.Scope, policyID string) (customers.CreditPolicy, error) {
	var value customers.CreditPolicy
	value.Scope = scope
	err := t.tx.QueryRow(ctx, `SELECT id,customer_id,credit_enabled,credit_limit_minor,payment_terms_days,
		max_overdue_days,risk_status,reason,effective_from,COALESCE(approved_by::text,'SYSTEM'),created_at,correlation_id
		FROM customer_credit_policies WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, policyID).Scan(
		&value.ID, &value.CustomerID, &value.CreditEnabled, &value.CreditLimitMinor, &value.PaymentTermsDays,
		&value.MaxOverdueDays, &value.RiskStatus, &value.Reason, &value.EffectiveFrom, &value.ApprovedBy,
		&value.CreatedAt, &value.CorrelationID)
	return value, normalizeError(err)
}
