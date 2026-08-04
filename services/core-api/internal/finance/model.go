// Package finance owns posting rules and immutable balanced journals.
package finance

import (
	"errors"
	"fmt"
	"time"
)

var ErrUnbalancedJournal = errors.New("journal debits and credits must be non-zero and equal")

type SalesPostingConfig struct {
	ReceivableAccountID string
	TaxPayableAccountID string
	CashAccounts        map[string]string
}

type Journal struct {
	ID         string
	TenantID   string
	CompanyID  string
	SourceType string
	SourceID   string
	Currency   string
	OccurredAt time.Time
	Entries    []JournalEntry
}

type JournalEntry struct {
	AccountID   string
	DebitMinor  int64
	CreditMinor int64
	Memo        string
}

func (j Journal) Validate() error {
	if j.ID == "" || j.TenantID == "" || j.CompanyID == "" || j.SourceType == "" ||
		j.SourceID == "" || len(j.Currency) != 3 || len(j.Entries) < 2 {
		return fmt.Errorf("%w: incomplete journal", ErrUnbalancedJournal)
	}
	var debits, credits int64
	for _, entry := range j.Entries {
		if entry.AccountID == "" || entry.DebitMinor < 0 || entry.CreditMinor < 0 ||
			(entry.DebitMinor == 0) == (entry.CreditMinor == 0) {
			return fmt.Errorf("%w: each line must have exactly one positive side", ErrUnbalancedJournal)
		}
		debits += entry.DebitMinor
		credits += entry.CreditMinor
	}
	if debits == 0 || debits != credits {
		return fmt.Errorf("%w: debits=%d credits=%d", ErrUnbalancedJournal, debits, credits)
	}
	return nil
}

func (j Journal) Reversed(id, sourceType, sourceID string, at time.Time) Journal {
	entries := make([]JournalEntry, 0, len(j.Entries))
	for _, entry := range j.Entries {
		entries = append(entries, JournalEntry{
			AccountID:   entry.AccountID,
			DebitMinor:  entry.CreditMinor,
			CreditMinor: entry.DebitMinor,
			Memo:        "Reversal: " + entry.Memo,
		})
	}
	return Journal{
		ID: id, TenantID: j.TenantID, CompanyID: j.CompanyID,
		SourceType: sourceType, SourceID: sourceID, Currency: j.Currency,
		OccurredAt: at, Entries: entries,
	}
}
