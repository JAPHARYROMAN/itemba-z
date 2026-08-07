package httpapi

import (
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"net/http"
	"strings"
	"time"
)

type employeeRequest struct {
	Number          string `json:"number"`
	FullName        string `json:"full_name"`
	JobTitle        string `json:"job_title"`
	Department      string `json:"department"`
	Currency        string `json:"currency"`
	HireDate        string `json:"hire_date"`
	BaseSalaryMinor int64  `json:"base_salary_minor"`
}
type attendanceRequest struct {
	EmployeeID      string `json:"employee_id"`
	WorkDate        string `json:"work_date"`
	Reason          string `json:"reason"`
	ReversalOf      string `json:"reversal_of"`
	RegularMinutes  int64  `json:"regular_minutes"`
	OvertimeMinutes int64  `json:"overtime_minutes"`
}
type leaveTypeRequest struct {
	Code                  string `json:"code"`
	NameEN                string `json:"name_en"`
	NameSW                string `json:"name_sw"`
	EffectiveFrom         string `json:"effective_from"`
	AnnualEntitlementDays int64  `json:"annual_entitlement_days"`
}
type leaveRequest struct {
	EmployeeID  string `json:"employee_id"`
	LeaveTypeID string `json:"leave_type_id"`
	StartsOn    string `json:"starts_on"`
	EndsOn      string `json:"ends_on"`
	Reason      string `json:"reason"`
}
type peopleTransitionRequest struct {
	Status people.WorkflowStatus `json:"status"`
	Reason string                `json:"reason"`
}
type loanRequest struct {
	EmployeeID          string `json:"employee_id"`
	Reference           string `json:"reference"`
	Currency            string `json:"currency"`
	ReceivableAccountID string `json:"receivable_account_id"`
	BankAccountID       string `json:"bank_account_id"`
	Reason              string `json:"reason"`
	PrincipalMinor      int64  `json:"principal_minor"`
}
type payrollRequest struct {
	Reference                   string               `json:"reference"`
	Currency                    string               `json:"currency"`
	PeriodStart                 string               `json:"period_start"`
	PeriodEnd                   string               `json:"period_end"`
	PaymentDate                 string               `json:"payment_date"`
	SalaryExpenseAccountID      string               `json:"salary_expense_account_id"`
	PayrollPayableAccountID     string               `json:"payroll_payable_account_id"`
	DeductionLiabilityAccountID string               `json:"deduction_liability_account_id"`
	Reason                      string               `json:"reason"`
	Lines                       []people.PayrollLine `json:"lines"`
}

func parsePeopleDate(v string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(v))
}
func (h *Handler) peopleSnapshot(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	v, e := h.people.Get(r.Context(), p.Scope, p.ActorID)
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createEmployee(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b employeeRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	d, e := parsePeopleDate(b.HireDate)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "hire_date must use YYYY-MM-DD")
		return
	}
	v, e := h.people.CreateEmployee(r.Context(), people.CreateEmployeeCommand{Scope: p.Scope, Number: b.Number, FullName: b.FullName, JobTitle: b.JobTitle, Department: b.Department, Currency: b.Currency, HireDate: d, BaseSalaryMinor: b.BaseSalaryMinor, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) recordAttendance(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b attendanceRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	d, e := parsePeopleDate(b.WorkDate)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "work_date must use YYYY-MM-DD")
		return
	}
	v, e := h.people.RecordAttendance(r.Context(), people.AttendanceCommand{Scope: p.Scope, EmployeeID: b.EmployeeID, WorkDate: d, RegularMinutes: b.RegularMinutes, OvertimeMinutes: b.OvertimeMinutes, Reason: b.Reason, ReversalOf: b.ReversalOf, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) createLeaveType(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b leaveTypeRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	d, e := parsePeopleDate(b.EffectiveFrom)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "effective_from must use YYYY-MM-DD")
		return
	}
	v, e := h.people.CreateLeaveType(r.Context(), people.LeaveTypeCommand{Scope: p.Scope, Code: b.Code, NameEN: b.NameEN, NameSW: b.NameSW, AnnualEntitlementDays: b.AnnualEntitlementDays, EffectiveFrom: d, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) createLeave(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b leaveRequest
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
	v, e := h.people.CreateLeave(r.Context(), people.LeaveCommand{Scope: p.Scope, EmployeeID: b.EmployeeID, LeaveTypeID: b.LeaveTypeID, StartsOn: start, EndsOn: end, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) transitionLeave(w http.ResponseWriter, r *http.Request) {
	h.transitionPeople(w, r, "leave", r.PathValue("leaveID"))
}
func (h *Handler) transitionLoan(w http.ResponseWriter, r *http.Request) {
	h.transitionPeople(w, r, "loan", r.PathValue("loanID"))
}
func (h *Handler) transitionPayroll(w http.ResponseWriter, r *http.Request) {
	h.transitionPeople(w, r, "payroll", r.PathValue("payrollID"))
}
func (h *Handler) transitionPeople(w http.ResponseWriter, r *http.Request, kind, id string) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b peopleTransitionRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	c := people.TransitionCommand{Scope: p.Scope, ID: id, Status: b.Status, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem}
	var v any
	var e error
	switch kind {
	case "leave":
		v, e = h.people.TransitionLeave(r.Context(), c)
	case "loan":
		v, e = h.people.TransitionLoan(r.Context(), c)
	default:
		v, e = h.people.TransitionPayroll(r.Context(), c)
	}
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) createLoan(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b loanRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	v, e := h.people.CreateLoan(r.Context(), people.LoanCommand{Scope: p.Scope, EmployeeID: b.EmployeeID, Reference: b.Reference, Currency: b.Currency, PrincipalMinor: b.PrincipalMinor, ReceivableAccountID: b.ReceivableAccountID, BankAccountID: b.BankAccountID, Reason: b.Reason, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
func (h *Handler) createPayroll(w http.ResponseWriter, r *http.Request) {
	p, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	idem, ok := requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var b payrollRequest
	if !decodeJSON(w, r, &b) {
		return
	}
	start, e := parsePeopleDate(b.PeriodStart)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "period_start must use YYYY-MM-DD")
		return
	}
	end, e := parsePeopleDate(b.PeriodEnd)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "period_end must use YYYY-MM-DD")
		return
	}
	pay, e := parsePeopleDate(b.PaymentDate)
	if e != nil {
		writeProblem(w, 400, "invalid_request", "payment_date must use YYYY-MM-DD")
		return
	}
	v, e := h.people.CreatePayroll(r.Context(), people.PayrollCommand{Scope: p.Scope, Reference: b.Reference, Currency: b.Currency, PeriodStart: start, PeriodEnd: end, PaymentDate: pay, SalaryExpenseAccountID: b.SalaryExpenseAccountID, PayrollPayableAccountID: b.PayrollPayableAccountID, DeductionLiabilityAccountID: b.DeductionLiabilityAccountID, Reason: b.Reason, Lines: b.Lines, ActorID: p.ActorID, IdempotencyKey: idem})
	if e != nil {
		h.writeError(w, r, e)
		return
	}
	writeJSON(w, 201, v)
}
