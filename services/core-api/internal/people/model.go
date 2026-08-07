// Package people owns employee records, attendance, leave, employee loans, payroll, and payslips.
package people

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type EmployeeStatus string
type WorkflowStatus string

const (
	EmployeeActive   EmployeeStatus = "ACTIVE"
	EmployeeInactive EmployeeStatus = "INACTIVE"
	Draft            WorkflowStatus = "DRAFT"
	Submitted        WorkflowStatus = "SUBMITTED"
	Approved         WorkflowStatus = "APPROVED"
	Posted           WorkflowStatus = "POSTED"
	Rejected         WorkflowStatus = "REJECTED"
)

type Employee struct {
	ID              string         `json:"id"`
	Number          string         `json:"number"`
	FullName        string         `json:"full_name"`
	JobTitle        string         `json:"job_title"`
	Department      string         `json:"department"`
	Currency        string         `json:"currency"`
	Scope           tenancy.Scope  `json:"scope"`
	HireDate        time.Time      `json:"hire_date"`
	Status          EmployeeStatus `json:"status"`
	BaseSalaryMinor int64          `json:"base_salary_minor"`
	CreatedBy       string         `json:"created_by"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Attendance struct {
	ID              string        `json:"id"`
	EmployeeID      string        `json:"employee_id"`
	Reason          string        `json:"reason"`
	RecordedBy      string        `json:"recorded_by"`
	ReversalOf      string        `json:"reversal_of,omitempty"`
	Scope           tenancy.Scope `json:"scope"`
	WorkDate        time.Time     `json:"work_date"`
	RegularMinutes  int64         `json:"regular_minutes"`
	OvertimeMinutes int64         `json:"overtime_minutes"`
	RecordedAt      time.Time     `json:"recorded_at"`
}

type LeaveType struct {
	ID                    string        `json:"id"`
	Code                  string        `json:"code"`
	NameEN                string        `json:"name_en"`
	NameSW                string        `json:"name_sw"`
	Scope                 tenancy.Scope `json:"scope"`
	AnnualEntitlementDays int64         `json:"annual_entitlement_days"`
	EffectiveFrom         time.Time     `json:"effective_from"`
	EffectiveTo           *time.Time    `json:"effective_to,omitempty"`
	CreatedBy             string        `json:"created_by"`
	CreatedAt             time.Time     `json:"created_at"`
}

type LeaveRequest struct {
	ID          string         `json:"id"`
	EmployeeID  string         `json:"employee_id"`
	LeaveTypeID string         `json:"leave_type_id"`
	Reason      string         `json:"reason"`
	CreatedBy   string         `json:"created_by"`
	ApprovedBy  string         `json:"approved_by,omitempty"`
	Scope       tenancy.Scope  `json:"scope"`
	StartsOn    time.Time      `json:"starts_on"`
	EndsOn      time.Time      `json:"ends_on"`
	Days        int64          `json:"days"`
	Status      WorkflowStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Loan struct {
	ID                  string         `json:"id"`
	EmployeeID          string         `json:"employee_id"`
	Reference           string         `json:"reference"`
	Currency            string         `json:"currency"`
	ReceivableAccountID string         `json:"receivable_account_id"`
	BankAccountID       string         `json:"bank_account_id"`
	Reason              string         `json:"reason"`
	CreatedBy           string         `json:"created_by"`
	ApprovedBy          string         `json:"approved_by,omitempty"`
	JournalID           string         `json:"journal_id,omitempty"`
	Scope               tenancy.Scope  `json:"scope"`
	PrincipalMinor      int64          `json:"principal_minor"`
	OutstandingMinor    int64          `json:"outstanding_minor"`
	Status              WorkflowStatus `json:"status"`
	CreatedAt           time.Time      `json:"created_at"`
}

type PayrollLine struct {
	EmployeeID           string `json:"employee_id"`
	GrossMinor           int64  `json:"gross_minor"`
	OtherDeductionsMinor int64  `json:"other_deductions_minor"`
	LoanDeductionMinor   int64  `json:"loan_deduction_minor"`
	NetMinor             int64  `json:"net_minor"`
}

type PayrollRun struct {
	ID                          string         `json:"id"`
	Reference                   string         `json:"reference"`
	Currency                    string         `json:"currency"`
	SalaryExpenseAccountID      string         `json:"salary_expense_account_id"`
	PayrollPayableAccountID     string         `json:"payroll_payable_account_id"`
	DeductionLiabilityAccountID string         `json:"deduction_liability_account_id"`
	Reason                      string         `json:"reason"`
	CreatedBy                   string         `json:"created_by"`
	ApprovedBy                  string         `json:"approved_by,omitempty"`
	JournalID                   string         `json:"journal_id,omitempty"`
	Scope                       tenancy.Scope  `json:"scope"`
	PeriodStart                 time.Time      `json:"period_start"`
	PeriodEnd                   time.Time      `json:"period_end"`
	PaymentDate                 time.Time      `json:"payment_date"`
	Status                      WorkflowStatus `json:"status"`
	Lines                       []PayrollLine  `json:"lines"`
	GrossMinor                  int64          `json:"gross_minor"`
	DeductionsMinor             int64          `json:"deductions_minor"`
	NetMinor                    int64          `json:"net_minor"`
	CreatedAt                   time.Time      `json:"created_at"`
}

type Snapshot struct {
	Employees     []Employee     `json:"employees"`
	Attendance    []Attendance   `json:"attendance"`
	LeaveTypes    []LeaveType    `json:"leave_types"`
	LeaveRequests []LeaveRequest `json:"leave_requests"`
	Loans         []Loan         `json:"loans"`
	PayrollRuns   []PayrollRun   `json:"payroll_runs"`
}

var (
	ErrInvalidCommand          = errors.New("people command is invalid")
	ErrInvalidTransition       = errors.New("people workflow transition is invalid")
	ErrSeparationOfDuties      = errors.New("maker cannot approve own people workflow")
	ErrOverlap                 = errors.New("approved leave overlaps an existing request")
	ErrInsufficientLoanBalance = errors.New("payroll loan deduction exceeds employee loan balance")
)
