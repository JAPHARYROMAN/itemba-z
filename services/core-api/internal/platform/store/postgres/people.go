package postgres

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) PeopleSnapshot(ctx context.Context, scope tenancy.Scope, actor string) (people.Snapshot, error) {
	r := people.Snapshot{Employees: []people.Employee{}, Attendance: []people.Attendance{}, LeaveTypes: []people.LeaveType{}, LeaveRequests: []people.LeaveRequest{}, Loans: []people.Loan{}, PayrollRuns: []people.PayrollRun{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "hr.people.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,employee_number,full_name,job_title,department,hire_date,status,currency,base_salary_minor,created_by::text,created_at FROM employees WHERE tenant_id=$1 AND company_id=$2 ORDER BY employee_number`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := people.Employee{Scope: scope}
			if e = rows.Scan(&v.ID, &v.Number, &v.FullName, &v.JobTitle, &v.Department, &v.HireDate, &v.Status, &v.Currency, &v.BaseSalaryMinor, &v.CreatedBy, &v.CreatedAt); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.Employees = append(r.Employees, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,employee_id::text,work_date,regular_minutes,overtime_minutes,reason,COALESCE(reversal_of::text,''),recorded_by::text,recorded_at FROM attendance_entries WHERE tenant_id=$1 AND company_id=$2 ORDER BY work_date,id`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := people.Attendance{Scope: scope}
			if e = rows.Scan(&v.ID, &v.EmployeeID, &v.WorkDate, &v.RegularMinutes, &v.OvertimeMinutes, &v.Reason, &v.ReversalOf, &v.RecordedBy, &v.RecordedAt); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.Attendance = append(r.Attendance, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,code,name_en,name_sw,annual_entitlement_days,effective_from,effective_to,created_by::text,created_at FROM leave_types WHERE tenant_id=$1 AND company_id=$2 ORDER BY code,effective_from`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := people.LeaveType{Scope: scope}
			if e = rows.Scan(&v.ID, &v.Code, &v.NameEN, &v.NameSW, &v.AnnualEntitlementDays, &v.EffectiveFrom, &v.EffectiveTo, &v.CreatedBy, &v.CreatedAt); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.LeaveTypes = append(r.LeaveTypes, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,employee_id::text,leave_type_id::text,starts_on,ends_on,days,reason,status,created_by::text,created_at,COALESCE(approved_by::text,'') FROM leave_requests WHERE tenant_id=$1 AND company_id=$2 ORDER BY starts_on,id`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := people.LeaveRequest{Scope: scope}
			if e = rows.Scan(&v.ID, &v.EmployeeID, &v.LeaveTypeID, &v.StartsOn, &v.EndsOn, &v.Days, &v.Reason, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.LeaveRequests = append(r.LeaveRequests, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,employee_id::text,reference,currency,principal_minor,outstanding_minor,receivable_account_id,bank_account_id,reason,status,created_by::text,created_at,COALESCE(approved_by::text,''),COALESCE(journal_id::text,'') FROM employee_loans WHERE tenant_id=$1 AND company_id=$2 ORDER BY created_at,id`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := people.Loan{Scope: scope}
			if e = rows.Scan(&v.ID, &v.EmployeeID, &v.Reference, &v.Currency, &v.PrincipalMinor, &v.OutstandingMinor, &v.ReceivableAccountID, &v.BankAccountID, &v.Reason, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.JournalID); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.Loans = append(r.Loans, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,reference,currency,period_start,period_end,payment_date,status,salary_expense_account_id,payroll_payable_account_id,deduction_liability_account_id,reason,gross_minor,deductions_minor,net_minor,created_by::text,created_at,COALESCE(approved_by::text,''),COALESCE(journal_id::text,'') FROM payroll_runs WHERE tenant_id=$1 AND company_id=$2 ORDER BY period_end,id`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := people.PayrollRun{Scope: scope, Lines: []people.PayrollLine{}}
			if e = rows.Scan(&v.ID, &v.Reference, &v.Currency, &v.PeriodStart, &v.PeriodEnd, &v.PaymentDate, &v.Status, &v.SalaryExpenseAccountID, &v.PayrollPayableAccountID, &v.DeductionLiabilityAccountID, &v.Reason, &v.GrossMinor, &v.DeductionsMinor, &v.NetMinor, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.JournalID); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			r.PayrollRuns = append(r.PayrollRuns, v)
		}
		rows.Close()
		for index := range r.PayrollRuns {
			v := &r.PayrollRuns[index]
			lr, x := tx.tx.Query(ctx, `SELECT employee_id::text,gross_minor,other_deductions_minor,loan_deduction_minor,net_minor FROM payroll_lines WHERE tenant_id=$1 AND company_id=$2 AND payroll_run_id=$3 ORDER BY employee_id`, scope.TenantID, scope.CompanyID, v.ID)
			if x != nil {
				return normalizeError(x)
			}
			for lr.Next() {
				var l people.PayrollLine
				if x = lr.Scan(&l.EmployeeID, &l.GrossMinor, &l.OtherDeductionsMinor, &l.LoanDeductionMinor, &l.NetMinor); x != nil {
					lr.Close()
					return normalizeError(x)
				}
				v.Lines = append(v.Lines, l)
			}
			lr.Close()
		}
		return nil
	})
	return r, err
}

func (s *Store) CreateEmployee(ctx context.Context, v people.Employee, idem, hash string) (people.Employee, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "hr.employees.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "hr.employee.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			return tx.tx.QueryRow(ctx, `SELECT id::text,employee_number,full_name,job_title,department,hire_date,status,currency,base_salary_minor,created_by::text,created_at FROM employees WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, id).Scan(&v.ID, &v.Number, &v.FullName, &v.JobTitle, &v.Department, &v.HireDate, &v.Status, &v.Currency, &v.BaseSalaryMinor, &v.CreatedBy, &v.CreatedAt)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO employees(id,tenant_id,company_id,branch_id,warehouse_id,employee_number,full_name,job_title,department,hire_date,status,currency,base_salary_minor,created_by,created_at) VALUES($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, v.Number, v.FullName, v.JobTitle, v.Department, v.HireDate, v.Status, v.Currency, v.BaseSalaryMinor, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.employee_created", "employee", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.employee.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) RecordAttendance(ctx context.Context, v people.Attendance, idem, hash string) (people.Attendance, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.RecordedBy, "hr.attendance.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "hr.attendance.record.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			return tx.tx.QueryRow(ctx, `SELECT id::text,employee_id::text,work_date,regular_minutes,overtime_minutes,reason,COALESCE(reversal_of::text,''),recorded_by::text,recorded_at FROM attendance_entries WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, id).Scan(&v.ID, &v.EmployeeID, &v.WorkDate, &v.RegularMinutes, &v.OvertimeMinutes, &v.Reason, &v.ReversalOf, &v.RecordedBy, &v.RecordedAt)
		}
		if v.ReversalOf != "" {
			var emp string
			var regular, overtime int64
			if e = tx.tx.QueryRow(ctx, `SELECT employee_id::text,regular_minutes,overtime_minutes FROM attendance_entries WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.ReversalOf).Scan(&emp, &regular, &overtime); e != nil {
				return normalizeError(e)
			}
			if emp != v.EmployeeID {
				return people.ErrInvalidCommand
			}
			v.RegularMinutes = -regular
			v.OvertimeMinutes = -overtime
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO attendance_entries(id,tenant_id,company_id,employee_id,work_date,regular_minutes,overtime_minutes,reason,reversal_of,recorded_by,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid,$10,$11)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID, v.WorkDate, v.RegularMinutes, v.OvertimeMinutes, v.Reason, v.ReversalOf, v.RecordedBy, v.RecordedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.RecordedBy, "hr.attendance_recorded", "attendance", v.ID, v.ID, v.RecordedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.attendance.record.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) CreateLeaveType(ctx context.Context, v people.LeaveType, actor, idem, hash string) (people.LeaveType, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, actor, "hr.settings.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "hr.leave-type.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO leave_types(id,tenant_id,company_id,code,name_en,name_sw,annual_entitlement_days,effective_from,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Code, v.NameEN, v.NameSW, v.AnnualEntitlementDays, v.EffectiveFrom, actor, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, actor, "hr.leave_type_created", "leave_type", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.leave-type.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) CreateLeave(ctx context.Context, v people.LeaveRequest, idem, hash string) (people.LeaveRequest, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "hr.leave.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "hr.leave.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO leave_requests(id,tenant_id,company_id,employee_id,leave_type_id,starts_on,ends_on,days,reason,status,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID, v.LeaveTypeID, v.StartsOn, v.EndsOn, v.Days, v.Reason, v.Status, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.leave_created", "leave_request", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.leave.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) TransitionLeave(ctx context.Context, scope tenancy.Scope, actor, id string, to people.WorkflowStatus, reason, idem, hash string, at time.Time) (people.LeaveRequest, error) {
	var v people.LeaveRequest
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "hr.leave.manage"
		if to == people.Approved || to == people.Rejected {
			perm = "hr.leave.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, perm)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "hr.leave.transition." + string(to) + ".v1"
		acquired, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			v.ID = id
			v.Scope = scope
			v.Status = to
			return nil
		}
		var from people.WorkflowStatus
		var maker string
		if e = tx.tx.QueryRow(ctx, `SELECT employee_id::text,leave_type_id::text,starts_on,ends_on,days,reason,status,created_by::text,created_at,COALESCE(approved_by::text,'') FROM leave_requests WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.EmployeeID, &v.LeaveTypeID, &v.StartsOn, &v.EndsOn, &v.Days, &v.Reason, &from, &maker, &v.CreatedAt, &v.ApprovedBy); e != nil {
			return normalizeError(e)
		}
		valid := from == people.Draft && to == people.Submitted || from == people.Submitted && (to == people.Approved || to == people.Rejected)
		if !valid {
			return people.ErrInvalidTransition
		}
		if (to == people.Approved || to == people.Rejected) && actor == maker {
			return people.ErrSeparationOfDuties
		}
		if to == people.Approved {
			var overlap bool
			if e = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM leave_requests WHERE tenant_id=$1 AND company_id=$2 AND employee_id=$3 AND status='APPROVED' AND id<>$4 AND NOT(ends_on<$5 OR starts_on>$6))`, scope.TenantID, scope.CompanyID, v.EmployeeID, id, v.StartsOn, v.EndsOn).Scan(&overlap); e != nil {
				return normalizeError(e)
			}
			if overlap {
				return people.ErrOverlap
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE leave_requests SET status=$1,approved_by=CASE WHEN $1 IN('APPROVED','REJECTED') THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1 IN('APPROVED','REJECTED') THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO people_transitions(tenant_id,company_id,entity_type,entity_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,'LEAVE',$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "hr.leave_"+string(to), "leave_request", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.ID = id
		v.Scope = scope
		v.Status = to
		v.CreatedBy = maker
		if to == people.Approved {
			v.ApprovedBy = actor
		}
		return nil
	})
	return v, err
}

func (s *Store) CreateLoan(ctx context.Context, v people.Loan, idem, hash string) (people.Loan, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "hr.loans.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "hr.loan.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO employee_loans(id,tenant_id,company_id,employee_id,reference,currency,principal_minor,outstanding_minor,receivable_account_id,bank_account_id,reason,status,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8,$9,$10,$11,$12,$13)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID, v.Reference, v.Currency, v.PrincipalMinor, v.ReceivableAccountID, v.BankAccountID, v.Reason, v.Status, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.loan_created", "employee_loan", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.loan.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) TransitionLoan(ctx context.Context, scope tenancy.Scope, actor, id string, to people.WorkflowStatus, reason, journal, idem, hash string, at time.Time) (people.Loan, error) {
	var v people.Loan
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "hr.loans.manage"
		if to != people.Submitted {
			perm = "hr.loans.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, perm)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "hr.loan.transition." + string(to) + ".v1"
		acquired, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			v.ID = id
			v.Scope = scope
			v.Status = to
			return nil
		}
		v.Scope = scope
		v.ID = id
		if e = tx.tx.QueryRow(ctx, `SELECT employee_id::text,reference,currency,principal_minor,outstanding_minor,receivable_account_id,bank_account_id,reason,status,created_by::text,created_at,COALESCE(approved_by::text,''),COALESCE(journal_id::text,'') FROM employee_loans WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.EmployeeID, &v.Reference, &v.Currency, &v.PrincipalMinor, &v.OutstandingMinor, &v.ReceivableAccountID, &v.BankAccountID, &v.Reason, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.JournalID); e != nil {
			return normalizeError(e)
		}
		from := v.Status
		valid := from == people.Draft && to == people.Submitted || from == people.Submitted && (to == people.Approved || to == people.Rejected) || from == people.Approved && to == people.Posted
		if !valid {
			return people.ErrInvalidTransition
		}
		if (to == people.Approved || to == people.Rejected) && actor == v.CreatedBy {
			return people.ErrSeparationOfDuties
		}
		if to == people.Posted {
			if e = tx.requireOpenPeriod(ctx, scope, at); e != nil {
				return e
			}
			if e = tx.CreateJournal(ctx, finance.Journal{ID: journal, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "EMPLOYEE_LOAN", SourceID: id, Currency: v.Currency, OccurredAt: at, Entries: []finance.JournalEntry{{AccountID: v.ReceivableAccountID, DebitMinor: v.PrincipalMinor, Memo: reason}, {AccountID: v.BankAccountID, CreditMinor: v.PrincipalMinor, Memo: reason}}}); e != nil {
				return e
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE employee_loans SET status=$1,approved_by=CASE WHEN $1='APPROVED' THEN $2::uuid ELSE approved_by END,journal_id=CASE WHEN $1='POSTED' THEN $3::uuid ELSE journal_id END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, nilUUID(journal), scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO people_transitions(tenant_id,company_id,entity_type,entity_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,'LOAN',$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "hr.loan_"+string(to), "employee_loan", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.Status = to
		if to == people.Approved {
			v.ApprovedBy = actor
		}
		if to == people.Posted {
			v.JournalID = journal
		}
		return nil
	})
	return v, err
}

func (s *Store) CreatePayroll(ctx context.Context, v people.PayrollRun, idem, hash string) (people.PayrollRun, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "hr.payroll.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "hr.payroll.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO payroll_runs(id,tenant_id,company_id,reference,currency,period_start,period_end,payment_date,status,salary_expense_account_id,payroll_payable_account_id,deduction_liability_account_id,reason,gross_minor,deductions_minor,net_minor,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Reference, v.Currency, v.PeriodStart, v.PeriodEnd, v.PaymentDate, v.Status, v.SalaryExpenseAccountID, v.PayrollPayableAccountID, v.DeductionLiabilityAccountID, v.Reason, v.GrossMinor, v.DeductionsMinor, v.NetMinor, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		for _, l := range v.Lines {
			_, e = tx.tx.Exec(ctx, `INSERT INTO payroll_lines(tenant_id,company_id,payroll_run_id,employee_id,gross_minor,other_deductions_minor,loan_deduction_minor,net_minor) SELECT $1,$2,$3,$4,$5,$6,$7,$8 WHERE EXISTS(SELECT 1 FROM employees WHERE tenant_id=$1 AND company_id=$2 AND id=$4 AND status='ACTIVE')`, v.Scope.TenantID, v.Scope.CompanyID, v.ID, l.EmployeeID, l.GrossMinor, l.OtherDeductionsMinor, l.LoanDeductionMinor, l.NetMinor)
			if e != nil {
				return normalizeError(e)
			}
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.payroll_created", "payroll_run", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.payroll.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) TransitionPayroll(ctx context.Context, scope tenancy.Scope, actor, id string, to people.WorkflowStatus, reason, journal, idem, hash string, at time.Time) (people.PayrollRun, error) {
	var v people.PayrollRun
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "hr.payroll.manage"
		if to != people.Submitted {
			perm = "hr.payroll.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, perm)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "hr.payroll.transition." + string(to) + ".v1"
		acquired, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			v.ID = id
			v.Scope = scope
			v.Status = to
			return nil
		}
		v.Scope = scope
		v.ID = id
		v.Lines = []people.PayrollLine{}
		if e = tx.tx.QueryRow(ctx, `SELECT reference,currency,period_start,period_end,payment_date,status,salary_expense_account_id,payroll_payable_account_id,deduction_liability_account_id,reason,gross_minor,deductions_minor,net_minor,created_by::text,created_at,COALESCE(approved_by::text,''),COALESCE(journal_id::text,'') FROM payroll_runs WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.Reference, &v.Currency, &v.PeriodStart, &v.PeriodEnd, &v.PaymentDate, &v.Status, &v.SalaryExpenseAccountID, &v.PayrollPayableAccountID, &v.DeductionLiabilityAccountID, &v.Reason, &v.GrossMinor, &v.DeductionsMinor, &v.NetMinor, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy, &v.JournalID); e != nil {
			return normalizeError(e)
		}
		from := v.Status
		valid := from == people.Draft && to == people.Submitted || from == people.Submitted && (to == people.Approved || to == people.Rejected) || from == people.Approved && to == people.Posted
		if !valid {
			return people.ErrInvalidTransition
		}
		if (to == people.Approved || to == people.Rejected) && actor == v.CreatedBy {
			return people.ErrSeparationOfDuties
		}
		if to == people.Posted {
			if e = tx.requireOpenPeriod(ctx, scope, v.PaymentDate); e != nil {
				return e
			}
			rows, e := tx.tx.Query(ctx, `SELECT employee_id::text,gross_minor,other_deductions_minor,loan_deduction_minor,net_minor FROM payroll_lines WHERE tenant_id=$1 AND company_id=$2 AND payroll_run_id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id)
			if e != nil {
				return normalizeError(e)
			}
			entries := []finance.JournalEntry{{AccountID: v.SalaryExpenseAccountID, DebitMinor: v.GrossMinor, Memo: reason}, {AccountID: v.PayrollPayableAccountID, CreditMinor: v.NetMinor, Memo: reason}}
			for rows.Next() {
				var l people.PayrollLine
				if e = rows.Scan(&l.EmployeeID, &l.GrossMinor, &l.OtherDeductionsMinor, &l.LoanDeductionMinor, &l.NetMinor); e != nil {
					rows.Close()
					return normalizeError(e)
				}
				v.Lines = append(v.Lines, l)
			}
			rows.Close()
			var other int64
			for _, l := range v.Lines {
				other += l.OtherDeductionsMinor
				if l.LoanDeductionMinor > 0 {
					var loanID, account string
					var outstanding int64
					if e = tx.tx.QueryRow(ctx, `SELECT id::text,receivable_account_id,outstanding_minor FROM employee_loans WHERE tenant_id=$1 AND company_id=$2 AND employee_id=$3 AND status='POSTED' AND outstanding_minor>0 ORDER BY created_at LIMIT 1 FOR UPDATE`, scope.TenantID, scope.CompanyID, l.EmployeeID).Scan(&loanID, &account, &outstanding); e != nil {
						return people.ErrInsufficientLoanBalance
					}
					if outstanding < l.LoanDeductionMinor {
						return people.ErrInsufficientLoanBalance
					}
					if _, e = tx.tx.Exec(ctx, `UPDATE employee_loans SET outstanding_minor=outstanding_minor-$1 WHERE tenant_id=$2 AND company_id=$3 AND id=$4`, l.LoanDeductionMinor, scope.TenantID, scope.CompanyID, loanID); e != nil {
						return normalizeError(e)
					}
					entries = append(entries, finance.JournalEntry{AccountID: account, CreditMinor: l.LoanDeductionMinor, Memo: reason})
				}
			}
			if other > 0 {
				entries = append(entries, finance.JournalEntry{AccountID: v.DeductionLiabilityAccountID, CreditMinor: other, Memo: reason})
			}
			if e = tx.CreateJournal(ctx, finance.Journal{ID: journal, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "PAYROLL", SourceID: id, Currency: v.Currency, OccurredAt: v.PaymentDate, Entries: entries}); e != nil {
				return e
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE payroll_runs SET status=$1,approved_by=CASE WHEN $1='APPROVED' THEN $2::uuid ELSE approved_by END,journal_id=CASE WHEN $1='POSTED' THEN $3::uuid ELSE journal_id END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, nilUUID(journal), scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO people_transitions(tenant_id,company_id,entity_type,entity_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,'PAYROLL',$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "hr.payroll_"+string(to), "payroll_run", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.Status = to
		if to == people.Approved {
			v.ApprovedBy = actor
		}
		if to == people.Posted {
			v.JournalID = journal
		}
		return nil
	})
	return v, err
}
func nilUUID(v string) any {
	if v == "" {
		return nil
	}
	return v
}
