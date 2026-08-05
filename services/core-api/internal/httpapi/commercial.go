package httpapi

import (
	"net/http"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/commercial"
)

type revisionRequest struct {
	EntityType commercial.EntityType    `json:"entity_type"`
	EntityID   string                   `json:"entity_id"`
	Supplier   *commercial.SupplierData `json:"supplier"`
	Product    *commercial.ProductData  `json:"product"`
	Reason     string                   `json:"reason"`
}
type revisionTransitionRequest struct {
	Status commercial.Status `json:"status"`
	Reason string            `json:"reason"`
}
type rfqRequest struct {
	Currency      string                      `json:"currency"`
	ResponseDueAt string                      `json:"response_due_at"`
	Reason        string                      `json:"reason"`
	Lines         []commercial.RFQCommandLine `json:"lines"`
}
type rfqTransitionRequest struct {
	Status commercial.RFQStatus `json:"status"`
	Reason string               `json:"reason"`
}
type quoteRequest struct {
	RFQID            string                        `json:"rfq_id"`
	SupplierID       string                        `json:"supplier_id"`
	Reference        string                        `json:"reference"`
	Currency         string                        `json:"currency"`
	DeliveryDays     int64                         `json:"delivery_days"`
	PaymentTermsDays int64                         `json:"payment_terms_days"`
	ValidUntil       string                        `json:"valid_until"`
	Reason           string                        `json:"reason"`
	Lines            []commercial.QuoteCommandLine `json:"lines"`
}
type reasonRequest struct {
	Reason string `json:"reason"`
}
type awardRequest struct {
	QuoteID string `json:"quote_id"`
	Reason  string `json:"reason"`
}

func (h *Handler) commercialWorkspace(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.commercial.Workspace(r.Context(), p.Scope, p.ActorID)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createMasterRevision(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b revisionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.commercial.CreateRevision(r.Context(), commercial.RevisionCommand{Scope: p.Scope, EntityType: b.EntityType, EntityID: b.EntityID, Supplier: b.Supplier, Product: b.Product, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionMasterRevision(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b revisionTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.commercial.TransitionRevision(r.Context(), commercial.RevisionTransitionCommand{Scope: p.Scope, RevisionID: r.PathValue("revisionID"), Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createRFQ(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b rfqRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	due, e := time.Parse(time.RFC3339, b.ResponseDueAt)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "response_due_at must be RFC 3339")
		return
	}
	v, e := h.commercial.CreateRFQ(r.Context(), commercial.RFQCommand{Scope: p.Scope, Currency: b.Currency, ResponseDueAt: due, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem, Lines: b.Lines})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionRFQ(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b rfqTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.commercial.TransitionRFQ(r.Context(), commercial.RFQTransitionCommand{Scope: p.Scope, RFQID: r.PathValue("rfqID"), Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createSupplierQuote(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b quoteRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	valid, e := time.Parse(time.RFC3339, b.ValidUntil)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "valid_until must be RFC 3339")
		return
	}
	v, e := h.commercial.CreateQuote(r.Context(), commercial.QuoteCommand{Scope: p.Scope, RFQID: b.RFQID, SupplierID: b.SupplierID, Reference: b.Reference, Currency: b.Currency, DeliveryDays: b.DeliveryDays, PaymentTermsDays: b.PaymentTermsDays, ValidUntil: valid, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem, Lines: b.Lines})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) submitSupplierQuote(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b reasonRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.commercial.SubmitQuote(r.Context(), commercial.QuoteSubmitCommand{Scope: p.Scope, QuoteID: r.PathValue("quoteID"), Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) rfqComparison(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.commercial.Comparison(r.Context(), p.Scope, p.ActorID, r.PathValue("rfqID"))
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) awardRFQ(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b awardRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.commercial.Award(r.Context(), commercial.AwardCommand{Scope: p.Scope, RFQID: r.PathValue("rfqID"), QuoteID: b.QuoteID, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
