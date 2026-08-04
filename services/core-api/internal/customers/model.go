// Package customers owns company-specific customer credit accounts and their
// append-only subledger entries.
package customers

import "time"

type Account struct {
	ID               string
	TenantID         string
	CompanyID        string
	Name             string
	Active           bool
	General          bool
	CreditEnabled    bool
	CreditLimitMinor int64
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
