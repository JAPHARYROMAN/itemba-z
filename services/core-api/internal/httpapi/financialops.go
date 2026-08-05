package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
)

type financialLineRequest struct {
	AccountID   string `json:"account_id"`
	DebitMinor  int64  `json:"debit_minor"`
	CreditMinor int64  `json:"credit_minor"`
	Memo        string `json:"memo"`
}
type createFinancialDocumentRequest struct {
	Type               financialops.DocumentType `json:"type"`
	Currency           string                    `json:"currency"`
	AccountingAt       string                    `json:"accounting_at"`
	Reason             string                    `json:"reason"`
	FromAccountID      string                    `json:"from_account_id,omitempty"`
	ToAccountID        string                    `json:"to_account_id,omitempty"`
	ReversesDocumentID string                    `json:"reverses_document_id,omitempty"`
	Lines              []financialLineRequest    `json:"lines"`
}

func idempotency(writer http.ResponseWriter, r *http.Request) (string, bool) {
	v := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(v) < 16 || len(v) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return "", false
	}
	return v, true
}
func (h *Handler) listFinancialDocuments(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	limit, ok := parsePageSize(w, r)
	if !ok {
		return
	}
	v, e := h.financialops.Documents(r.Context(), p.Scope, p.ActorID, r.URL.Query().Get("cursor"), limit)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) getFinancialDocument(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, e := identity.CanonicalUUID(r.PathValue("documentID"))
	if e != nil {
		writeProblem(w, 400, "invalid_request", "documentID must be a UUID")
		return
	}
	v, e := h.financialops.Document(r.Context(), p.Scope, p.ActorID, id)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createFinancialDocument(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := idempotency(w, r)
	if !ok {
		return
	}
	var b createFinancialDocumentRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	at, e := time.Parse(time.RFC3339, b.AccountingAt)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "accounting_at must be RFC3339")
		return
	}
	lines := make([]finance.JournalEntry, len(b.Lines))
	for i, l := range b.Lines {
		lines[i] = finance.JournalEntry{AccountID: strings.TrimSpace(l.AccountID), DebitMinor: l.DebitMinor, CreditMinor: l.CreditMinor, Memo: strings.TrimSpace(l.Memo)}
	}
	v, e := h.financialops.Create(r.Context(), financialops.CreateCommand{Scope: p.Scope, Type: b.Type, Currency: b.Currency, AccountingAt: at, Reason: b.Reason, FromAccountID: b.FromAccountID, ToAccountID: b.ToAccountID, ReversesDocumentID: b.ReversesDocumentID, Lines: lines, ActorID: p.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(w)})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	w.Header().Set("Location", "/v1/finance/documents/"+v.ID)
	writeJSON(w, 201, v)
}

type transitionFinancialRequest struct {
	Status financialops.Status `json:"status"`
	Reason string              `json:"reason"`
}

func (h *Handler) transitionFinancialDocument(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := idempotency(w, r)
	if !ok {
		return
	}
	id, e := identity.CanonicalUUID(r.PathValue("documentID"))
	if e != nil {
		writeProblem(w, 400, "invalid_request", "documentID must be a UUID")
		return
	}
	var b transitionFinancialRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.financialops.Transition(r.Context(), financialops.TransitionCommand{Scope: p.Scope, DocumentID: id, Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(w)})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) listFiscalPeriods(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.financialops.Periods(r.Context(), p.Scope, p.ActorID)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]any{"items": v})
}
func (h *Handler) listFiscalPeriodActions(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.financialops.PeriodActions(r.Context(), p.Scope, p.ActorID)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type periodActionRequest struct {
	Action financialops.PeriodAction `json:"action"`
	Reason string                    `json:"reason"`
}

func (h *Handler) requestFiscalPeriodAction(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := idempotency(w, r)
	if !ok {
		return
	}
	id, e := identity.CanonicalUUID(r.PathValue("periodID"))
	if e != nil {
		writeProblem(w, 400, "invalid_request", "periodID must be a UUID")
		return
	}
	var b periodActionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.financialops.RequestPeriodAction(r.Context(), financialops.PeriodCommand{Scope: p.Scope, PeriodID: id, Action: b.Action, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem, CorrelationID: correlationID(w)})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}

type approvalRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) approveFiscalPeriodAction(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := idempotency(w, r)
	if !ok {
		return
	}
	id, e := identity.CanonicalUUID(r.PathValue("actionID"))
	if e != nil {
		writeProblem(w, 400, "invalid_request", "actionID must be a UUID")
		return
	}
	var b approvalRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.financialops.ApprovePeriodAction(r.Context(), p.Scope, p.ActorID, id, b.Reason, idem)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
