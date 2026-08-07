package people

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Repository interface {
	PeopleSnapshot(context.Context, tenancy.Scope, string) (Snapshot, error)
	CreateEmployee(context.Context, Employee, string, string) (Employee, error)
	RecordAttendance(context.Context, Attendance, string, string) (Attendance, error)
	CreateLeaveType(context.Context, LeaveType, string, string, string) (LeaveType, error)
	CreateLeave(context.Context, LeaveRequest, string, string) (LeaveRequest, error)
	TransitionLeave(context.Context, tenancy.Scope, string, string, WorkflowStatus, string, string, string, time.Time) (LeaveRequest, error)
	CreateLoan(context.Context, Loan, string, string) (Loan, error)
	TransitionLoan(context.Context, tenancy.Scope, string, string, WorkflowStatus, string, string, string, string, time.Time) (Loan, error)
	CreatePayroll(context.Context, PayrollRun, string, string) (PayrollRun, error)
	TransitionPayroll(context.Context, tenancy.Scope, string, string, WorkflowStatus, string, string, string, string, time.Time) (PayrollRun, error)
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || ids == nil || c == nil {
		return nil, errors.New("people repository, id generator, and clock are required")
	}
	return &Service{r, ids, c}, nil
}

type CreateEmployeeCommand struct {
	Scope                                            tenancy.Scope
	Number, FullName, JobTitle, Department, Currency string
	HireDate                                         time.Time
	BaseSalaryMinor                                  int64
	ActorID, IdempotencyKey                          string
}
type AttendanceCommand struct {
	Scope                                       tenancy.Scope
	EmployeeID                                  string
	WorkDate                                    time.Time
	RegularMinutes, OvertimeMinutes             int64
	Reason, ReversalOf, ActorID, IdempotencyKey string
}
type LeaveTypeCommand struct {
	Scope                   tenancy.Scope
	Code, NameEN, NameSW    string
	AnnualEntitlementDays   int64
	EffectiveFrom           time.Time
	ActorID, IdempotencyKey string
}
type LeaveCommand struct {
	Scope                           tenancy.Scope
	EmployeeID, LeaveTypeID         string
	StartsOn, EndsOn                time.Time
	Reason, ActorID, IdempotencyKey string
}
type TransitionCommand struct {
	Scope                           tenancy.Scope
	ID                              string
	Status                          WorkflowStatus
	Reason, ActorID, IdempotencyKey string
}
type LoanCommand struct {
	Scope                                                                                                tenancy.Scope
	EmployeeID, Reference, Currency, ReceivableAccountID, BankAccountID, Reason, ActorID, IdempotencyKey string
	PrincipalMinor                                                                                       int64
}
type PayrollCommand struct {
	Scope                                                                                                                              tenancy.Scope
	Reference, Currency, SalaryExpenseAccountID, PayrollPayableAccountID, DeductionLiabilityAccountID, Reason, ActorID, IdempotencyKey string
	PeriodStart, PeriodEnd, PaymentDate                                                                                                time.Time
	Lines                                                                                                                              []PayrollLine
}

