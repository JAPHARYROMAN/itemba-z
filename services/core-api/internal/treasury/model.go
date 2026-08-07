// Package treasury owns governed borrowing facilities and their append-only subledger.
package treasury

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type FacilityType string
type Status string
type TransactionType string

const (
	TermLoan  FacilityType = "TERM_LOAN"
	Overdraft FacilityType = "OVERDRAFT"

	Draft     Status = "DRAFT"
	Submitted Status = "SUBMITTED"
	Active    Status = "ACTIVE"
	Rejected  Status = "REJECTED"
	Closed    Status = "CLOSED"

	Drawdown           TransactionType = "DRAWDOWN"
	PrincipalRepayment TransactionType = "PRINCIPAL_REPAYMENT"
	InterestAccrual    TransactionType = "INTEREST_ACCRUAL"
	InterestPayment    TransactionType = "INTEREST_PAYMENT"
)

type Facility struct {
	ID                        string        `json:"id"`
	Scope                     tenancy.Scope `json:"scope"`
	Reference                 string        `json:"reference"`
	Lender                    string        `json:"lender"`
	Type                      FacilityType  `json:"type"`
	Status                    Status        `json:"status"`
	Currency                  string        `json:"currency"`
	LimitMinor                int64         `json:"limit_minor"`
	AnnualInterestBasisPoints int64         `json:"annual_interest_basis_points"`
	StartDate                 time.Time     `json:"start_date"`
	MaturityDate              time.Time     `json:"maturity_date"`
	BankAccountID             string        `json:"bank_account_id"`
	PrincipalAccountID        string        `json:"principal_account_id"`
	InterestExpenseAccountID  string        `json:"interest_expense_account_id"`
	AccruedInterestAccountID  string        `json:"accrued_interest_account_id"`
	OutstandingPrincipalMinor int64         `json:"outstanding_principal_minor"`
	AccruedInterestMinor      int64         `json:"accrued_interest_minor"`
	AvailableMinor            int64         `json:"available_minor"`
	Reason                    string        `json:"reason"`
	CreatedBy                 string        `json:"created_by"`
	CreatedAt                 time.Time     `json:"created_at"`
	ApprovedBy                string        `json:"approved_by,omitempty"`
	ClosedAt                  *time.Time    `json:"closed_at,omitempty"`
	Transactions              []Transaction `json:"transactions"`
}

type Transaction struct {
	ID          string          `json:"id"`
	FacilityID  string          `json:"facility_id"`
	Type        TransactionType `json:"type"`
	AmountMinor int64           `json:"amount_minor"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Reason      string          `json:"reason"`
	JournalID   string          `json:"journal_id"`
	PostedBy    string          `json:"posted_by"`
	PostedAt    time.Time       `json:"posted_at"`
}

type Page struct {
	Items []Facility `json:"items"`
}

var (
	ErrInvalidCommand     = errors.New("treasury command is invalid")
	ErrInvalidTransition  = errors.New("treasury transition is invalid")
	ErrSeparationOfDuties = errors.New("maker cannot approve own treasury facility")
	ErrLimitExceeded      = errors.New("treasury facility limit or balance exceeded")
)
