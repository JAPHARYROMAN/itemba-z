// Package banking owns cash and bank account statement reconciliation.
// Imported statement facts, matches, and approvals are append-only evidence.
package banking

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type AccountType string

const (
	BankAccount        AccountType = "BANK"
	CashAccount        AccountType = "CASH"
	MobileMoneyAccount AccountType = "MOBILE_MONEY"
)

type StatementStatus string

const (
	Imported   StatementStatus = "IMPORTED"
	Reconciled StatementStatus = "RECONCILED"
)

type Account struct {
	ID          string        `json:"id"`
	Scope       tenancy.Scope `json:"scope"`
	Code        string        `json:"code"`
	Name        string        `json:"name"`
	Type        AccountType   `json:"type"`
	Currency    string        `json:"currency"`
	GLAccountID string        `json:"gl_account_id"`
	Active      bool          `json:"active"`
}

type Candidate struct {
	JournalLineID string    `json:"journal_line_id"`
	JournalID     string    `json:"journal_id"`
	SourceType    string    `json:"source_type"`
	SourceID      string    `json:"source_id"`
	AmountMinor   int64     `json:"amount_minor"`
	OccurredAt    time.Time `json:"occurred_at"`
	Memo          string    `json:"memo"`
}

type Match struct {
	ID            string    `json:"id"`
	JournalLineID string    `json:"journal_line_id"`
	ActorID       string    `json:"actor_id"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type StatementLine struct {
	ID                string      `json:"id"`
	TransactionAt     time.Time   `json:"transaction_at"`
	ExternalReference string      `json:"external_reference"`
	Description       string      `json:"description"`
	AmountMinor       int64       `json:"amount_minor"`
	Match             *Match      `json:"match,omitempty"`
	Candidates        []Candidate `json:"candidates"`
}

type Statement struct {
	ID                string          `json:"id"`
	Scope             tenancy.Scope   `json:"scope"`
	AccountID         string          `json:"account_id"`
	ExternalReference string          `json:"external_reference"`
	Currency          string          `json:"currency"`
	PeriodStart       time.Time       `json:"period_start"`
	PeriodEnd         time.Time       `json:"period_end"`
	OpeningMinor      int64           `json:"opening_minor"`
	ClosingMinor      int64           `json:"closing_minor"`
	Status            StatementStatus `json:"status"`
	ImportedBy        string          `json:"imported_by"`
	ImportedAt        time.Time       `json:"imported_at"`
	ReconciledBy      string          `json:"reconciled_by,omitempty"`
	ReconciledAt      *time.Time      `json:"reconciled_at,omitempty"`
	CorrelationID     string          `json:"correlation_id"`
	Lines             []StatementLine `json:"lines"`
}

type Page struct {
	Items      []Statement `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

var (
	ErrInvalidCommand     = errors.New("banking command is incomplete or invalid")
	ErrInactiveAccount    = errors.New("cash or bank account is inactive")
	ErrStatementImbalance = errors.New("statement opening balance plus movements must equal closing balance")
	ErrDuplicateStatement = errors.New("statement reference has already been imported for this account")
	ErrMatchMismatch      = errors.New("statement line and general-ledger entry do not match")
	ErrAlreadyMatched     = errors.New("statement or general-ledger line is already matched")
	ErrUnmatchedLines     = errors.New("all statement lines must be matched before reconciliation")
	ErrSeparationOfDuties = errors.New("statement importer cannot approve its reconciliation")
	ErrAlreadyReconciled  = errors.New("statement is already reconciled")
)
