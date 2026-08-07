package workforce_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/workforce"
)

func TestGovernedShiftAssignmentAndDocument(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 6, 9, 0, 0, 0, time.UTC)
	store := memory.New()
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	store.SeedPermission(scope, "scheduler", "hr.workforce.read")
	for _, actor := range []string{"maker", "checker"} {
		for _, permission := range []string{"hr.employees.manage", "hr.workforce.read", "hr.shifts.manage", "hr.shifts.approve", "hr.documents.read", "hr.documents.manage"} {
			store.SeedPermission(scope, actor, permission)
		}
	}
	peopleService, _ := people.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	employee, e := peopleService.CreateEmployee(ctx, people.CreateEmployeeCommand{Scope: scope, Number: "EMP-100", FullName: "Asha Mollel", JobTitle: "Sales Officer", Department: "Commercial", Currency: "TZS", HireDate: at, BaseSalaryMinor: 80000000, ActorID: "maker", IdempotencyKey: "employee-for-workforce-001"})
	if e != nil {
		t.Fatal(e)
	}
	svc, _ := workforce.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	shift, e := svc.CreateShift(ctx, workforce.ShiftCommand{Scope: scope, Code: "DAY", NameEN: "Day shift", NameSW: "Zamu ya mchana", StartMinute: 480, EndMinute: 1020, BreakMinutes: 60, WeekdayMask: 31, EffectiveFrom: at, Reason: "Create approved operating schedule", ActorID: "maker", IdempotencyKey: "create-day-shift-000001"})
	if e != nil {
		t.Fatal(e)
	}
	shift, e = svc.TransitionShift(ctx, workforce.TransitionCommand{Scope: scope, ID: shift.ID, Status: workforce.Submitted, Reason: "Submit schedule for independent review", ActorID: "maker", IdempotencyKey: "submit-day-shift-000001"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.TransitionShift(ctx, workforce.TransitionCommand{Scope: scope, ID: shift.ID, Status: workforce.Active, Reason: "Maker attempts own shift approval", ActorID: "maker", IdempotencyKey: "self-approve-shift-00001"}); !errors.Is(e, workforce.ErrSeparationOfDuties) {
		t.Fatalf("expected separation of duties, got %v", e)
	}
	shift, e = svc.TransitionShift(ctx, workforce.TransitionCommand{Scope: scope, ID: shift.ID, Status: workforce.Active, Reason: "Independent schedule approval complete", ActorID: "checker", IdempotencyKey: "activate-day-shift-00001"})
	if e != nil {
		t.Fatal(e)
	}
	approve := func(start, end time.Time) error {
		assignment, x := svc.CreateAssignment(ctx, workforce.AssignmentCommand{Scope: scope, EmployeeID: employee.ID, ShiftTemplateID: shift.ID, StartsOn: start, EndsOn: end, Reason: "Assign employee to approved schedule", ActorID: "maker", IdempotencyKey: "create-assignment-" + start.Format("20060102")})
		if x != nil {
			return x
		}
		assignment, x = svc.TransitionAssignment(ctx, workforce.TransitionCommand{Scope: scope, ID: assignment.ID, Status: workforce.Submitted, Reason: "Submit assignment for review", ActorID: "maker", IdempotencyKey: "submit-assignment-" + assignment.ID})
		if x != nil {
			return x
		}
		_, x = svc.TransitionAssignment(ctx, workforce.TransitionCommand{Scope: scope, ID: assignment.ID, Status: workforce.Approved, Reason: "Independent assignment approval complete", ActorID: "checker", IdempotencyKey: "approve-assignment" + assignment.ID})
		return x
	}
	if e = approve(at, at.AddDate(0, 0, 6)); e != nil {
		t.Fatal(e)
	}
	if e = approve(at.AddDate(0, 0, 3), at.AddDate(0, 0, 10)); !errors.Is(e, workforce.ErrOverlap) {
		t.Fatalf("expected overlap rejection, got %v", e)
	}
	if _, e = svc.RegisterDocument(ctx, workforce.DocumentCommand{Scope: scope, EmployeeID: employee.ID, DocumentType: "CONTRACT", Title: "Signed employment contract", ObjectKey: "employees/emp-100/contract.pdf", SHA256: strings.Repeat("a", 64), MediaType: "application/pdf", Classification: workforce.Restricted, Reason: "Register verified employee contract", ActorID: "maker", IdempotencyKey: "register-contract-000001"}); e != nil {
		t.Fatal(e)
	}
	snapshot, e := svc.Get(ctx, scope, "checker")
	if e != nil || len(snapshot.Assignments) != 2 || len(snapshot.Documents) != 1 {
		t.Fatalf("unexpected snapshot %+v %v", snapshot, e)
	}
	limited, e := svc.Get(ctx, scope, "scheduler")
	if e != nil || len(limited.Documents) != 0 {
		t.Fatalf("workforce-only reader received sensitive document metadata: %+v %v", limited.Documents, e)
	}
}

func TestConfigurablePayrollCSV(t *testing.T) {
	raw := []byte(`{"format":"BANK_CSV","delimiter":";","include_header":true,"columns":["employee_number","net_minor"],"file_name_prefix":"bank-payroll"}`)
	content, prefix, e := workforce.BuildPayrollCSV(raw, workforce.BankCSV, []workforce.ExportRow{{EmployeeNumber: "EMP-2", NetMinor: 200}, {EmployeeNumber: "EMP-1", NetMinor: 100}})
	if e != nil {
		t.Fatal(e)
	}
	if prefix != "bank-payroll" || content != "employee_number;net_minor\nEMP-1;100\nEMP-2;200\n" {
		t.Fatalf("unexpected export %q %q", prefix, content)
	}
	if name := workforce.PayrollFileName(prefix, "PAY/AUG\r\n"); name != "bank-payroll-PAY_AUG.csv" {
		t.Fatalf("unsafe payroll file name %q", name)
	}
	if _, _, e = workforce.BuildPayrollCSV(raw, workforce.StatutoryCSV, nil); !errors.Is(e, workforce.ErrConfiguration) {
		t.Fatalf("expected format-bound configuration rejection, got %v", e)
	}
}
