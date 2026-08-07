// Package customers owns company-specific customer credit accounts and their
// append-only subledger entries.
package customers

import "time"

type Account struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	TenantID         string `json:"tenant_id"`
	CompanyID        string `json:"company_id"`
	Name             string `json:"name"`
	Active           bool   `json:"active"`
	General          bool   `json:"is_general_customer"`
	CreditEnabled    bool   `json:"legacy_credit_enabled"`
	CreditLimitMinor int64  `json:"legacy_credit_limit_minor"`
}

// LedgerEntry uses positive amounts for receivable increases and negative
// amounts for payments, credits, or reversals.
type LedgerEntry struct {
	ID          string
	TenantID    string
	CompanyID   string
	CustomerID  string
	SourceType  string
	SourceID    string
	AmountMinor int64
	Currency    string
	OccurredAt  time.Time
}
