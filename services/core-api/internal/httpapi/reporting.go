package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
)

func reportDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}
func (h *Handler) reportPrincipal(w http.ResponseWriter, r *http.Request) (reporting.Query, bool) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return reporting.Query{}, false
	}
	return reporting.Query{Scope: p.Scope, ActorID: p.ActorID}, true
}

func (h *Handler) trialBalance(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok {
		return
	}
	asOf, err := reportDate(r.URL.Query().Get("as_of"))
	if err != nil {
		writeProblem(w, 400, "invalid_report_query", "as_of must use YYYY-MM-DD")
		return
	}
	q.AsOf = asOf
	value, err := h.reporting.TrialBalance(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, value)
}
func reportRange(w http.ResponseWriter, r *http.Request, q *reporting.Query) bool {
	from, err := reportDate(r.URL.Query().Get("from"))
	if err != nil {
		writeProblem(w, 400, "invalid_report_query", "from must use YYYY-MM-DD")
		return false
	}
	to, err := reportDate(r.URL.Query().Get("to"))
	if err != nil {
		writeProblem(w, 400, "invalid_report_query", "to must use YYYY-MM-DD")
		return false
	}
	q.From, q.To = from, to
	return true
}

func optionalDashboardRange(w http.ResponseWriter, r *http.Request, q *reporting.Query) bool {
	from, to := strings.TrimSpace(r.URL.Query().Get("from")), strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" && to == "" {
		return true
	}
	if from == "" || to == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_report_query", "from and to must be provided together")
		return false
	}
	var err error
	q.From, err = reportDate(from)
	if err == nil {
		q.To, err = reportDate(to)
	}
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_report_query", "from and to must use YYYY-MM-DD")
		return false
	}
	return true
}

func dashboardScopeMatches(w http.ResponseWriter, r *http.Request, q reporting.Query) bool {
	checks := []struct {
		name     string
		expected string
	}{
		{"legal_company_id", q.Scope.CompanyID},
		{"branch_id", q.Scope.BranchID},
	}
	for _, check := range checks {
		value := strings.TrimSpace(r.URL.Query().Get(check.name))
		if value == "" {
			continue
		}
		canonical, err := identity.CanonicalUUID(value)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_report_query", check.name+" must be a UUID")
			return false
		}
		if canonical != check.expected {
			writeProblem(w, http.StatusForbidden, "scope_mismatch", "dashboard scope must match the authenticated organizational scope")
			return false
		}
	}
	return true
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok || !dashboardScopeMatches(w, r, q) || !optionalDashboardRange(w, r, &q) {
		return
	}
	value, err := h.reporting.Dashboard(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *Handler) generalLedger(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok || !reportRange(w, r, &q) {
		return
	}
	q.AccountID = r.URL.Query().Get("account_id")
	value, err := h.reporting.GeneralLedger(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, value)
}
func (h *Handler) profitAndLoss(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok || !reportRange(w, r, &q) {
		return
	}
	value, err := h.reporting.ProfitAndLoss(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, value)
}
func (h *Handler) balanceSheet(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok {
		return
	}
	asOf, err := reportDate(r.URL.Query().Get("as_of"))
	if err != nil {
		writeProblem(w, 400, "invalid_report_query", "as_of must use YYYY-MM-DD")
		return
	}
	q.AsOf = asOf
	value, err := h.reporting.BalanceSheet(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, value)
}
func (h *Handler) cashFlow(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok || !reportRange(w, r, &q) {
		return
	}
	value, err := h.reporting.CashFlow(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, value)
}

type exportFinancialReportRequest struct {
	ReportType     reporting.ReportType   `json:"report_type"`
	Format         reporting.ExportFormat `json:"format,omitempty"`
	From           string                 `json:"from,omitempty"`
	To             string                 `json:"to,omitempty"`
	AsOf           string                 `json:"as_of,omitempty"`
	ComparisonFrom string                 `json:"comparison_from,omitempty"`
	ComparisonTo   string                 `json:"comparison_to,omitempty"`
	ComparisonAsOf string                 `json:"comparison_as_of,omitempty"`
	AccountID      string                 `json:"account_id,omitempty"`
}

func (h *Handler) exportFinancialReport(w http.ResponseWriter, r *http.Request) {
	q, ok := h.reportPrincipal(w, r)
	if !ok {
		return
	}
	key, ok := idempotency(w, r)
	if !ok {
		return
	}
	var body exportFinancialReportRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	var err error
	if body.From != "" {
		q.From, err = reportDate(body.From)
		if err != nil {
			writeProblem(w, 400, "invalid_report_query", "from must use YYYY-MM-DD")
			return
		}
	}
	if body.To != "" {
		q.To, err = reportDate(body.To)
		if err != nil {
			writeProblem(w, 400, "invalid_report_query", "to must use YYYY-MM-DD")
			return
		}
	}
	if body.AsOf != "" {
		q.AsOf, err = reportDate(body.AsOf)
		if err != nil {
			writeProblem(w, 400, "invalid_report_query", "as_of must use YYYY-MM-DD")
			return
		}
	}
	parseComparison := func(value string) (time.Time, bool) {
		if value == "" {
			return time.Time{}, true
		}
		parsed, parseErr := reportDate(value)
		if parseErr != nil {
			writeProblem(w, 400, "invalid_report_query", "comparison dates must use YYYY-MM-DD")
			return time.Time{}, false
		}
		return parsed, true
	}
	comparisonFrom, valid := parseComparison(body.ComparisonFrom)
	if !valid {
		return
	}
	comparisonTo, valid := parseComparison(body.ComparisonTo)
	if !valid {
		return
	}
	comparisonAsOf, valid := parseComparison(body.ComparisonAsOf)
	if !valid {
		return
	}
	q.AccountID = body.AccountID
	value, err := h.reporting.Export(r.Context(), reporting.ExportCommand{Query: q, Type: body.ReportType, Format: body.Format, ComparisonFrom: comparisonFrom, ComparisonTo: comparisonTo, ComparisonAsOf: comparisonAsOf, IdempotencyKey: key})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
