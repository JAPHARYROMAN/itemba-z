package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
)

type createIntercompanyRequest struct {
	CounterpartyCompanyID       string                       `json:"counterparty_company_id"`
	Reference                   string                       `json:"reference"`
	Type                        groupfinance.TransactionType `json:"type"`
	Currency                    string                       `json:"currency"`
	AmountMinor                 int64                        `json:"amount_minor"`
	OccurredAt                  string                       `json:"occurred_at"`
	SourceDebitAccountID        string                       `json:"source_debit_account_id"`
	SourceCreditAccountID       string                       `json:"source_credit_account_id"`
	CounterpartyDebitAccountID  string                       `json:"counterparty_debit_account_id"`
	CounterpartyCreditAccountID string                       `json:"counterparty_credit_account_id"`
	Reason                      string                       `json:"reason"`
}
type groupTransitionRequest struct {
	Status groupfinance.Status `json:"status"`
	Reason string              `json:"reason"`
}

func (h *Handler) listIntercompany(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, err := h.groupfinance.Transactions(r.Context(), p.Scope, p.ActorID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createIntercompany(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b createIntercompanyRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	occurred, err := time.Parse(time.RFC3339, strings.TrimSpace(b.OccurredAt))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "occurred_at must use RFC3339")
		return
	}
	v, err := h.groupfinance.Create(r.Context(), groupfinance.CreateCommand{Scope: p.Scope, CounterpartyCompanyID: b.CounterpartyCompanyID, Reference: b.Reference, Type: b.Type, Currency: b.Currency, AmountMinor: b.AmountMinor, OccurredAt: occurred, SourceDebitAccountID: b.SourceDebitAccountID, SourceCreditAccountID: b.SourceCreditAccountID, CounterpartyDebitAccountID: b.CounterpartyDebitAccountID, CounterpartyCreditAccountID: b.CounterpartyCreditAccountID, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionIntercompany(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "transactionID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b groupTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, err := h.groupfinance.Transition(r.Context(), groupfinance.TransitionCommand{Scope: p.Scope, TransactionID: id, Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) consolidation(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	asOf, err := time.Parse(time.RFC3339, strings.TrimSpace(r.URL.Query().Get("as_of")))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "as_of must use RFC3339")
		return
	}
	v, err := h.groupfinance.Consolidated(r.Context(), p.Scope, p.ActorID, asOf)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
