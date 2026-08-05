// Package workforce owns governed scheduling, employee-document metadata, and reproducible payroll output artifacts.
package workforce

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Status string
type Classification string
type ExportFormat string

const (
	Draft        Status         = "DRAFT"
	Submitted    Status         = "SUBMITTED"
	Active       Status         = "ACTIVE"
	Approved     Status         = "APPROVED"
	Rejected     Status         = "REJECTED"
	Internal     Classification = "INTERNAL"
	Confidential Classification = "CONFIDENTIAL"
	Restricted   Classification = "RESTRICTED"
	BankCSV      ExportFormat   = "BANK_CSV"
	StatutoryCSV ExportFormat   = "STATUTORY_CSV"
)

type ShiftTemplate struct {
	ID            string        `json:"id"`
	Scope         tenancy.Scope `json:"scope"`
	Code          string        `json:"code"`
	NameEN        string        `json:"name_en"`
	NameSW        string        `json:"name_sw"`
	Status        Status        `json:"status"`
	StartMinute   int64         `json:"start_minute"`
	EndMinute     int64         `json:"end_minute"`
	BreakMinutes  int64         `json:"break_minutes"`
	WeekdayMask   int64         `json:"weekday_mask"`
	EffectiveFrom time.Time     `json:"effective_from"`
	EffectiveTo   *time.Time    `json:"effective_to,omitempty"`
	Reason        string        `json:"reason"`
	CreatedBy     string        `json:"created_by"`
	CreatedAt     time.Time     `json:"created_at"`
	ApprovedBy    string        `json:"approved_by,omitempty"`
}
type Assignment struct {
	ID              string        `json:"id"`
	Scope           tenancy.Scope `json:"scope"`
	EmployeeID      string        `json:"employee_id"`
	ShiftTemplateID string        `json:"shift_template_id"`
	Status          Status        `json:"status"`
	StartsOn        time.Time     `json:"starts_on"`
	EndsOn          time.Time     `json:"ends_on"`
	Reason          string        `json:"reason"`
	CreatedBy       string        `json:"created_by"`
	CreatedAt       time.Time     `json:"created_at"`
	ApprovedBy      string        `json:"approved_by,omitempty"`
}
type EmployeeDocument struct {
	ID             string         `json:"id"`
	Scope          tenancy.Scope  `json:"scope"`
	EmployeeID     string         `json:"employee_id"`
	DocumentType   string         `json:"document_type"`
	Title          string         `json:"title"`
	ObjectKey      string         `json:"object_key"`
	SHA256         string         `json:"sha256"`
	MediaType      string         `json:"media_type"`
	Classification Classification `json:"classification"`
	IssuedOn       *time.Time     `json:"issued_on,omitempty"`
	ExpiresOn      *time.Time     `json:"expires_on,omitempty"`
	Reason         string         `json:"reason"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
}
type PayrollArtifact struct {
	ID                  string        `json:"id"`
	Scope               tenancy.Scope `json:"scope"`
	PayrollRunID        string        `json:"payroll_run_id"`
	ConfigurationID     string        `json:"configuration_id"`
	Format              ExportFormat  `json:"format"`
	FileName            string        `json:"file_name"`
	MediaType           string        `json:"media_type"`
	SHA256              string        `json:"sha256"`
	ConfigurationSHA256 string        `json:"configuration_sha256"`
	RowCount            int64         `json:"row_count"`
	Content             string        `json:"content,omitempty"`
	CreatedBy           string        `json:"created_by"`
	CreatedAt           time.Time     `json:"created_at"`
}
type Snapshot struct {
	Templates        []ShiftTemplate    `json:"templates"`
	Assignments      []Assignment       `json:"assignments"`
	Documents        []EmployeeDocument `json:"documents"`
	PayrollArtifacts []PayrollArtifact  `json:"payroll_artifacts"`
}

var (
	ErrInvalidCommand     = errors.New("workforce command is invalid")
	ErrInvalidTransition  = errors.New("workforce transition is invalid")
	ErrSeparationOfDuties = errors.New("maker cannot approve own workforce record")
	ErrOverlap            = errors.New("approved shift assignment overlaps an existing assignment")
	ErrConfiguration      = errors.New("approved effective payroll export configuration is required")
	ErrPayrollNotPosted   = errors.New("payroll must be posted before export")
)
