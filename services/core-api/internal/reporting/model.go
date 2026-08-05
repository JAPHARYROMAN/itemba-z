// Package reporting owns governed, legal-company financial statement read models.
package reporting

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

var ErrInvalidQuery = errors.New("invalid financial report query")

type Query struct {
	Scope     tenancy.Scope
	ActorID   string
	From      time.Time
	To        time.Time
	AsOf      time.Time
	AccountID string
}

type AccountActivity struct {
	AccountID       string                   `json:"account_id"`
	Code            string                   `json:"code"`
	Name            string                   `json:"name"`
	Type            financialops.AccountType `json:"type"`
	ParentAccountID string                   `json:"parent_account_id,omitempty"`
	OpeningMinor    int64                    `json:"opening_minor"`
	DebitMinor      int64                    `json:"debit_minor"`
	CreditMinor     int64                    `json:"credit_minor"`
}

type StatementLine struct {
	AccountID   string                   `json:"account_id"`
	Code        string                   `json:"code"`
	Name        string                   `json:"name"`
	Type        financialops.AccountType `json:"type"`
	AmountMinor int64                    `json:"amount_minor"`
	DebitMinor  int64                    `json:"debit_minor,omitempty"`
	CreditMinor int64                    `json:"credit_minor,omitempty"`
}

type TrialBalance struct {
	AsOf             time.Time       `json:"as_of"`
	Currency         string          `json:"currency"`
	Lines            []StatementLine `json:"lines"`
	TotalDebitMinor  int64           `json:"total_debit_minor"`
	TotalCreditMinor int64           `json:"total_credit_minor"`
	Balanced         bool            `json:"balanced"`
	GeneratedAt      time.Time       `json:"generated_at"`
}

type LedgerEntry struct {
	JournalLineID       int64     `json:"journal_line_id"`
	JournalID           string    `json:"journal_id"`
	OccurredAt          time.Time `json:"occurred_at"`
	SourceType          string    `json:"source_type"`
	SourceID            string    `json:"source_id"`
	Memo                string    `json:"memo"`
	DebitMinor          int64     `json:"debit_minor"`
	CreditMinor         int64     `json:"credit_minor"`
	RunningBalanceMinor int64     `json:"running_balance_minor"`
}

type GeneralLedger struct {
	From                time.Time     `json:"from"`
	To                  time.Time     `json:"to"`
	Currency            string        `json:"currency"`
	AccountID           string        `json:"account_id"`
	AccountCode         string        `json:"account_code"`
	AccountName         string        `json:"account_name"`
	OpeningBalanceMinor int64         `json:"opening_balance_minor"`
	ClosingBalanceMinor int64         `json:"closing_balance_minor"`
	TotalDebitMinor     int64         `json:"total_debit_minor"`
	TotalCreditMinor    int64         `json:"total_credit_minor"`
	Entries             []LedgerEntry `json:"entries"`
	GeneratedAt         time.Time     `json:"generated_at"`
}

type ProfitAndLoss struct {
	From              time.Time       `json:"from"`
	To                time.Time       `json:"to"`
	Currency          string          `json:"currency"`
	Revenue           []StatementLine `json:"revenue"`
	Expenses          []StatementLine `json:"expenses"`
	TotalRevenueMinor int64           `json:"total_revenue_minor"`
	TotalExpenseMinor int64           `json:"total_expense_minor"`
	NetProfitMinor    int64           `json:"net_profit_minor"`
	UnclassifiedMinor int64           `json:"unclassified_minor"`
	GeneratedAt       time.Time       `json:"generated_at"`
}

type BalanceSheet struct {
	AsOf                  time.Time       `json:"as_of"`
	Currency              string          `json:"currency"`
	Assets                []StatementLine `json:"assets"`
	Liabilities           []StatementLine `json:"liabilities"`
	Equity                []StatementLine `json:"equity"`
	TotalAssetsMinor      int64           `json:"total_assets_minor"`
	TotalLiabilitiesMinor int64           `json:"total_liabilities_minor"`
	TotalEquityMinor      int64           `json:"total_equity_minor"`
	CurrentEarningsMinor  int64           `json:"current_earnings_minor"`
	UnclassifiedMinor     int64           `json:"unclassified_minor"`
	Balanced              bool            `json:"balanced"`
	GeneratedAt           time.Time       `json:"generated_at"`
}

type CashMovement struct {
	JournalID   string    `json:"journal_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	SourceType  string    `json:"source_type"`
	SourceID    string    `json:"source_id"`
	Memo        string    `json:"memo"`
	AmountMinor int64     `json:"amount_minor"`
}

type CashFlowSection struct {
	Activity string         `json:"activity"`
	Lines    []CashMovement `json:"lines"`
	NetMinor int64          `json:"net_minor"`
}

type CashFlow struct {
	From             time.Time       `json:"from"`
	To               time.Time       `json:"to"`
	Currency         string          `json:"currency"`
	OpeningCashMinor int64           `json:"opening_cash_minor"`
	Operating        CashFlowSection `json:"operating"`
	Investing        CashFlowSection `json:"investing"`
	Financing        CashFlowSection `json:"financing"`
	Unclassified     CashFlowSection `json:"unclassified"`
	NetChangeMinor   int64           `json:"net_change_minor"`
	ClosingCashMinor int64           `json:"closing_cash_minor"`
	Reconciled       bool            `json:"reconciled"`
	Classified       bool            `json:"classified"`
	GeneratedAt      time.Time       `json:"generated_at"`
}

type ReportType string

const (
	ReportTrialBalance  ReportType = "TRIAL_BALANCE"
	ReportGeneralLedger ReportType = "GENERAL_LEDGER"
	ReportProfitAndLoss ReportType = "PROFIT_AND_LOSS"
	ReportBalanceSheet  ReportType = "BALANCE_SHEET"
	ReportCashFlow      ReportType = "CASH_FLOW"
)

type ExportCommand struct {
	Query
	Type           ReportType
	IdempotencyKey string
}

type ExportArtifact struct {
	ID            string     `json:"id"`
	ReportType    ReportType `json:"report_type"`
	Filename      string     `json:"filename"`
	MediaType     string     `json:"media_type"`
	ContentBase64 string     `json:"content_base64"`
	GeneratedAt   time.Time  `json:"generated_at"`
}
