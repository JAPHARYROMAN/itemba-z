package httpapi

import (
	"net/http"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/inventorycontrol"
)

type inventoryPolicyRequest struct {
	ProductID           string                      `json:"product_id"`
	CostMethod          inventorycontrol.CostMethod `json:"cost_method"`
	LotControlled       bool                        `json:"lot_controlled"`
	ReorderPoint        int64                       `json:"reorder_point"`
	ReorderQuantity     int64                       `json:"reorder_quantity"`
	MaximumStock        int64                       `json:"maximum_stock"`
	SafetyStock         int64                       `json:"safety_stock"`
	LeadTimeDays        int64                       `json:"lead_time_days"`
	PreferredSupplierID string                      `json:"preferred_supplier_id"`
	Reason              string                      `json:"reason"`
}

type inventoryPolicyTransitionRequest struct {
	Status inventorycontrol.PolicyStatus `json:"status"`
	Reason string                        `json:"reason"`
}

type receiptLotLineRequest struct {
	ProductID      string `json:"product_id"`
	LotNumber      string `json:"lot_number"`
	Quantity       int64  `json:"quantity"`
	ManufacturedAt string `json:"manufactured_at"`
	ExpiresAt      string `json:"expires_at"`
}

type receiptLotsRequest struct {
	Reason string                  `json:"reason"`
	Lines  []receiptLotLineRequest `json:"lines"`
}

func (h *Handler) inventoryControlWorkspace(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, err := h.inventorycontrol.Workspace(r.Context(), p.Scope, p.ActorID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) createInventoryPolicy(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b inventoryPolicyRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, err := h.inventorycontrol.CreatePolicy(r.Context(), inventorycontrol.PolicyCommand{Scope: p.Scope, ProductID: b.ProductID, CostMethod: b.CostMethod, LotControlled: b.LotControlled, ReorderPoint: b.ReorderPoint, ReorderQuantity: b.ReorderQuantity, MaximumStock: b.MaximumStock, SafetyStock: b.SafetyStock, LeadTimeDays: b.LeadTimeDays, PreferredSupplierID: b.PreferredSupplierID, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *Handler) transitionInventoryPolicy(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b inventoryPolicyTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, err := h.inventorycontrol.TransitionPolicy(r.Context(), inventorycontrol.PolicyTransitionCommand{Scope: p.Scope, PolicyID: r.PathValue("policyID"), Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) registerReceiptLots(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b receiptLotsRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	lines := make([]inventorycontrol.LotCommandLine, 0, len(b.Lines))
	for _, line := range b.Lines {
		manufacturedAt, err := optionalRFC3339(line.ManufacturedAt)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "manufactured_at must be RFC 3339")
			return
		}
		expiresAt, err := optionalRFC3339(line.ExpiresAt)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "expires_at must be RFC 3339")
			return
		}
		lines = append(lines, inventorycontrol.LotCommandLine{ProductID: line.ProductID, LotNumber: line.LotNumber, Quantity: line.Quantity, ManufacturedAt: manufacturedAt, ExpiresAt: expiresAt})
	}
	v, err := h.inventorycontrol.RegisterLots(r.Context(), inventorycontrol.LotCommand{Scope: p.Scope, GoodsReceiptID: r.PathValue("receiptID"), Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem, Lines: lines})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func optionalRFC3339(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	v, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
