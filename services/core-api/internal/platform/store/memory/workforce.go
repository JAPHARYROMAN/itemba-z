package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/workforce"
)

func (s *Store) WorkforceSnapshot(_ context.Context, scope tenancy.Scope, actor string) (workforce.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "hr.workforce.read")] {
		return workforce.Snapshot{}, sales.ErrForbidden
	}
	v := workforce.Snapshot{Templates: []workforce.ShiftTemplate{}, Assignments: []workforce.Assignment{}, Documents: []workforce.EmployeeDocument{}, PayrollArtifacts: []workforce.PayrollArtifact{}}
	for _, x := range s.state.shiftTemplates {
		if x.Scope.TenantID == scope.TenantID && x.Scope.CompanyID == scope.CompanyID {
			v.Templates = append(v.Templates, x)
		}
	}
	for _, x := range s.state.shiftAssignments {
		if x.Scope.TenantID == scope.TenantID && x.Scope.CompanyID == scope.CompanyID {
			v.Assignments = append(v.Assignments, x)
		}
	}
	if s.state.permissions[permissionKey(scope, actor, "hr.documents.read")] || s.state.permissions[permissionKey(scope, actor, "hr.documents.manage")] {
		for _, x := range s.state.employeeDocuments {
			if x.Scope.TenantID == scope.TenantID && x.Scope.CompanyID == scope.CompanyID {
				v.Documents = append(v.Documents, x)
			}
		}
	}
	for _, x := range s.state.payrollArtifacts {
		if x.Scope.TenantID == scope.TenantID && x.Scope.CompanyID == scope.CompanyID {
			x.Content = ""
			v.PayrollArtifacts = append(v.PayrollArtifacts, x)
		}
	}
	return v, nil
}
func (s *Store) CreateShiftTemplate(_ context.Context, v workforce.ShiftTemplate, idem, hash string) (workforce.ShiftTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.shifts.manage")] {
		return v, sales.ErrForbidden
	}
	if id, ok, e := s.memoryIdem(v.Scope, "hr.shift-template.create.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.shiftTemplates[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, id)], nil
	}
	s.state.shiftTemplates[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.shift-template.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionShiftTemplate(_ context.Context, scope tenancy.Scope, actor, id string, to workforce.Status, _ string, idem, hash string, _ time.Time) (workforce.ShiftTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "hr.shifts.manage"
	if to == workforce.Active || to == workforce.Rejected {
		perm = "hr.shifts.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return workforce.ShiftTemplate{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	v, ok := s.state.shiftTemplates[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	if _, hit, e := s.memoryIdem(scope, "hr.shift-template.transition."+string(to)+".v1", idem, hash); e != nil {
		return v, e
	} else if hit {
		return v, nil
	}
	valid := v.Status == workforce.Draft && to == workforce.Submitted || v.Status == workforce.Submitted && (to == workforce.Active || to == workforce.Rejected)
	if !valid {
		return v, workforce.ErrInvalidTransition
	}
	if (to == workforce.Active || to == workforce.Rejected) && v.CreatedBy == actor {
		return v, workforce.ErrSeparationOfDuties
	}
	v.Status = to
	if to == workforce.Active {
		v.ApprovedBy = actor
	}
	s.state.shiftTemplates[k] = v
	s.putIdem(scope, "hr.shift-template.transition."+string(to)+".v1", idem, hash, id)
	return v, nil
}
func (s *Store) CreateShiftAssignment(_ context.Context, v workforce.Assignment, idem, hash string) (workforce.Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.shifts.manage")] {
		return v, sales.ErrForbidden
	}
	if id, ok, e := s.memoryIdem(v.Scope, "hr.shift-assignment.create.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.shiftAssignments[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, id)], nil
	}
	employee, ok := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID)]
	if !ok || employee.Status != people.EmployeeActive {
		return v, sales.ErrNotFound
	}
	template, ok := s.state.shiftTemplates[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ShiftTemplateID)]
	if !ok || template.Status != workforce.Active {
		return v, workforce.ErrInvalidCommand
	}
	s.state.shiftAssignments[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.shift-assignment.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionShiftAssignment(_ context.Context, scope tenancy.Scope, actor, id string, to workforce.Status, _ string, idem, hash string, _ time.Time) (workforce.Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "hr.shifts.manage"
	if to == workforce.Approved || to == workforce.Rejected {
		perm = "hr.shifts.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return workforce.Assignment{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	v, ok := s.state.shiftAssignments[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	if resultID, hit, e := s.memoryIdem(scope, "hr.shift-assignment.transition."+string(to)+".v1", idem, hash); e != nil {
		return v, e
	} else if hit {
		return s.state.shiftAssignments[companyEntityKey(scope.TenantID, scope.CompanyID, resultID)], nil
	}
	valid := v.Status == workforce.Draft && to == workforce.Submitted || v.Status == workforce.Submitted && (to == workforce.Approved || to == workforce.Rejected)
	if !valid {
		return v, workforce.ErrInvalidTransition
	}
	if (to == workforce.Approved || to == workforce.Rejected) && v.CreatedBy == actor {
		return v, workforce.ErrSeparationOfDuties
	}
	if to == workforce.Approved {
		for _, x := range s.state.shiftAssignments {
			if x.ID != v.ID && x.EmployeeID == v.EmployeeID && x.Status == workforce.Approved && !x.EndsOn.Before(v.StartsOn) && !x.StartsOn.After(v.EndsOn) {
				return v, workforce.ErrOverlap
			}
		}
		v.ApprovedBy = actor
	}
	v.Status = to
	s.state.shiftAssignments[k] = v
	s.putIdem(scope, "hr.shift-assignment.transition."+string(to)+".v1", idem, hash, id)
	return v, nil
}
func (s *Store) RegisterEmployeeDocument(_ context.Context, v workforce.EmployeeDocument, idem, hash string) (workforce.EmployeeDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.documents.manage")] {
		return v, sales.ErrForbidden
	}
	if id, ok, e := s.memoryIdem(v.Scope, "hr.employee-document.register.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.employeeDocuments[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, id)], nil
	}
	if _, ok := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID)]; !ok {
		return v, sales.ErrNotFound
	}
	s.state.employeeDocuments[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.employee-document.register.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) GeneratePayrollArtifact(_ context.Context, v workforce.PayrollArtifact, idem, hash string) (workforce.PayrollArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "hr.payroll.export")] {
		return v, sales.ErrForbidden
	}
	if id, ok, e := s.memoryIdem(v.Scope, "hr.payroll-export.generate.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.payrollArtifacts[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, id)], nil
	}
	run, ok := s.state.payrollRuns[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.PayrollRunID)]
	if !ok || run.Status != people.Posted {
		return v, workforce.ErrPayrollNotPosted
	}
	config, ok := s.state.configurations[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ConfigurationID)]
	if !ok || config.Status != configuration.Active || (config.Category != configuration.HR && config.Category != configuration.Integrations) || config.EffectiveFrom.After(run.PaymentDate) || (config.EffectiveTo != nil && config.EffectiveTo.Before(run.PaymentDate)) {
		return v, workforce.ErrConfiguration
	}
	rows := []workforce.ExportRow{}
	for _, line := range run.Lines {
		employee := s.state.employees[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, line.EmployeeID)]
		rows = append(rows, workforce.ExportRow{EmployeeNumber: employee.Number, FullName: employee.FullName, Currency: run.Currency, PaymentDate: run.PaymentDate.Format("2006-01-02"), PayrollReference: run.Reference, GrossMinor: line.GrossMinor, OtherDeductionsMinor: line.OtherDeductionsMinor, LoanDeductionMinor: line.LoanDeductionMinor, NetMinor: line.NetMinor})
	}
	content, prefix, e := workforce.BuildPayrollCSV(config.Value, v.Format, rows)
	if e != nil {
		return v, e
	}
	sum := sha256.Sum256([]byte(content))
	cfg := sha256.Sum256(config.Value)
	v.Content = content
	v.FileName = workforce.PayrollFileName(prefix, run.Reference)
	v.SHA256 = hex.EncodeToString(sum[:])
	v.ConfigurationSHA256 = hex.EncodeToString(cfg[:])
	v.RowCount = int64(len(rows))
	s.state.payrollArtifacts[companyEntityKey(v.Scope.TenantID, v.Scope.CompanyID, v.ID)] = v
	s.putIdem(v.Scope, "hr.payroll-export.generate.v1", idem, hash, v.ID)
	return v, nil
}
