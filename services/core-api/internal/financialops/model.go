// Package financialops owns governed general-ledger commands and fiscal-period control.
package financialops

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type DocumentType string

const (
	ManualJournal  DocumentType = "MANUAL_JOURNAL"
	CashTransfer   DocumentType = "CASH_TRANSFER"
	BankAdjustment DocumentType = "BANK_ADJUSTMENT"
	Reversal       DocumentType = "REVERSAL"
)

type Status string

const (
	Draft     Status = "DRAFT"
	Submitted Status = "SUBMITTED"
	Posted    Status = "POSTED"
	Rejected  Status = "REJECTED"
)

type Document struct {
	ID                 string                 `json:"id"`
	Scope              tenancy.Scope          `json:"scope"`
	Number             string                 `json:"number"`
	Type               DocumentType           `json:"type"`
	Status             Status                 `json:"status"`
	Currency           string                 `json:"currency"`
	AccountingAt       time.Time              `json:"accounting_at"`
	Reason             string                 `json:"reason"`
	FromAccountID      string                 `json:"from_account_id,omitempty"`
	ToAccountID        string                 `json:"to_account_id,omitempty"`
	ReversesDocumentID string                 `json:"reverses_document_id,omitempty"`
	JournalID          string                 `json:"journal_id,omitempty"`
	CreatedBy          string                 `json:"created_by"`
	CreatedAt          time.Time              `json:"created_at"`
	SubmittedBy        string                 `json:"submitted_by,omitempty"`
	PostedBy           string                 `json:"posted_by,omitempty"`
	Lines              []finance.JournalEntry `json:"lines"`
}
type Page struct {
	Items      []Document `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

type FiscalPeriod struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	CompanyID string    `json:"company_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Open      bool      `json:"open"`
}
type PeriodAction string

const (
	ClosePeriod  PeriodAction = "CLOSE"
	ReopenPeriod PeriodAction = "REOPEN"
)

type PeriodActionRequest struct {
	ID          string       `json:"id"`
	PeriodID    string       `json:"period_id"`
	Action      PeriodAction `json:"action"`
	Reason      string       `json:"reason"`
	RequestedBy string       `json:"requested_by"`
	RequestedAt time.Time    `json:"requested_at"`
	ApprovedBy  string       `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time   `json:"approved_at,omitempty"`
}
type PeriodActionPage struct {
	Items []PeriodActionRequest `json:"items"`
}

var (
	ErrInvalidCommand     = errors.New("financial control command is invalid")
	ErrInvalidTransition  = errors.New("financial document transition is invalid")
	ErrSeparationOfDuties = errors.New("maker cannot approve or post own financial command")
	ErrPeriodClosed       = errors.New("fiscal period is closed")
	ErrPeriodCloseBlocked = errors.New("fiscal period has unreconciled bank evidence or unposted financial documents")
	ErrAlreadyReversed    = errors.New("financial document is already reversed")
)
