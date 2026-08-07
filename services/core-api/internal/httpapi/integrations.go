package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/integrations"
)

type integrationRouteRequest struct {
	Capability              integrations.Capability `json:"capability"`
	ProviderCode            string                  `json:"provider_code"`
	ContractVersion         string                  `json:"contract_version"`
	EndpointURL             string                  `json:"endpoint_url"`
	SecretReference         string                  `json:"secret_reference"`
	TimeoutMilliseconds     int                     `json:"timeout_milliseconds"`
	MaxAttempts             int                     `json:"max_attempts"`
	BaseBackoffSeconds      int                     `json:"base_backoff_seconds"`
	MaxBackoffSeconds       int                     `json:"max_backoff_seconds"`
	CircuitFailureThreshold int                     `json:"circuit_failure_threshold"`
	CircuitOpenSeconds      int                     `json:"circuit_open_seconds"`
	ValidFrom               string                  `json:"valid_from"`
	ValidUntil              string                  `json:"valid_until"`
	Reason                  string                  `json:"reason"`
}
type integrationTransitionRequest struct {
	Status integrations.RouteStatus `json:"status"`
	Reason string                   `json:"reason"`
}
type integrationReasonRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) integrationWorkspace(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	value, err := h.integrations.Workspace(r.Context(), p.Scope, p.ActorID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *Handler) createIntegrationRoute(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body integrationRouteRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	from, err := time.Parse(time.RFC3339, strings.TrimSpace(body.ValidFrom))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "valid_from must use RFC 3339")
		return
	}
	var until *time.Time
	if strings.TrimSpace(body.ValidUntil) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(body.ValidUntil))
		if parseErr != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "valid_until must use RFC 3339")
			return
		}
		until = &parsed
	}
	value, err := h.integrations.CreateRoute(r.Context(), integrations.CreateRouteCommand{Scope: p.Scope, Capability: body.Capability, ProviderCode: body.ProviderCode, ContractVersion: body.ContractVersion, EndpointURL: body.EndpointURL, SecretReference: body.SecretReference, TimeoutMilliseconds: body.TimeoutMilliseconds, MaxAttempts: body.MaxAttempts, BaseBackoffSeconds: body.BaseBackoffSeconds, MaxBackoffSeconds: body.MaxBackoffSeconds, CircuitFailureThreshold: body.CircuitFailureThreshold, CircuitOpenSeconds: body.CircuitOpenSeconds, ValidFrom: from, ValidUntil: until, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (h *Handler) transitionIntegrationRoute(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body integrationTransitionRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	value, err := h.integrations.TransitionRoute(r.Context(), integrations.TransitionRouteCommand{Scope: p.Scope, RouteID: r.PathValue("routeID"), Status: body.Status, Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *Handler) replayIntegrationDelivery(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body integrationReasonRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	value, err := h.integrations.Replay(r.Context(), integrations.ReplayCommand{Scope: p.Scope, DeliveryID: r.PathValue("deliveryID"), Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}
func (h *Handler) resetIntegrationCircuit(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var body integrationReasonRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	value, err := h.integrations.ResetCircuit(r.Context(), integrations.ResetCircuitCommand{Scope: p.Scope, RouteID: r.PathValue("routeID"), Reason: body.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
