package httpapi

import (
	"encoding/json"
	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"net/http"
	"strings"
	"time"
)

type configurationRequest struct {
	Category      configuration.Category `json:"category"`
	Key           string                 `json:"key"`
	NameEN        string                 `json:"name_en"`
	NameSW        string                 `json:"name_sw"`
	Value         json.RawMessage        `json:"value"`
	SecretRef     string                 `json:"secret_ref"`
	EffectiveFrom string                 `json:"effective_from"`
	EffectiveTo   string                 `json:"effective_to"`
	Reason        string                 `json:"reason"`
}
type configurationTransitionRequest struct {
	Status configuration.Status `json:"status"`
	Reason string               `json:"reason"`
}
type sequenceRequest struct {
	Key       string `json:"key"`
	Prefix    string `json:"prefix"`
	NextValue int64  `json:"next_value"`
	Padding   int64  `json:"padding"`
}

func (h *Handler) configurationSnapshot(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.configuration.Get(r.Context(), p.Scope, p.ActorID)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createConfiguration(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b configurationRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	from, e := time.Parse("2006-01-02", strings.TrimSpace(b.EffectiveFrom))
	if e != nil {
		writeProblem(w, 400, "invalid_request", "effective_from must use YYYY-MM-DD")
		return
	}
	var to *time.Time
	if strings.TrimSpace(b.EffectiveTo) != "" {
		x, e := time.Parse("2006-01-02", strings.TrimSpace(b.EffectiveTo))
		if e != nil {
			writeProblem(w, 400, "invalid_request", "effective_to must use YYYY-MM-DD")
			return
		}
		to = &x
	}
	v, e := h.configuration.Create(r.Context(), configuration.CreateCommand{Scope: p.Scope, Category: b.Category, Key: b.Key, NameEN: b.NameEN, NameSW: b.NameSW, Value: b.Value, SecretRef: b.SecretRef, EffectiveFrom: from, EffectiveTo: to, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionConfiguration(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b configurationTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.configuration.Transition(r.Context(), configuration.TransitionCommand{Scope: p.Scope, ID: r.PathValue("configurationID"), Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createNumberSequence(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b sequenceRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.configuration.CreateSequence(r.Context(), configuration.SequenceCommand{Scope: p.Scope, Key: b.Key, Prefix: b.Prefix, NextValue: b.NextValue, Padding: b.Padding, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) allocateNumber(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	v, e := h.configuration.Allocate(r.Context(), configuration.AllocateCommand{Scope: p.Scope, SequenceID: r.PathValue("sequenceID"), ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
