package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
)

type budgetLineRequest struct {
	AccountID   string `json:"account_id"`
	Month       string `json:"month"`
	AmountMinor int64  `json:"amount_minor"`
}
type createBudgetRequest struct {
	Name       string              `json:"name"`
	FiscalYear int                 `json:"fiscal_year"`
	Currency   string              `json:"currency"`
	Reason     string              `json:"reason"`
	Lines      []budgetLineRequest `json:"lines"`
}
type transitionAdvancedRequest struct {
	Status advancedfinance.Status `json:"status"`
	Reason string                 `json:"reason"`
}
type createAssetRequest struct {
	Code                             string `json:"code"`
	Name                             string `json:"name"`
	Category                         string `json:"category"`
	Currency                         string `json:"currency"`
	AcquiredAt                       string `json:"acquired_at"`
	CostMinor                        int64  `json:"cost_minor"`
	ResidualMinor                    int64  `json:"residual_minor"`
	UsefulLifeMonths                 int    `json:"useful_life_months"`
	AssetAccountID                   string `json:"asset_account_id"`
	AccumulatedDepreciationAccountID string `json:"accumulated_depreciation_account_id"`
	DepreciationExpenseAccountID     string `json:"depreciation_expense_account_id"`
	CapitalizationOffsetAccountID    string `json:"capitalization_offset_account_id"`
	DisposalGainAccountID            string `json:"disposal_gain_account_id"`
	DisposalLossAccountID            string `json:"disposal_loss_account_id"`
	Reason                           string `json:"reason"`
}
type createPurchasedAssetRequest struct {
	SourceDocumentID                 string `json:"source_document_id"`
	SourceProductID                  string `json:"source_product_id"`
	Code                             string `json:"code"`
	Name                             string `json:"name"`
	Category                         string `json:"category"`
	Currency                         string `json:"currency"`
	ResidualMinor                    int64  `json:"residual_minor"`
	UsefulLifeMonths                 int    `json:"useful_life_months"`
	AssetAccountID                   string `json:"asset_account_id"`
	AccumulatedDepreciationAccountID string `json:"accumulated_depreciation_account_id"`
	DepreciationExpenseAccountID     string `json:"depreciation_expense_account_id"`
	CapitalizationOffsetAccountID    string `json:"capitalization_offset_account_id"`
	DisposalGainAccountID            string `json:"disposal_gain_account_id"`
	DisposalLossAccountID            string `json:"disposal_loss_account_id"`
	Reason                           string `json:"reason"`
}
type depreciationRequest struct {
	Period string `json:"period"`
	Reason string `json:"reason"`
}
type disposalRequest struct {
	ProceedsMinor     int64  `json:"proceeds_minor"`
	ProceedsAccountID string `json:"proceeds_account_id"`
	Reason            string `json:"reason"`
}

func advancedID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id, err := identity.CanonicalUUID(r.PathValue(name))
	if err != nil {
		writeProblem(w, 400, "invalid_request", name+" must be a UUID")
		return "", false
	}
	return id, true
}
func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, err := h.advancedfinance.Budgets(r.Context(), p.Scope, p.ActorID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body createBudgetRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	c := advancedfinance.CreateBudgetCommand{Scope: p.Scope, Name: body.Name, FiscalYear: body.FiscalYear, Currency: body.Currency, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem}
	for _, l := range body.Lines {
		m, err := time.Parse("2006-01-02", strings.TrimSpace(l.Month))
		if err != nil {
			writeProblem(w, 400, "invalid_request", "budget line month must use YYYY-MM-01")
			return
		}
		c.Lines = append(c.Lines, advancedfinance.BudgetLine{AccountID: l.AccountID, Month: m, AmountMinor: l.AmountMinor})
	}
	v, err := h.advancedfinance.CreateBudget(r.Context(), c)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionBudget(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "budgetID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body transitionAdvancedRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	v, err := h.advancedfinance.TransitionBudget(r.Context(), advancedfinance.TransitionCommand{Scope: p.Scope, ID: id, Status: body.Status, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) budgetActual(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "budgetID")
	if !ok {
		return
	}
	v, err := h.advancedfinance.BudgetActual(r.Context(), p.Scope, p.ActorID, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) listAssets(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, err := h.advancedfinance.Assets(r.Context(), p.Scope, p.ActorID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createAsset(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body createAssetRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	acquired, err := time.Parse("2006-01-02", strings.TrimSpace(body.AcquiredAt))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "acquired_at must use YYYY-MM-DD")
		return
	}
	v, err := h.advancedfinance.CreateAsset(r.Context(), advancedfinance.CreateAssetCommand{Scope: p.Scope, Code: body.Code, Name: body.Name, Category: body.Category, Currency: body.Currency, AcquiredAt: acquired, CostMinor: body.CostMinor, ResidualMinor: body.ResidualMinor, UsefulLifeMonths: body.UsefulLifeMonths, AssetAccountID: body.AssetAccountID, AccumulatedDepreciationAccountID: body.AccumulatedDepreciationAccountID, DepreciationExpenseAccountID: body.DepreciationExpenseAccountID, CapitalizationOffsetAccountID: body.CapitalizationOffsetAccountID, DisposalGainAccountID: body.DisposalGainAccountID, DisposalLossAccountID: body.DisposalLossAccountID, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) createPurchasedAsset(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b createPurchasedAssetRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, err := h.advancedfinance.CreatePurchasedAsset(r.Context(), advancedfinance.CreatePurchasedAssetCommand{Scope: p.Scope, SourceDocumentID: b.SourceDocumentID, SourceProductID: b.SourceProductID, Code: b.Code, Name: b.Name, Category: b.Category, Currency: b.Currency, ResidualMinor: b.ResidualMinor, UsefulLifeMonths: b.UsefulLifeMonths, AssetAccountID: b.AssetAccountID, AccumulatedDepreciationAccountID: b.AccumulatedDepreciationAccountID, DepreciationExpenseAccountID: b.DepreciationExpenseAccountID, CapitalizationOffsetAccountID: b.CapitalizationOffsetAccountID, DisposalGainAccountID: b.DisposalGainAccountID, DisposalLossAccountID: b.DisposalLossAccountID, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionAsset(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "assetID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body transitionAdvancedRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	v, err := h.advancedfinance.TransitionAsset(r.Context(), advancedfinance.TransitionCommand{Scope: p.Scope, ID: id, Status: body.Status, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) depreciateAsset(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "assetID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body depreciationRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	period, err := time.Parse("2006-01-02", strings.TrimSpace(body.Period))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "period must use YYYY-MM-01")
		return
	}
	v, err := h.advancedfinance.Depreciate(r.Context(), advancedfinance.DepreciateCommand{Scope: p.Scope, AssetID: id, Period: period, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) disposeAsset(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "assetID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body disposalRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	v, err := h.advancedfinance.Dispose(r.Context(), advancedfinance.DisposeCommand{Scope: p.Scope, AssetID: id, ProceedsMinor: body.ProceedsMinor, ProceedsAccountID: body.ProceedsAccountID, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
