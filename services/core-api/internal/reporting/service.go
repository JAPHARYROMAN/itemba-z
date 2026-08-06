package reporting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Repository interface {
	AccountActivity(context.Context, tenancy.Scope, string, time.Time, time.Time) ([]AccountActivity, string, error)
	LedgerEntries(context.Context, tenancy.Scope, string, string, time.Time, time.Time) (GeneralLedger, error)
	CashActivity(context.Context, tenancy.Scope, string, time.Time, time.Time) (int64, int64, []CashMovement, string, error)
	DashboardControls(context.Context, tenancy.Scope, string) (int64, string, error)
	SaveReportExport(context.Context, tenancy.Scope, string, ExportArtifact, string, string) (ExportArtifact, error)
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || c == nil {
		return nil, errors.New("reporting repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: c}, nil
}

func normalizeQuery(q Query, requireRange bool, requireAsOf bool) (Query, error) {
	q.Scope = q.Scope.Normalize()
	q.ActorID = identity.NormalizeClaim(q.ActorID)
	q.AccountID = strings.ToLower(strings.TrimSpace(q.AccountID))
	if q.Scope.Validate() != nil || q.ActorID == "" {
		return q, ErrInvalidQuery
	}
	if requireRange && (q.From.IsZero() || q.To.IsZero() || q.To.Before(q.From) || q.To.Sub(q.From) > 366*24*time.Hour) {
		return q, ErrInvalidQuery
	}
	if requireAsOf && q.AsOf.IsZero() {
		return q, ErrInvalidQuery
	}
	q.From, q.To, q.AsOf = day(q.From), day(q.To), day(q.AsOf)
	return q, nil
}
func day(v time.Time) time.Time {
	if v.IsZero() {
		return v
	}
	y, m, d := v.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func safe(values ...int64) error {
	for _, v := range values {
		if !wire.IsSafeInteger(v) {
			return wire.ErrUnsafeInteger
		}
	}
	return nil
}

func decimalAmount(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

// Dashboard returns a month-to-date executive snapshot derived from the same
// governed reporting services used by the financial statements. The period is
// based on the service clock and the legal company's configured business
// timezone; repository implementations then translate those calendar dates to
// exact query instants.
func (s *Service) Dashboard(ctx context.Context, q Query) (Dashboard, error) {
	now := s.clock.Now().UTC()
	providedRange := !q.From.IsZero() || !q.To.IsZero()
	var err error
	q, err = normalizeQuery(q, providedRange, false)
	if err != nil {
		return Dashboard{}, err
	}
	pending, zoneName, err := s.repository.DashboardControls(ctx, q.Scope, q.ActorID)
	if err != nil {
		return Dashboard{}, err
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return Dashboard{}, ErrInvalidQuery
	}
	localNow := now.In(zone)
	to := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	from := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, time.UTC)
	if providedRange {
		from, to = q.From, q.To
	}
	q.From, q.To, q.AsOf = from, to, to

	profitAndLoss, err := s.ProfitAndLoss(ctx, q)
	if err != nil {
		return Dashboard{}, err
	}
	balanceSheet, err := s.BalanceSheet(ctx, q)
	if err != nil {
		return Dashboard{}, err
	}
	cashFlow, err := s.CashFlow(ctx, q)
	if err != nil {
		return Dashboard{}, err
	}
	if err = safe(profitAndLoss.TotalRevenueMinor, profitAndLoss.NetProfitMinor, cashFlow.ClosingCashMinor, balanceSheet.TotalAssetsMinor, pending); err != nil {
		return Dashboard{}, err
	}

	currency := profitAndLoss.Currency
	metric := func(key, label string, minor int64) Metric {
		return Metric{Key: key, Label: label, Value: Money{Amount: decimalAmount(minor), Currency: currency}}
	}
	return Dashboard{
		AsOf: now,
		Metrics: []Metric{
			metric("revenue", "Month-to-date revenue", profitAndLoss.TotalRevenueMinor),
			metric("net_profit", "Month-to-date net profit", profitAndLoss.NetProfitMinor),
			metric("cash_position", "Cash position", cashFlow.ClosingCashMinor),
			metric("total_assets", "Total assets", balanceSheet.TotalAssetsMinor),
		},
		Alerts:           []DashboardAlert{},
		PendingApprovals: pending,
	}, nil
}

func (s *Service) TrialBalance(ctx context.Context, q Query) (TrialBalance, error) {
	q, err := normalizeQuery(q, false, true)
	if err != nil {
		return TrialBalance{}, err
	}
	activities, currency, err := s.repository.AccountActivity(ctx, q.Scope, q.ActorID, time.Time{}, q.AsOf.AddDate(0, 0, 1))
	if err != nil {
		return TrialBalance{}, err
	}
	result := TrialBalance{AsOf: q.AsOf, Currency: currency, Lines: []StatementLine{}, GeneratedAt: s.clock.Now().UTC()}
	for _, a := range activities {
		net := a.OpeningMinor + a.DebitMinor - a.CreditMinor
		line := StatementLine{AccountID: a.AccountID, Code: a.Code, Name: a.Name, Type: a.Type}
		if net >= 0 {
			line.DebitMinor = net
			result.TotalDebitMinor += net
		} else {
			line.CreditMinor = -net
			result.TotalCreditMinor -= net
		}
		if net != 0 {
			result.Lines = append(result.Lines, line)
		}
	}
	result.Balanced = result.TotalDebitMinor == result.TotalCreditMinor
	if err = safe(result.TotalDebitMinor, result.TotalCreditMinor); err != nil {
		return TrialBalance{}, err
	}
	return result, nil
}

func (s *Service) GeneralLedger(ctx context.Context, q Query) (GeneralLedger, error) {
	q, err := normalizeQuery(q, true, false)
	if err != nil || q.AccountID == "" {
		return GeneralLedger{}, ErrInvalidQuery
	}
	result, err := s.repository.LedgerEntries(ctx, q.Scope, q.ActorID, q.AccountID, q.From, q.To.AddDate(0, 0, 1))
	if err != nil {
		return result, err
	}
	result.From = q.From
	result.To = q.To
	result.GeneratedAt = s.clock.Now().UTC()
	if err = safe(result.OpeningBalanceMinor, result.ClosingBalanceMinor, result.TotalDebitMinor, result.TotalCreditMinor); err != nil {
		return GeneralLedger{}, err
	}
	for _, v := range result.Entries {
		if err = safe(v.DebitMinor, v.CreditMinor, v.RunningBalanceMinor); err != nil {
			return GeneralLedger{}, err
		}
	}
	return result, nil
}

func (s *Service) ProfitAndLoss(ctx context.Context, q Query) (ProfitAndLoss, error) {
	q, err := normalizeQuery(q, true, false)
	if err != nil {
		return ProfitAndLoss{}, err
	}
	activities, currency, err := s.repository.AccountActivity(ctx, q.Scope, q.ActorID, q.From, q.To.AddDate(0, 0, 1))
	if err != nil {
		return ProfitAndLoss{}, err
	}
	result := ProfitAndLoss{From: q.From, To: q.To, Currency: currency, Revenue: []StatementLine{}, Expenses: []StatementLine{}, GeneratedAt: s.clock.Now().UTC()}
	for _, a := range activities {
		switch a.Type {
		case financialops.AccountRevenue:
			amount := a.CreditMinor - a.DebitMinor
			if amount != 0 {
				result.Revenue = append(result.Revenue, statementLine(a, amount))
				result.TotalRevenueMinor += amount
			}
		case financialops.AccountExpense:
			amount := a.DebitMinor - a.CreditMinor
			if amount != 0 {
				result.Expenses = append(result.Expenses, statementLine(a, amount))
				result.TotalExpenseMinor += amount
			}
		case financialops.AccountUnclassified:
			result.UnclassifiedMinor += a.DebitMinor - a.CreditMinor
		}
	}
	result.NetProfitMinor = result.TotalRevenueMinor - result.TotalExpenseMinor
	if err = safe(result.TotalRevenueMinor, result.TotalExpenseMinor, result.NetProfitMinor, result.UnclassifiedMinor); err != nil {
		return ProfitAndLoss{}, err
	}
	return result, nil
}

func (s *Service) BalanceSheet(ctx context.Context, q Query) (BalanceSheet, error) {
	q, err := normalizeQuery(q, false, true)
	if err != nil {
		return BalanceSheet{}, err
	}
	activities, currency, err := s.repository.AccountActivity(ctx, q.Scope, q.ActorID, time.Time{}, q.AsOf.AddDate(0, 0, 1))
	if err != nil {
		return BalanceSheet{}, err
	}
	r := BalanceSheet{AsOf: q.AsOf, Currency: currency, Assets: []StatementLine{}, Liabilities: []StatementLine{}, Equity: []StatementLine{}, GeneratedAt: s.clock.Now().UTC()}
	for _, a := range activities {
		net := a.OpeningMinor + a.DebitMinor - a.CreditMinor
		switch a.Type {
		case financialops.AccountAsset:
			amount := net
			if amount != 0 {
				r.Assets = append(r.Assets, statementLine(a, amount))
				r.TotalAssetsMinor += amount
			}
		case financialops.AccountLiability:
			amount := -net
			if amount != 0 {
				r.Liabilities = append(r.Liabilities, statementLine(a, amount))
				r.TotalLiabilitiesMinor += amount
			}
		case financialops.AccountEquity:
			amount := -net
			if amount != 0 {
				r.Equity = append(r.Equity, statementLine(a, amount))
				r.TotalEquityMinor += amount
			}
		case financialops.AccountRevenue:
			r.CurrentEarningsMinor -= net
		case financialops.AccountExpense:
			r.CurrentEarningsMinor -= net
		case financialops.AccountUnclassified:
			r.UnclassifiedMinor += net
		}
	}
	r.TotalEquityMinor += r.CurrentEarningsMinor
	r.Balanced = r.UnclassifiedMinor == 0 && r.TotalAssetsMinor == r.TotalLiabilitiesMinor+r.TotalEquityMinor
	if err = safe(r.TotalAssetsMinor, r.TotalLiabilitiesMinor, r.TotalEquityMinor, r.CurrentEarningsMinor, r.UnclassifiedMinor); err != nil {
		return BalanceSheet{}, err
	}
	return r, nil
}

func statementLine(a AccountActivity, amount int64) StatementLine {
	return StatementLine{AccountID: a.AccountID, Code: a.Code, Name: a.Name, Type: a.Type, AmountMinor: amount}
}

func (s *Service) CashFlow(ctx context.Context, q Query) (CashFlow, error) {
	q, err := normalizeQuery(q, true, false)
	if err != nil {
		return CashFlow{}, err
	}
	opening, closing, movements, currency, err := s.repository.CashActivity(ctx, q.Scope, q.ActorID, q.From, q.To.AddDate(0, 0, 1))
	if err != nil {
		return CashFlow{}, err
	}
	r := CashFlow{From: q.From, To: q.To, Currency: currency, OpeningCashMinor: opening, Operating: CashFlowSection{Activity: "OPERATING", Lines: []CashMovement{}}, Investing: CashFlowSection{Activity: "INVESTING", Lines: []CashMovement{}}, Financing: CashFlowSection{Activity: "FINANCING", Lines: []CashMovement{}}, Unclassified: CashFlowSection{Activity: "UNCLASSIFIED", Lines: []CashMovement{}}, GeneratedAt: s.clock.Now().UTC()}
	for _, m := range movements {
		section := cashActivity(m.SourceType)
		switch section {
		case "OPERATING":
			r.Operating.Lines = append(r.Operating.Lines, m)
			r.Operating.NetMinor += m.AmountMinor
		case "INVESTING":
			r.Investing.Lines = append(r.Investing.Lines, m)
			r.Investing.NetMinor += m.AmountMinor
		case "FINANCING":
			r.Financing.Lines = append(r.Financing.Lines, m)
			r.Financing.NetMinor += m.AmountMinor
		default:
			r.Unclassified.Lines = append(r.Unclassified.Lines, m)
			r.Unclassified.NetMinor += m.AmountMinor
		}
	}
	r.NetChangeMinor = r.Operating.NetMinor + r.Investing.NetMinor + r.Financing.NetMinor + r.Unclassified.NetMinor
	r.ClosingCashMinor = closing
	r.Reconciled = r.OpeningCashMinor+r.NetChangeMinor == closing
	r.Classified = len(r.Unclassified.Lines) == 0
	if err = safe(r.OpeningCashMinor, r.NetChangeMinor, r.ClosingCashMinor); err != nil {
		return CashFlow{}, err
	}
	return r, nil
}

func cashActivity(source string) string {
	switch source {
	case "SALE", "REVERSAL", "CUSTOMER_COLLECTION", "SUPPLIER_PAYMENT", "PURCHASE_RETURN", "BANK_ADJUSTMENT":
		return "OPERATING"
	case "ASSET_PURCHASE", "ASSET_DISPOSAL":
		return "INVESTING"
	case "CAPITAL_CONTRIBUTION", "DIVIDEND", "BORROWING", "LOAN_REPAYMENT":
		return "FINANCING"
	case "STOCK_TRANSFER_DISPATCH", "STOCK_TRANSFER_RECEIPT", "CASH_TRANSFER":
		return "INTERNAL"
	default:
		return "UNCLASSIFIED"
	}
}

func (s *Service) Export(ctx context.Context, c ExportCommand) (ExportArtifact, error) {
	if c.Format == "" {
		c.Format = ExportCSV
	}
	if c.Format != ExportCSV && c.Format != ExportPDF && c.Format != ExportXLSX {
		return ExportArtifact{}, ErrInvalidQuery
	}
	data, label, err := s.report(ctx, c.Type, c.Query)
	if err != nil {
		return ExportArtifact{}, err
	}
	pack := ReportPack{ReportType: c.Type, Current: data, CurrentLabel: label}
	if !c.ComparisonFrom.IsZero() || !c.ComparisonTo.IsZero() || !c.ComparisonAsOf.IsZero() {
		comparison := c.Query
		comparison.From, comparison.To, comparison.AsOf = c.ComparisonFrom, c.ComparisonTo, c.ComparisonAsOf
		pack.Comparison, pack.ComparisonLabel, err = s.report(ctx, c.Type, comparison)
		if err != nil {
			return ExportArtifact{}, err
		}
	}
	if len(c.IdempotencyKey) < 16 || len(c.IdempotencyKey) > 128 {
		return ExportArtifact{}, ErrInvalidQuery
	}
	id, err := s.ids.New()
	if err != nil {
		return ExportArtifact{}, err
	}
	artifact, err := renderReport(id, pack, c.Format, s.clock.Now().UTC())
	if err != nil {
		return ExportArtifact{}, err
	}
	encoded, _ := json.Marshal(c)
	sum := sha256.Sum256(encoded)
	return s.repository.SaveReportExport(ctx, c.Scope.Normalize(), identity.NormalizeClaim(c.ActorID), artifact, c.IdempotencyKey, hex.EncodeToString(sum[:]))
}

func (s *Service) report(ctx context.Context, kind ReportType, query Query) (any, string, error) {
	switch kind {
	case ReportTrialBalance:
		v, e := s.TrialBalance(ctx, query)
		return v, v.AsOf.Format("2006-01-02"), e
	case ReportGeneralLedger:
		v, e := s.GeneralLedger(ctx, query)
		return v, v.From.Format("2006-01-02") + " to " + v.To.Format("2006-01-02"), e
	case ReportProfitAndLoss:
		v, e := s.ProfitAndLoss(ctx, query)
		return v, v.From.Format("2006-01-02") + " to " + v.To.Format("2006-01-02"), e
	case ReportBalanceSheet:
		v, e := s.BalanceSheet(ctx, query)
		return v, v.AsOf.Format("2006-01-02"), e
	case ReportCashFlow:
		v, e := s.CashFlow(ctx, query)
		return v, v.From.Format("2006-01-02") + " to " + v.To.Format("2006-01-02"), e
	default:
		return nil, "", ErrInvalidQuery
	}
}
