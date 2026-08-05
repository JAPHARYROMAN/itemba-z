package customers

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type CreditRiskStatus string

const (
	CreditRiskStandard CreditRiskStatus = "STANDARD"
	CreditRiskWatch    CreditRiskStatus = "WATCH"
	CreditRiskHold     CreditRiskStatus = "HOLD"
)

// CreditPolicy is an append-only, effective-dated authorization fact. A later
// policy supersedes an earlier policy; historical rows are never closed or edited.
type CreditPolicy struct {
	ID               string           `json:"id"`
	Scope            tenancy.Scope    `json:"scope"`
	CustomerID       string           `json:"customer_id"`
	CreditEnabled    bool             `json:"credit_enabled"`
	CreditLimitMinor int64            `json:"credit_limit_minor"`
	PaymentTermsDays int64            `json:"payment_terms_days"`
	MaxOverdueDays   int64            `json:"max_overdue_days"`
	RiskStatus       CreditRiskStatus `json:"risk_status"`
	Reason           string           `json:"reason"`
	EffectiveFrom    time.Time        `json:"effective_from"`
	ApprovedBy       string           `json:"approved_by"`
	CreatedAt        time.Time        `json:"created_at"`
	CorrelationID    string           `json:"correlation_id"`
	IdempotencyKey   string           `json:"-"`
	RequestHash      string           `json:"-"`
}

type ReceivableItemKind string

const (
	ReceivableInvoice    ReceivableItemKind = "INVOICE"
	ReceivableCreditNote ReceivableItemKind = "CREDIT_NOTE"
	ReceivableReceipt    ReceivableItemKind = "RECEIPT"
)

// ReceivableItem retains invoice-level provenance. Amounts are positive;
// CREDIT_NOTE and RECEIPT items reduce exposure through allocations.
type ReceivableItem struct {
	ID               string             `json:"id"`
	TenantID         string             `json:"tenant_id"`
	CompanyID        string             `json:"company_id"`
	CustomerID       string             `json:"customer_id"`
	Kind             ReceivableItemKind `json:"kind"`
	SourceType       string             `json:"source_type"`
	SourceID         string             `json:"source_id"`
	AmountMinor      int64              `json:"amount_minor"`
	OutstandingMinor int64              `json:"outstanding_minor"`
	Currency         string             `json:"currency"`
	DocumentAt       time.Time          `json:"document_at"`
	DueAt            *time.Time         `json:"due_at"`
	OccurredAt       time.Time          `json:"occurred_at"`
}

type ReceivableAllocation struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	CompanyID    string    `json:"company_id"`
	CustomerID   string    `json:"customer_id"`
	DebitItemID  string    `json:"debit_item_id"`
	CreditItemID string    `json:"credit_item_id"`
	AmountMinor  int64     `json:"amount_minor"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type AgingBuckets struct {
	CurrentMinor int64 `json:"current_minor"`
	Days1To30    int64 `json:"days_1_30_minor"`
	Days31To60   int64 `json:"days_31_60_minor"`
	Days61To90   int64 `json:"days_61_90_minor"`
	DaysOver90   int64 `json:"days_over_90_minor"`
}

type ReceivableAging struct {
	LedgerBalanceMinor   int64        `json:"ledger_balance_minor"`
	OpenInvoiceMinor     int64        `json:"open_invoice_minor"`
	UnappliedCreditMinor int64        `json:"unapplied_credit_minor"`
	CalculatedExposure   int64        `json:"calculated_exposure_minor"`
	OverdueMinor         int64        `json:"overdue_minor"`
	OldestOverdueDays    int64        `json:"oldest_overdue_days"`
	Reconciled           bool         `json:"reconciled"`
	Buckets              AgingBuckets `json:"buckets"`
}

type AccountDetail struct {
	Customer          Account          `json:"customer"`
	ActivePolicy      CreditPolicy     `json:"active_policy"`
	ScheduledPolicies []CreditPolicy   `json:"scheduled_policies"`
	Aging             ReceivableAging  `json:"aging"`
	OpenItems         []ReceivableItem `json:"open_items"`
}

var (
	ErrCreditPolicyMissing   = errors.New("no effective customer credit policy is configured")
	ErrCreditPolicySequence  = errors.New("credit policy effective time must follow the latest policy")
	ErrCreditPolicyBackdated = errors.New("credit policy cannot be materially backdated")
	ErrCreditRiskHold        = errors.New("customer credit policy is on hold")
	ErrCreditOverdue         = errors.New("customer has receivables beyond the approved overdue tolerance")
	ErrReceivablesUnbalanced = errors.New("customer receivable open items do not reconcile to the customer ledger")
)
