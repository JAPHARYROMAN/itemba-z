// Package groupfinance owns dual-company intercompany accounting and consolidation.
package groupfinance

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type TransactionType string
type Status string

const (
	CashTransfer   TransactionType = "CASH_TRANSFER"
	CostAllocation TransactionType = "COST_ALLOCATION"
	Draft          Status          = "DRAFT"
	Submitted      Status          = "SUBMITTED"
	SourceApproved Status          = "SOURCE_APPROVED"
	Posted         Status          = "POSTED"
	Rejected       Status          = "REJECTED"
)

type Transaction struct {
	ID                          string          `json:"id"`
	Scope                       tenancy.Scope   `json:"scope"`
	CounterpartyCompanyID       string          `json:"counterparty_company_id"`
	CounterpartyCompanyName     string          `json:"counterparty_company_name,omitempty"`
	Reference                   string          `json:"reference"`
	Type                        TransactionType `json:"type"`
	Status                      Status          `json:"status"`
	Currency                    string          `json:"currency"`
	AmountMinor                 int64           `json:"amount_minor"`
	OccurredAt                  time.Time       `json:"occurred_at"`
	SourceDebitAccountID        string          `json:"source_debit_account_id"`
	SourceCreditAccountID       string          `json:"source_credit_account_id"`
	CounterpartyDebitAccountID  string          `json:"counterparty_debit_account_id"`
	CounterpartyCreditAccountID string          `json:"counterparty_credit_account_id"`
	Reason                      string          `json:"reason"`
	CreatedBy                   string          `json:"created_by"`
	CreatedAt                   time.Time       `json:"created_at"`
	SourceApprovedBy            string          `json:"source_approved_by,omitempty"`
	PostedBy                    string          `json:"posted_by,omitempty"`
	SourceJournalID             string          `json:"source_journal_id,omitempty"`
	CounterpartyJournalID       string          `json:"counterparty_journal_id,omitempty"`
}

type Page struct {
	Items []Transaction `json:"items"`
}
type CompanySummary struct {
	CompanyID        string `json:"company_id"`
	CompanyName      string `json:"company_name"`
	AssetsMinor      int64  `json:"assets_minor"`
	LiabilitiesMinor int64  `json:"liabilities_minor"`
	EquityMinor      int64  `json:"equity_minor"`
	RevenueMinor     int64  `json:"revenue_minor"`
	ExpenseMinor     int64  `json:"expense_minor"`
}
type Consolidation struct {
	Currency                             string           `json:"currency"`
	AsOf                                 time.Time        `json:"as_of"`
	Companies                            []CompanySummary `json:"companies"`
	AssetsBeforeMinor                    int64            `json:"assets_before_minor"`
	LiabilitiesBeforeMinor               int64            `json:"liabilities_before_minor"`
	EquityMinor                          int64            `json:"equity_minor"`
	RevenueBeforeMinor                   int64            `json:"revenue_before_minor"`
	ExpenseBeforeMinor                   int64            `json:"expense_before_minor"`
	IntercompanyBalanceEliminationMinor  int64            `json:"intercompany_balance_elimination_minor"`
	IntercompanyActivityEliminationMinor int64            `json:"intercompany_activity_elimination_minor"`
	AssetsMinor                          int64            `json:"assets_minor"`
	LiabilitiesMinor                     int64            `json:"liabilities_minor"`
	RevenueMinor                         int64            `json:"revenue_minor"`
	ExpenseMinor                         int64            `json:"expense_minor"`
	NetProfitMinor                       int64            `json:"net_profit_minor"`
	Balanced                             bool             `json:"balanced"`
}

var (
	ErrInvalidCommand       = errors.New("group finance command is invalid")
	ErrInvalidTransition    = errors.New("intercompany transition is invalid")
	ErrSeparationOfDuties   = errors.New("intercompany maker-checker separation failed")
	ErrCounterpartyApproval = errors.New("counterparty company approval is required")
)
