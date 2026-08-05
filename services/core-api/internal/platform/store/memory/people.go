package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) PeopleSnapshot(_ context.Context, scope tenancy.Scope, actor string) (people.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := people.Snapshot{Employees: []people.Employee{}, Attendance: []people.Attendance{}, LeaveTypes: []people.LeaveType{}, LeaveRequests: []people.LeaveRequest{}, Loans: []people.Loan{}, PayrollRuns: []people.PayrollRun{}}
	if !s.state.permissions[permissionKey(scope, actor, "hr.people.read")] {
		return r, sales.ErrForbidden
	}
	for _, v := range s.state.employees {
		if same(v.Scope, scope) {
			r.Employees = append(r.Employees, v)
		}
	}
	for _, v := range s.state.attendance {
		if same(v.Scope, scope) {
			r.Attendance = append(r.Attendance, v)
		}
	}
	for _, v := range s.state.leaveTypes {
		if same(v.Scope, scope) {
			r.LeaveTypes = append(r.LeaveTypes, v)
		}
	}
	for _, v := range s.state.leaveRequests {
		if same(v.Scope, scope) {
			r.LeaveRequests = append(r.LeaveRequests, v)
		}
	}
	for _, v := range s.state.employeeLoans {
		if same(v.Scope, scope) {
			r.Loans = append(r.Loans, v)
		}
	}
	for _, v := range s.state.payrollRuns {
		if same(v.Scope, scope) {
			v.Lines = append([]people.PayrollLine(nil), v.Lines...)
			r.PayrollRuns = append(r.PayrollRuns, v)
		}
	}
	sort.Slice(r.Employees, func(i, j int) bool { return r.Employees[i].Number < r.Employees[j].Number })
	return r, nil
}
func same(a, b tenancy.Scope) bool { return a.TenantID == b.TenantID && a.CompanyID == b.CompanyID }
func (s *Store) CreateEmployee(_ context.Context, v people.Employee, idem, hash string) (people.Employee, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.employees.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, err := s.memoryIdem(v.Scope, "hr.employee.create.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	for _, e := range s.state.employees {
		if same(e.Scope, v.Scope) && e.Number == v.Number {
			return v, people.ErrInvalidCommand
		}
	}
	s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.employee.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) RecordAttendance(_ context.Context, v people.Attendance, idem, hash string) (people.Attendance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.RecordedBy, "hr.attendance.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, err := s.memoryIdem(v.Scope, "hr.attendance.record.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.attendance[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	if _, ok := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID)]; !ok {
		return v, sales.ErrNotFound
	}
	if v.ReversalOf != "" {
		orig, ok := s.state.attendance[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ReversalOf)]
		if !ok || orig.EmployeeID != v.EmployeeID {
			return v, people.ErrInvalidCommand
		}
		v.RegularMinutes = -orig.RegularMinutes
		v.OvertimeMinutes = -orig.OvertimeMinutes
	}
	s.state.attendance[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.attendance.record.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) CreateLeaveType(_ context.Context, v people.LeaveType, actor, idem, hash string) (people.LeaveType, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, actor, "hr.settings.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, err := s.memoryIdem(v.Scope, "hr.leave-type.create.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.leaveTypes[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	s.state.leaveTypes[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.leave-type.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) CreateLeave(_ context.Context, v people.LeaveRequest, idem, hash string) (people.LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.leave.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, err := s.memoryIdem(v.Scope, "hr.leave.create.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.leaveRequests[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	if _, ok := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID)]; !ok {
		return v, sales.ErrNotFound
	}
	if _, ok := s.state.leaveTypes[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.LeaveTypeID)]; !ok {
		return v, sales.ErrNotFound
	}
	s.state.leaveRequests[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.leave.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionLeave(_ context.Context, scope tenancy.Scope, actor, id string, to people.WorkflowStatus, _ string, idem, hash string, _ time.Time) (people.LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "hr.leave.manage"
	if to == people.Approved || to == people.Rejected {
		perm = "hr.leave.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return people.LeaveRequest{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	v, ok := s.state.leaveRequests[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	if x, hit, err := s.memoryIdem(scope, "hr.leave.transition."+string(to)+".v1", idem, hash); err != nil {
		return v, err
	} else if hit {
		return s.state.leaveRequests[companyEntityKey(scope.TenantID, scope.CompanyID, x)], nil
	}
	valid := v.Status == people.Draft && to == people.Submitted || v.Status == people.Submitted && (to == people.Approved || to == people.Rejected)
	if !valid {
		return v, people.ErrInvalidTransition
	}
	if (to == people.Approved || to == people.Rejected) && actor == v.CreatedBy {
		return v, people.ErrSeparationOfDuties
	}
	if to == people.Approved {
		for _, x := range s.state.leaveRequests {
			if x.ID != v.ID && x.EmployeeID == v.EmployeeID && x.Status == people.Approved && !x.EndsOn.Before(v.StartsOn) && !x.StartsOn.After(v.EndsOn) {
				return v, people.ErrOverlap
			}
		}
		v.ApprovedBy = actor
	}
	v.Status = to
	s.state.leaveRequests[k] = v
	s.putIdem(scope, "hr.leave.transition."+string(to)+".v1", idem, hash, id)
	return v, nil
}
func (s *Store) CreateLoan(_ context.Context, v people.Loan, idem, hash string) (people.Loan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.loans.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, err := s.memoryIdem(v.Scope, "hr.loan.create.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.employeeLoans[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	if _, ok := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID)]; !ok {
		return v, sales.ErrNotFound
	}
	s.state.employeeLoans[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.loan.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionLoan(_ context.Context, scope tenancy.Scope, actor, id string, to people.WorkflowStatus, _ string, journal, idem, hash string, at time.Time) (people.Loan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "hr.loans.manage"
	if to == people.Approved || to == people.Rejected || to == people.Posted {
		perm = "hr.loans.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return people.Loan{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	v, ok := s.state.employeeLoans[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	if x, hit, err := s.memoryIdem(scope, "hr.loan.transition."+string(to)+".v1", idem, hash); err != nil {
		return v, err
	} else if hit {
		return s.state.employeeLoans[companyEntityKey(scope.TenantID, scope.CompanyID, x)], nil
	}
	valid := v.Status == people.Draft && to == people.Submitted || v.Status == people.Submitted && (to == people.Approved || to == people.Rejected) || v.Status == people.Approved && to == people.Posted
	if !valid {
		return v, people.ErrInvalidTransition
	}
	if (to == people.Approved || to == people.Rejected) && actor == v.CreatedBy {
		return v, people.ErrSeparationOfDuties
	}
	if to == people.Approved {
		v.ApprovedBy = actor
	}
	if to == people.Posted {
		v.JournalID = journal
		s.state.journals = append(s.state.journals, finance.Journal{ID: journal, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "EMPLOYEE_LOAN", SourceID: v.ID, Currency: v.Currency, OccurredAt: at, Entries: []finance.JournalEntry{{AccountID: v.ReceivableAccountID, DebitMinor: v.PrincipalMinor}, {AccountID: v.BankAccountID, CreditMinor: v.PrincipalMinor}}})
	}
	v.Status = to
	s.state.employeeLoans[k] = v
	s.putIdem(scope, "hr.loan.transition."+string(to)+".v1", idem, hash, id)
	return v, nil
}
func (s *Store) CreatePayroll(_ context.Context, v people.PayrollRun, idem, hash string) (people.PayrollRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.payroll.manage")] {
		return v, sales.ErrForbidden
	}
	if x, ok, err := s.memoryIdem(v.Scope, "hr.payroll.create.v1", idem, hash); err != nil {
		return v, err
	} else if ok {
		return s.state.payrollRuns[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, x)], nil
	}
	for _, l := range v.Lines {
		e, ok := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, l.EmployeeID)]
		if !ok || e.Status != people.EmployeeActive {
			return v, people.ErrInvalidCommand
		}
	}
	s.state.payrollRuns[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.payroll.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionPayroll(_ context.Context, scope tenancy.Scope, actor, id string, to people.WorkflowStatus, _ string, journal, idem, hash string, at time.Time) (people.PayrollRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "hr.payroll.manage"
	if to == people.Approved || to == people.Rejected || to == people.Posted {
		perm = "hr.payroll.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return people.PayrollRun{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	v, ok := s.state.payrollRuns[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	if x, hit, err := s.memoryIdem(scope, "hr.payroll.transition."+string(to)+".v1", idem, hash); err != nil {
		return v, err
	} else if hit {
		return s.state.payrollRuns[companyEntityKey(scope.TenantID, scope.CompanyID, x)], nil
	}
	valid := v.Status == people.Draft && to == people.Submitted || v.Status == people.Submitted && (to == people.Approved || to == people.Rejected) || v.Status == people.Approved && to == people.Posted
	if !valid {
		return v, people.ErrInvalidTransition
	}
	if (to == people.Approved || to == people.Rejected) && actor == v.CreatedBy {
		return v, people.ErrSeparationOfDuties
	}
	if to == people.Approved {
		v.ApprovedBy = actor
	}
	if to == people.Posted {
		entries := []finance.JournalEntry{{AccountID: v.SalaryExpenseAccountID, DebitMinor: v.GrossMinor}, {AccountID: v.PayrollPayableAccountID, CreditMinor: v.NetMinor}}
		var other int64
		for _, l := range v.Lines {
			other += l.OtherDeductionsMinor
			if l.LoanDeductionMinor > 0 {
				var selectedKey string
				var loan people.Loan
				for lk, x := range s.state.employeeLoans {
					if x.EmployeeID == l.EmployeeID && x.Status == people.Posted && x.OutstandingMinor > 0 {
						selectedKey = lk
						loan = x
						break
					}
				}
				if selectedKey == "" || loan.OutstandingMinor < l.LoanDeductionMinor {
					return v, people.ErrInsufficientLoanBalance
				}
				loan.OutstandingMinor -= l.LoanDeductionMinor
				s.state.employeeLoans[selectedKey] = loan
				entries = append(entries, finance.JournalEntry{AccountID: loan.ReceivableAccountID, CreditMinor: l.LoanDeductionMinor})
			}
		}
		if other > 0 {
			entries = append(entries, finance.JournalEntry{AccountID: v.DeductionLiabilityAccountID, CreditMinor: other})
		}
		v.JournalID = journal
		s.state.journals = append(s.state.journals, finance.Journal{ID: journal, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "PAYROLL", SourceID: v.ID, Currency: v.Currency, OccurredAt: v.PaymentDate, Entries: entries})
	}
	v.Status = to
	s.state.payrollRuns[k] = v
	s.putIdem(scope, "hr.payroll.transition."+string(to)+".v1", idem, hash, id)
	return v, nil
}
func (s *Store) memoryIdem(scope tenancy.Scope, op, key, hash string) (string, bool, error) {
	x, ok := s.state.idempotencies[idempotencyKey(scope, op, key)]
	if !ok {
		return "", false, nil
	}
	if x.RequestHash != hash {
		return "", false, sales.ErrIdempotencyConflict
	}
	return x.ResultID, true, nil
}
func (s *Store) putIdem(scope tenancy.Scope, op, key, hash, id string) {
	s.state.idempotencies[idempotencyKey(scope, op, key)] = idempotency{RequestHash: hash, ResultID: id}
}
