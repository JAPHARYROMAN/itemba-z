package people_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestPeoplePayrollLifecyclePostsBalancedLedgerAndLoanDeduction(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	ids := &identity.SequenceGenerator{Values: []string{"23000000-0000-4000-8000-000000000001", "23000000-0000-4000-8000-000000000002", "23000000-0000-4000-8000-000000000003", "23000000-0000-4000-8000-000000000004", "23000000-0000-4000-8000-000000000005", "23000000-0000-4000-8000-000000000006", "23000000-0000-4000-8000-000000000007"}}
	store := memory.New()
	svc, e := people.NewService(store, ids, clock.Fixed{Time: at})
	if e != nil {
		t.Fatal(e)
	}
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	maker, checker := "maker", "checker"
	for _, a := range []string{maker, checker} {
		for _, p := range []string{"hr.people.read", "hr.employees.manage", "hr.attendance.manage", "hr.settings.manage", "hr.leave.manage", "hr.leave.approve", "hr.loans.manage", "hr.loans.approve", "hr.payroll.manage", "hr.payroll.approve"} {
			store.SeedPermission(scope, a, p)
		}
	}
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: "tenant", CompanyID: "company", StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	emp, e := svc.CreateEmployee(ctx, people.CreateEmployeeCommand{Scope: scope, Number: "emp-1", FullName: "Asha Mrema", JobTitle: "Accountant", Department: "Finance", Currency: "TZS", HireDate: at.AddDate(-1, 0, 0), BaseSalaryMinor: 100000, ActorID: maker, IdempotencyKey: "employee-create-0001"})
	if e != nil {
		t.Fatal(e)
	}
	lt, e := svc.CreateLeaveType(ctx, people.LeaveTypeCommand{Scope: scope, Code: "ANNUAL", NameEN: "Annual leave", NameSW: "Likizo ya mwaka", AnnualEntitlementDays: 28, EffectiveFrom: at, ActorID: maker, IdempotencyKey: "leave-type-create1"})
	if e != nil {
		t.Fatal(e)
	}
	lv, e := svc.CreateLeave(ctx, people.LeaveCommand{Scope: scope, EmployeeID: emp.ID, LeaveTypeID: lt.ID, StartsOn: at.AddDate(0, 0, 10), EndsOn: at.AddDate(0, 0, 12), Reason: "Approved annual leave request", ActorID: maker, IdempotencyKey: "leave-request-0001"})
	if e != nil {
		t.Fatal(e)
	}
	lv, e = svc.TransitionLeave(ctx, people.TransitionCommand{Scope: scope, ID: lv.ID, Status: people.Submitted, Reason: "Leave evidence submitted for review", ActorID: maker, IdempotencyKey: "leave-submit-00001"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = svc.TransitionLeave(ctx, people.TransitionCommand{Scope: scope, ID: lv.ID, Status: people.Approved, Reason: "Maker attempted own approval", ActorID: maker, IdempotencyKey: "leave-self-approve"})
	if !errors.Is(e, people.ErrSeparationOfDuties) {
		t.Fatalf("expected separation error, got %v", e)
	}
	lv, e = svc.TransitionLeave(ctx, people.TransitionCommand{Scope: scope, ID: lv.ID, Status: people.Approved, Reason: "Independent leave approval complete", ActorID: checker, IdempotencyKey: "leave-approve-0001"})
	if e != nil || lv.Days != 3 {
		t.Fatalf("leave %+v %v", lv, e)
	}
	loan, e := svc.CreateLoan(ctx, people.LoanCommand{Scope: scope, EmployeeID: emp.ID, Reference: "loan-1", Currency: "TZS", PrincipalMinor: 20000, ReceivableAccountID: "employee-loan", BankAccountID: "bank", Reason: "Approved employee welfare loan", ActorID: maker, IdempotencyKey: "employee-loan-create"})
	if e != nil {
		t.Fatal(e)
	}
	for i, status := range []people.WorkflowStatus{people.Submitted, people.Approved, people.Posted} {
		actor := maker
		if status != people.Submitted {
			actor = checker
		}
		loan, e = svc.TransitionLoan(ctx, people.TransitionCommand{Scope: scope, ID: loan.ID, Status: status, Reason: "Employee loan evidence and approval complete", ActorID: actor, IdempotencyKey: []string{"employee-loan-submit", "employee-loan-approve", "employee-loan-posted"}[i]})
		if e != nil {
			t.Fatal(e)
		}
	}
	pay, e := svc.CreatePayroll(ctx, people.PayrollCommand{Scope: scope, Reference: "pay-aug", Currency: "TZS", PeriodStart: at, PeriodEnd: at.AddDate(0, 0, 20), PaymentDate: at, SalaryExpenseAccountID: "salary-expense", PayrollPayableAccountID: "salary-payable", DeductionLiabilityAccountID: "deductions-payable", Reason: "Approved payroll inputs for August", Lines: []people.PayrollLine{{EmployeeID: emp.ID, GrossMinor: 100000, OtherDeductionsMinor: 10000, LoanDeductionMinor: 5000}}, ActorID: maker, IdempotencyKey: "payroll-create-0001"})
	if e != nil {
		t.Fatal(e)
	}
	for i, status := range []people.WorkflowStatus{people.Submitted, people.Approved, people.Posted} {
		actor := maker
		if status != people.Submitted {
			actor = checker
		}
		pay, e = svc.TransitionPayroll(ctx, people.TransitionCommand{Scope: scope, ID: pay.ID, Status: status, Reason: "Payroll independently controlled and posted", ActorID: actor, IdempotencyKey: []string{"payroll-submit-0001", "payroll-approve-001", "payroll-posting-001"}[i]})
		if e != nil {
			t.Fatal(e)
		}
	}
	view, e := svc.Get(ctx, scope, checker)
	if e != nil {
		t.Fatal(e)
	}
	if pay.NetMinor != 85000 || len(view.PayrollRuns) != 1 || view.Loans[0].OutstandingMinor != 15000 {
		t.Fatalf("unexpected payroll or loan: %+v %+v", pay, view.Loans)
	}
	journals := store.Snapshot().Journals
	if len(journals) != 2 {
		t.Fatalf("expected loan and payroll journals, got %d", len(journals))
	}
	for _, j := range journals {
		var debit, credit int64
		for _, line := range j.Entries {
			debit += line.DebitMinor
			credit += line.CreditMinor
		}
		if debit != credit {
			t.Fatalf("unbalanced journal %+v", j)
		}
	}
}
