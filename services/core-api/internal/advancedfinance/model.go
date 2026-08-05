// Package advancedfinance owns governed budgets and the fixed-asset subledger.
package advancedfinance

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Status string

const (
	Draft     Status = "DRAFT"
	Submitted Status = "SUBMITTED"
	Approved  Status = "APPROVED"
	Rejected  Status = "REJECTED"
	Active    Status = "ACTIVE"
	Disposed  Status = "DISPOSED"
)

type BudgetLine struct {
	AccountID   string    `json:"account_id"`
	Month       time.Time `json:"month"`
	AmountMinor int64     `json:"amount_minor"`
}

type Budget struct {
	ID         string        `json:"id"`
	Scope      tenancy.Scope `json:"scope"`
	Name       string        `json:"name"`
	FiscalYear int           `json:"fiscal_year"`
	Currency   string        `json:"currency"`
	Status     Status        `json:"status"`
	Reason     string        `json:"reason"`
	Lines      []BudgetLine  `json:"lines"`
	CreatedBy  string        `json:"created_by"`
	CreatedAt  time.Time     `json:"created_at"`
	ApprovedBy string        `json:"approved_by,omitempty"`
}

type BudgetActualLine struct {
	AccountID     string    `json:"account_id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Month         time.Time `json:"month"`
	BudgetMinor   int64     `json:"budget_minor"`
	ActualMinor   int64     `json:"actual_minor"`
	VarianceMinor int64     `json:"variance_minor"`
}

type BudgetActual struct {
	BudgetID           string             `json:"budget_id"`
	Currency           string             `json:"currency"`
	Lines              []BudgetActualLine `json:"lines"`
	TotalBudgetMinor   int64              `json:"total_budget_minor"`
	TotalActualMinor   int64              `json:"total_actual_minor"`
	TotalVarianceMinor int64              `json:"total_variance_minor"`
	GeneratedAt        time.Time          `json:"generated_at"`
}

type Asset struct {
	ID                               string        `json:"id"`
	Scope                            tenancy.Scope `json:"scope"`
	Code                             string        `json:"code"`
	Name                             string        `json:"name"`
	Category                         string        `json:"category"`
	Status                           Status        `json:"status"`
	Currency                         string        `json:"currency"`
	AcquiredAt                       time.Time     `json:"acquired_at"`
	CostMinor                        int64         `json:"cost_minor"`
	ResidualMinor                    int64         `json:"residual_minor"`
	UsefulLifeMonths                 int           `json:"useful_life_months"`
	AccumulatedDepreciationMinor     int64         `json:"accumulated_depreciation_minor"`
	NetBookValueMinor                int64         `json:"net_book_value_minor"`
	AssetAccountID                   string        `json:"asset_account_id"`
	AccumulatedDepreciationAccountID string        `json:"accumulated_depreciation_account_id"`
	DepreciationExpenseAccountID     string        `json:"depreciation_expense_account_id"`
	CapitalizationOffsetAccountID    string        `json:"capitalization_offset_account_id"`
	DisposalGainAccountID            string        `json:"disposal_gain_account_id"`
	DisposalLossAccountID            string        `json:"disposal_loss_account_id"`
	Reason                           string        `json:"reason"`
	CreatedBy                        string        `json:"created_by"`
	CreatedAt                        time.Time     `json:"created_at"`
	ApprovedBy                       string        `json:"approved_by,omitempty"`
	DisposedAt                       *time.Time    `json:"disposed_at,omitempty"`
}

type Depreciation struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	Period      time.Time `json:"period"`
	AmountMinor int64     `json:"amount_minor"`
	JournalID   string    `json:"journal_id"`
	PostedBy    string    `json:"posted_by"`
	PostedAt    time.Time `json:"posted_at"`
}

type Page[T any] struct {
	Items []T `json:"items"`
}

var (
	ErrInvalidCommand        = errors.New("advanced finance command is invalid")
	ErrInvalidTransition     = errors.New("advanced finance transition is invalid")
	ErrSeparationOfDuties    = errors.New("maker cannot approve own advanced finance command")
	ErrAssetFullyDepreciated = errors.New("asset is already fully depreciated")
)
