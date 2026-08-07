package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

type createFacilityRequest struct {
	Reference                 string                `json:"reference"`
	Lender                    string                `json:"lender"`
	Type                      treasury.FacilityType `json:"type"`
	Currency                  string                `json:"currency"`
	LimitMinor                int64                 `json:"limit_minor"`
	AnnualInterestBasisPoints int64                 `json:"annual_interest_basis_points"`
	StartDate                 string                `json:"start_date"`
	MaturityDate              string                `json:"maturity_date"`
	BankAccountID             string                `json:"bank_account_id"`
	PrincipalAccountID        string                `json:"principal_account_id"`
	InterestExpenseAccountID  string                `json:"interest_expense_account_id"`
	AccruedInterestAccountID  string                `json:"accrued_interest_account_id"`
	Reason                    string                `json:"reason"`
}
type treasuryTransitionRequest struct {
	Status treasury.Status `json:"status"`
	Reason string          `json:"reason"`
}
type treasuryTransactionRequest struct {
	Type        treasury.TransactionType `json:"type"`
	AmountMinor int64                    `json:"amount_minor"`
	OccurredAt  string                   `json:"occurred_at"`
	Reason      string                   `json:"reason"`
}

func (h *Handler) listTreasuryFacilities(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, err := h.treasury.Facilities(r.Context(), p.Scope, p.ActorID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createTreasuryFacility(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b createFacilityRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	start, err := time.Parse("2006-01-02", strings.TrimSpace(b.StartDate))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "start_date must use YYYY-MM-DD")
		return
	}
	maturity, err := time.Parse("2006-01-02", strings.TrimSpace(b.MaturityDate))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "maturity_date must use YYYY-MM-DD")
		return
	}
	v, err := h.treasury.CreateFacility(r.Context(), treasury.CreateFacilityCommand{Scope: p.Scope, Reference: b.Reference, Lender: b.Lender, Type: b.Type, Currency: b.Currency, LimitMinor: b.LimitMinor, AnnualInterestBasisPoints: b.AnnualInterestBasisPoints, StartDate: start, MaturityDate: maturity, BankAccountID: b.BankAccountID, PrincipalAccountID: b.PrincipalAccountID, InterestExpenseAccountID: b.InterestExpenseAccountID, AccruedInterestAccountID: b.AccruedInterestAccountID, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionTreasuryFacility(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "facilityID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b treasuryTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, err := h.treasury.TransitionFacility(r.Context(), treasury.TransitionCommand{Scope: p.Scope, FacilityID: id, Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) postTreasuryTransaction(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	id, ok := advancedID(w, r, "facilityID")
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b treasuryTransactionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	occurred, err := time.Parse(time.RFC3339, strings.TrimSpace(b.OccurredAt))
	if err != nil {
		writeProblem(w, 400, "invalid_request", "occurred_at must use RFC3339")
		return
	}
	v, err := h.treasury.PostTransaction(r.Context(), treasury.PostTransactionCommand{Scope: p.Scope, FacilityID: id, Type: b.Type, AmountMinor: b.AmountMinor, OccurredAt: occurred, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, 201, v)
}