func (s *Service) Get(ctx context.Context, scope tenancy.Scope, actor string) (Snapshot, error) {
	return s.repository.PeopleSnapshot(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) CreateEmployee(ctx context.Context, c CreateEmployeeCommand) (Employee, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Number = strings.ToUpper(strings.TrimSpace(c.Number))
	c.FullName = strings.TrimSpace(c.FullName)
	c.JobTitle = strings.TrimSpace(c.JobTitle)
	c.Department = strings.TrimSpace(c.Department)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.HireDate = date(c.HireDate)
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Number) < 2 || len(c.FullName) < 3 || len(c.Currency) != 3 || c.BaseSalaryMinor < 0 || !wire.IsSafeInteger(c.BaseSalaryMinor) || c.HireDate.IsZero() || !validIdem(c.IdempotencyKey) {
		return Employee{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Employee{}, e
	}
	v := Employee{ID: id, Scope: c.Scope, Number: c.Number, FullName: c.FullName, JobTitle: c.JobTitle, Department: c.Department, Currency: c.Currency, HireDate: c.HireDate, Status: EmployeeActive, BaseSalaryMinor: c.BaseSalaryMinor, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateEmployee(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) RecordAttendance(ctx context.Context, c AttendanceCommand) (Attendance, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.EmployeeID = identity.NormalizeClaim(c.EmployeeID)
	c.ReversalOf = identity.NormalizeClaim(c.ReversalOf)
	c.Reason = strings.TrimSpace(c.Reason)
	c.WorkDate = date(c.WorkDate)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.EmployeeID) || c.ActorID == "" || c.WorkDate.IsZero() || c.RegularMinutes < 0 || c.RegularMinutes > 1440 || c.OvertimeMinutes < 0 || c.OvertimeMinutes > 720 || (c.RegularMinutes+c.OvertimeMinutes == 0 && c.ReversalOf == "") || len(c.Reason) < 8 || !validIdem(c.IdempotencyKey) {
		return Attendance{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Attendance{}, e
	}
	v := Attendance{ID: id, Scope: c.Scope, EmployeeID: c.EmployeeID, WorkDate: c.WorkDate, RegularMinutes: c.RegularMinutes, OvertimeMinutes: c.OvertimeMinutes, Reason: c.Reason, ReversalOf: c.ReversalOf, RecordedBy: c.ActorID, RecordedAt: s.clock.Now().UTC()}
	return s.repository.RecordAttendance(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) CreateLeaveType(ctx context.Context, c LeaveTypeCommand) (LeaveType, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	c.NameEN = strings.TrimSpace(c.NameEN)
	c.NameSW = strings.TrimSpace(c.NameSW)
	c.EffectiveFrom = date(c.EffectiveFrom)
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Code) < 2 || len(c.NameEN) < 2 || len(c.NameSW) < 2 || c.AnnualEntitlementDays < 0 || c.AnnualEntitlementDays > 366 || c.EffectiveFrom.IsZero() || !validIdem(c.IdempotencyKey) {
		return LeaveType{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return LeaveType{}, e
	}
	v := LeaveType{ID: id, Scope: c.Scope, Code: c.Code, NameEN: c.NameEN, NameSW: c.NameSW, AnnualEntitlementDays: c.AnnualEntitlementDays, EffectiveFrom: c.EffectiveFrom, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateLeaveType(ctx, v, c.ActorID, c.IdempotencyKey, hash(c))
}
func (s *Service) CreateLeave(ctx context.Context, c LeaveCommand) (LeaveRequest, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.EmployeeID = identity.NormalizeClaim(c.EmployeeID)
	c.LeaveTypeID = identity.NormalizeClaim(c.LeaveTypeID)
	c.Reason = strings.TrimSpace(c.Reason)
	c.StartsOn = date(c.StartsOn)
	c.EndsOn = date(c.EndsOn)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.EmployeeID) || !identity.IsUUID(c.LeaveTypeID) || c.ActorID == "" || c.EndsOn.Before(c.StartsOn) || len(c.Reason) < 8 || !validIdem(c.IdempotencyKey) {
		return LeaveRequest{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return LeaveRequest{}, e
	}
	v := LeaveRequest{ID: id, Scope: c.Scope, EmployeeID: c.EmployeeID, LeaveTypeID: c.LeaveTypeID, StartsOn: c.StartsOn, EndsOn: c.EndsOn, Days: int64(c.EndsOn.Sub(c.StartsOn).Hours()/24) + 1, Reason: c.Reason, Status: Draft, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateLeave(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionLeave(ctx context.Context, c TransitionCommand) (LeaveRequest, error) {
	if !transitionValid(&c) {
		return LeaveRequest{}, ErrInvalidCommand
	}
	return s.repository.TransitionLeave(ctx, c.Scope.Normalize(), identity.NormalizeClaim(c.ActorID), identity.NormalizeClaim(c.ID), c.Status, strings.TrimSpace(c.Reason), c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) CreateLoan(ctx context.Context, c LoanCommand) (Loan, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.EmployeeID = identity.NormalizeClaim(c.EmployeeID)
	c.Reference = strings.ToUpper(strings.TrimSpace(c.Reference))
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.ReceivableAccountID = identity.NormalizeClaim(c.ReceivableAccountID)
	c.BankAccountID = identity.NormalizeClaim(c.BankAccountID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.EmployeeID) || c.ActorID == "" || len(c.Reference) < 2 || len(c.Currency) != 3 || c.PrincipalMinor <= 0 || !wire.IsSafeInteger(c.PrincipalMinor) || len(c.Reason) < 8 || !validIdem(c.IdempotencyKey) {
		return Loan{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Loan{}, e
	}
	v := Loan{ID: id, Scope: c.Scope, EmployeeID: c.EmployeeID, Reference: c.Reference, Currency: c.Currency, PrincipalMinor: c.PrincipalMinor, OutstandingMinor: c.PrincipalMinor, ReceivableAccountID: c.ReceivableAccountID, BankAccountID: c.BankAccountID, Reason: c.Reason, Status: Draft, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateLoan(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionLoan(ctx context.Context, c TransitionCommand) (Loan, error) {
	if !transitionValid(&c) {
		return Loan{}, ErrInvalidCommand
	}
	journal := ""
	if c.Status == Posted {
		var e error
		journal, e = s.ids.New()
		if e != nil {
			return Loan{}, e
		}
	}
	return s.repository.TransitionLoan(ctx, c.Scope.Normalize(), identity.NormalizeClaim(c.ActorID), identity.NormalizeClaim(c.ID), c.Status, strings.TrimSpace(c.Reason), journal, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) CreatePayroll(ctx context.Context, c PayrollCommand) (PayrollRun, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reference = strings.ToUpper(strings.TrimSpace(c.Reference))
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	c.PeriodStart = date(c.PeriodStart)
	c.PeriodEnd = date(c.PeriodEnd)
	c.PaymentDate = date(c.PaymentDate)
	var gross, deductions, net int64
	for i := range c.Lines {
		l := &c.Lines[i]
		if !identity.IsUUID(identity.NormalizeClaim(l.EmployeeID)) || l.GrossMinor < 0 || l.OtherDeductionsMinor < 0 || l.LoanDeductionMinor < 0 || l.OtherDeductionsMinor+l.LoanDeductionMinor > l.GrossMinor {
			return PayrollRun{}, ErrInvalidCommand
		}
		l.NetMinor = l.GrossMinor - l.OtherDeductionsMinor - l.LoanDeductionMinor
		gross += l.GrossMinor
		deductions += l.OtherDeductionsMinor + l.LoanDeductionMinor
		net += l.NetMinor
	}
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Reference) < 2 || len(c.Currency) != 3 || c.PeriodStart.IsZero() || c.PeriodEnd.Before(c.PeriodStart) || c.PaymentDate.Before(c.PeriodStart) || len(c.Lines) == 0 || gross <= 0 || len(c.Reason) < 8 || !validIdem(c.IdempotencyKey) {
		return PayrollRun{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return PayrollRun{}, e
	}
	v := PayrollRun{ID: id, Scope: c.Scope, Reference: c.Reference, Currency: c.Currency, PeriodStart: c.PeriodStart, PeriodEnd: c.PeriodEnd, PaymentDate: c.PaymentDate, SalaryExpenseAccountID: c.SalaryExpenseAccountID, PayrollPayableAccountID: c.PayrollPayableAccountID, DeductionLiabilityAccountID: c.DeductionLiabilityAccountID, Reason: c.Reason, Status: Draft, Lines: c.Lines, GrossMinor: gross, DeductionsMinor: deductions, NetMinor: net, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreatePayroll(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionPayroll(ctx context.Context, c TransitionCommand) (PayrollRun, error) {
	if !transitionValid(&c) {
		return PayrollRun{}, ErrInvalidCommand
	}
	journal := ""
	if c.Status == Posted {
		var e error
		journal, e = s.ids.New()
		if e != nil {
			return PayrollRun{}, e
		}
	}
	return s.repository.TransitionPayroll(ctx, c.Scope.Normalize(), identity.NormalizeClaim(c.ActorID), identity.NormalizeClaim(c.ID), c.Status, strings.TrimSpace(c.Reason), journal, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}

func transitionValid(c *TransitionCommand) bool {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.ID = identity.NormalizeClaim(c.ID)
	c.Reason = strings.TrimSpace(c.Reason)
	return c.Scope.Validate() == nil && identity.IsUUID(c.ID) && c.ActorID != "" && (c.Status == Submitted || c.Status == Approved || c.Status == Posted || c.Status == Rejected) && len(c.Reason) >= 8 && validIdem(c.IdempotencyKey)
}
func validIdem(v string) bool { return len(strings.TrimSpace(v)) >= 16 && len(v) <= 128 }
func date(v time.Time) time.Time {
	y, m, d := v.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	x := sha256.Sum256(b)
	return hex.EncodeToString(x[:])
}
