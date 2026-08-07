package httpapi

import (
	"net/http"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/workforce"
)

type shiftTemplateRequest struct {
	Code          string `json:"code"`
	NameEN        string `json:"name_en"`
	NameSW        string `json:"name_sw"`
	StartMinute   int64  `json:"start_minute"`
	EndMinute     int64  `json:"end_minute"`
	BreakMinutes  int64  `json:"break_minutes"`
	WeekdayMask   int64  `json:"weekday_mask"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
	Reason        string `json:"reason"`
}
type shiftAssignmentRequest struct {
	EmployeeID      string `json:"employee_id"`
	ShiftTemplateID string `json:"shift_template_id"`
	StartsOn        string `json:"starts_on"`
	EndsOn          string `json:"ends_on"`
	Reason          string `json:"reason"`
}
type workforceTransitionRequest struct {
	Status workforce.Status `json:"status"`
	Reason string           `json:"reason"`
}
type employeeDocumentRequest struct {
	EmployeeID     string                   `json:"employee_id"`
	DocumentType   string                   `json:"document_type"`
	Title          string                   `json:"title"`
	ObjectKey      string                   `json:"object_key"`
	SHA256         string                   `json:"sha256"`
	MediaType      string                   `json:"media_type"`
	Classification workforce.Classification `json:"classification"`
	IssuedOn       string                   `json:"issued_on"`
	ExpiresOn      string                   `json:"expires_on"`
	Reason         string                   `json:"reason"`
}
type payrollExportRequest struct {
	ConfigurationID string                 `json:"configuration_id"`
	Format          workforce.ExportFormat `json:"format"`
}

func optionalPeopleDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	v, e := parsePeopleDate(value)
	if e != nil {
		return nil, e
	}
	return &v, nil
}
func (h *Handler) workforceSnapshot(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.workforce.Get(r.Context(), p.Scope, p.ActorID)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) createShiftTemplate(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b shiftTemplateRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	from, e := parsePeopleDate(b.EffectiveFrom)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "effective_from must use YYYY-MM-DD")
		return
	}
	to, e := optionalPeopleDate(b.EffectiveTo)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "effective_to must use YYYY-MM-DD")
		return
	}
	v, e := h.workforce.CreateShift(r.Context(), workforce.ShiftCommand{Scope: p.Scope, Code: b.Code, NameEN: b.NameEN, NameSW: b.NameSW, StartMinute: b.StartMinute, EndMinute: b.EndMinute, BreakMinutes: b.BreakMinutes, WeekdayMask: b.WeekdayMask, EffectiveFrom: from, EffectiveTo: to, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionShiftTemplate(w http.ResponseWriter, r *http.Request) {
	h.transitionWorkforce(w, r, r.PathValue("templateID"), true)
}
func (h *Handler) transitionShiftAssignment(w http.ResponseWriter, r *http.Request) {
	h.transitionWorkforce(w, r, r.PathValue("assignmentID"), false)
}
func (h *Handler) transitionWorkforce(w http.ResponseWriter, r *http.Request, id string, template bool) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b workforceTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	command := workforce.TransitionCommand{Scope: p.Scope, ID: id, Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem}
	var v any
	var e error
	if template {
		v, e = h.workforce.TransitionShift(r.Context(), command)
	} else {
		v, e = h.workforce.TransitionAssignment(r.Context(), command)
	}
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createShiftAssignment(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b shiftAssignmentRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	start, e := parsePeopleDate(b.StartsOn)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "starts_on must use YYYY-MM-DD")
		return
	}
	end, e := parsePeopleDate(b.EndsOn)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "ends_on must use YYYY-MM-DD")
		return
	}
	v, e := h.workforce.CreateAssignment(r.Context(), workforce.AssignmentCommand{Scope: p.Scope, EmployeeID: b.EmployeeID, ShiftTemplateID: b.ShiftTemplateID, StartsOn: start, EndsOn: end, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) registerEmployeeDocument(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b employeeDocumentRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	issued, e := optionalPeopleDate(b.IssuedOn)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "issued_on must use YYYY-MM-DD")
		return
	}
	expires, e := optionalPeopleDate(b.ExpiresOn)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "expires_on must use YYYY-MM-DD")
		return
	}
	v, e := h.workforce.RegisterDocument(r.Context(), workforce.DocumentCommand{Scope: p.Scope, EmployeeID: b.EmployeeID, DocumentType: b.DocumentType, Title: b.Title, ObjectKey: b.ObjectKey, SHA256: b.SHA256, MediaType: b.MediaType, Classification: b.Classification, IssuedOn: issued, ExpiresOn: expires, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) generatePayrollArtifact(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b payrollExportRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.workforce.GenerateExport(r.Context(), workforce.ExportCommand{Scope: p.Scope, PayrollRunID: r.PathValue("payrollID"), ConfigurationID: b.ConfigurationID, Format: b.Format, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
