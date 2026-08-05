package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/workforce"
)

func workforceAuth(ctx context.Context, tx *transaction, scope tenancy.Scope, actor, permission string) error {
	ok, e := tx.Authorize(ctx, scope, actor, permission)
	if e != nil {
		return e
	}
	if !ok {
		return sales.ErrForbidden
	}
	return nil
}
func (s *Store) WorkforceSnapshot(ctx context.Context, scope tenancy.Scope, actor string) (workforce.Snapshot, error) {
	v := workforce.Snapshot{Templates: []workforce.ShiftTemplate{}, Assignments: []workforce.Assignment{}, Documents: []workforce.EmployeeDocument{}, PayrollArtifacts: []workforce.PayrollArtifact{}}
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if x := workforceAuth(ctx, tx, scope, actor, "hr.workforce.read"); x != nil {
			return x
		}
		canReadDocuments, x := tx.Authorize(ctx, scope, actor, "hr.documents.read")
		if x != nil {
			return x
		}
		if !canReadDocuments {
			canReadDocuments, x = tx.Authorize(ctx, scope, actor, "hr.documents.manage")
			if x != nil {
				return x
			}
		}
		rows, x := tx.tx.Query(ctx, `SELECT id::text,code,name_en,name_sw,status,start_minute,end_minute,break_minutes,weekday_mask,effective_from,effective_to,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM shift_templates WHERE tenant_id=$1 AND company_id=$2 ORDER BY effective_from DESC,code`, scope.TenantID, scope.CompanyID)
		if x != nil {
			return normalizeError(x)
		}
		for rows.Next() {
			item := workforce.ShiftTemplate{Scope: scope}
			if x = rows.Scan(&item.ID, &item.Code, &item.NameEN, &item.NameSW, &item.Status, &item.StartMinute, &item.EndMinute, &item.BreakMinutes, &item.WeekdayMask, &item.EffectiveFrom, &item.EffectiveTo, &item.Reason, &item.CreatedBy, &item.CreatedAt, &item.ApprovedBy); x != nil {
				rows.Close()
				return normalizeError(x)
			}
			v.Templates = append(v.Templates, item)
		}
		rows.Close()
		rows, x = tx.tx.Query(ctx, `SELECT id::text,employee_id::text,shift_template_id::text,status,starts_on,ends_on,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM shift_assignments WHERE tenant_id=$1 AND company_id=$2 ORDER BY starts_on DESC,id`, scope.TenantID, scope.CompanyID)
		if x != nil {
			return normalizeError(x)
		}
		for rows.Next() {
			item := workforce.Assignment{Scope: scope}
			if x = rows.Scan(&item.ID, &item.EmployeeID, &item.ShiftTemplateID, &item.Status, &item.StartsOn, &item.EndsOn, &item.Reason, &item.CreatedBy, &item.CreatedAt, &item.ApprovedBy); x != nil {
				rows.Close()
				return normalizeError(x)
			}
			v.Assignments = append(v.Assignments, item)
		}
		rows.Close()
		if canReadDocuments {
			rows, x = tx.tx.Query(ctx, `SELECT id::text,employee_id::text,document_type,title,object_key,sha256,media_type,classification,issued_on,expires_on,reason,created_by::text,created_at FROM employee_documents WHERE tenant_id=$1 AND company_id=$2 ORDER BY expires_on NULLS LAST,created_at DESC`, scope.TenantID, scope.CompanyID)
			if x != nil {
				return normalizeError(x)
			}
			for rows.Next() {
				item := workforce.EmployeeDocument{Scope: scope}
				if x = rows.Scan(&item.ID, &item.EmployeeID, &item.DocumentType, &item.Title, &item.ObjectKey, &item.SHA256, &item.MediaType, &item.Classification, &item.IssuedOn, &item.ExpiresOn, &item.Reason, &item.CreatedBy, &item.CreatedAt); x != nil {
					rows.Close()
					return normalizeError(x)
				}
				v.Documents = append(v.Documents, item)
			}
			rows.Close()
		}
		rows, x = tx.tx.Query(ctx, `SELECT id::text,payroll_run_id::text,configuration_id::text,format,file_name,media_type,sha256,configuration_sha256,row_count,created_by::text,created_at FROM payroll_export_artifacts WHERE tenant_id=$1 AND company_id=$2 ORDER BY created_at DESC`, scope.TenantID, scope.CompanyID)
		if x != nil {
			return normalizeError(x)
		}
		for rows.Next() {
			item := workforce.PayrollArtifact{Scope: scope}
			if x = rows.Scan(&item.ID, &item.PayrollRunID, &item.ConfigurationID, &item.Format, &item.FileName, &item.MediaType, &item.SHA256, &item.ConfigurationSHA256, &item.RowCount, &item.CreatedBy, &item.CreatedAt); x != nil {
				rows.Close()
				return normalizeError(x)
			}
			v.PayrollArtifacts = append(v.PayrollArtifacts, item)
		}
		rows.Close()
		return nil
	})
	return v, e
}

func (s *Store) CreateShiftTemplate(ctx context.Context, v workforce.ShiftTemplate, idem, hash string) (workforce.ShiftTemplate, error) {
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if x := workforceAuth(ctx, tx, v.Scope, v.CreatedBy, "hr.shifts.manage"); x != nil {
			return x
		}
		acquired, id, x := tx.ClaimIdempotency(ctx, v.Scope, "hr.shift-template.create.v1", idem, hash)
		if x != nil {
			return x
		}
		if !acquired {
			v.ID = id
			return nil
		}
		_, x = tx.tx.Exec(ctx, `INSERT INTO shift_templates(id,tenant_id,company_id,code,name_en,name_sw,status,start_minute,end_minute,break_minutes,weekday_mask,effective_from,effective_to,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Code, v.NameEN, v.NameSW, v.Status, v.StartMinute, v.EndMinute, v.BreakMinutes, v.WeekdayMask, v.EffectiveFrom, v.EffectiveTo, v.Reason, v.CreatedBy, v.CreatedAt)
		if x != nil {
			return normalizeError(x)
		}
		if x = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.shift_template_created", "shift_template", v.ID, v.ID, v.CreatedAt); x != nil {
			return x
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.shift-template.create.v1", idem, v.ID)
	})
	return v, e
}
func (s *Store) TransitionShiftTemplate(ctx context.Context, scope tenancy.Scope, actor, id string, to workforce.Status, reason, idem, hash string, at time.Time) (workforce.ShiftTemplate, error) {
	v := workforce.ShiftTemplate{ID: id, Scope: scope}
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		permission := "hr.shifts.manage"
		if to == workforce.Active || to == workforce.Rejected {
			permission = "hr.shifts.approve"
		}
		if x := workforceAuth(ctx, tx, scope, actor, permission); x != nil {
			return x
		}
		op := "hr.shift-template.transition." + string(to) + ".v1"
		acquired, _, x := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if x != nil {
			return x
		}
		if !acquired {
			v.Status = to
			return nil
		}
		var from workforce.Status
		if x = tx.tx.QueryRow(ctx, `SELECT code,name_en,name_sw,status,start_minute,end_minute,break_minutes,weekday_mask,effective_from,effective_to,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM shift_templates WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.Code, &v.NameEN, &v.NameSW, &from, &v.StartMinute, &v.EndMinute, &v.BreakMinutes, &v.WeekdayMask, &v.EffectiveFrom, &v.EffectiveTo, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); x != nil {
			return normalizeError(x)
		}
		valid := from == workforce.Draft && to == workforce.Submitted || from == workforce.Submitted && (to == workforce.Active || to == workforce.Rejected)
		if !valid {
			return workforce.ErrInvalidTransition
		}
		if (to == workforce.Active || to == workforce.Rejected) && v.CreatedBy == actor {
			return workforce.ErrSeparationOfDuties
		}
		if to == workforce.Active {
			var overlap bool
			if x = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shift_templates WHERE tenant_id=$1 AND company_id=$2 AND code=$3 AND status='ACTIVE' AND id<>$4 AND NOT(COALESCE(effective_to,'infinity'::date)<$5 OR COALESCE($6::date,'infinity'::date)<effective_from))`, scope.TenantID, scope.CompanyID, v.Code, id, v.EffectiveFrom, v.EffectiveTo).Scan(&overlap); x != nil {
				return normalizeError(x)
			}
			if overlap {
				return workforce.ErrOverlap
			}
		}
		_, x = tx.tx.Exec(ctx, `UPDATE shift_templates SET status=$1,approved_by=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if x != nil {
			return normalizeError(x)
		}
		if _, x = tx.tx.Exec(ctx, `INSERT INTO workforce_transitions(tenant_id,company_id,entity_type,entity_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,'SHIFT_TEMPLATE',$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at); x != nil {
			return normalizeError(x)
		}
		if x = tx.financialEvidence(ctx, scope, actor, "hr.shift_template_"+string(to), "shift_template", id, id, at); x != nil {
			return x
		}
		if x = tx.CompleteIdempotency(ctx, scope, op, idem, id); x != nil {
			return x
		}
		v.Status = to
		if to == workforce.Active {
			v.ApprovedBy = actor
		}
		return nil
	})
	return v, e
}

func (s *Store) CreateShiftAssignment(ctx context.Context, v workforce.Assignment, idem, hash string) (workforce.Assignment, error) {
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if x := workforceAuth(ctx, tx, v.Scope, v.CreatedBy, "hr.shifts.manage"); x != nil {
			return x
		}
		acquired, id, x := tx.ClaimIdempotency(ctx, v.Scope, "hr.shift-assignment.create.v1", idem, hash)
		if x != nil {
			return x
		}
		if !acquired {
			v.ID = id
			return nil
		}
		var valid bool
		if x = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM employees e JOIN shift_templates t ON t.tenant_id=e.tenant_id AND t.company_id=e.company_id WHERE e.tenant_id=$1 AND e.company_id=$2 AND e.id=$3 AND e.status='ACTIVE' AND t.id=$4 AND t.status='ACTIVE' AND t.effective_from<=$5 AND COALESCE(t.effective_to,'infinity'::date)>=$6)`, v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID, v.ShiftTemplateID, v.StartsOn, v.EndsOn).Scan(&valid); x != nil {
			return normalizeError(x)
		}
		if !valid {
			return workforce.ErrInvalidCommand
		}
		_, x = tx.tx.Exec(ctx, `INSERT INTO shift_assignments(id,tenant_id,company_id,employee_id,shift_template_id,status,starts_on,ends_on,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID, v.ShiftTemplateID, v.Status, v.StartsOn, v.EndsOn, v.Reason, v.CreatedBy, v.CreatedAt)
		if x != nil {
			return normalizeError(x)
		}
		if x = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.shift_assignment_created", "shift_assignment", v.ID, v.ID, v.CreatedAt); x != nil {
			return x
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.shift-assignment.create.v1", idem, v.ID)
	})
	return v, e
}
func (s *Store) TransitionShiftAssignment(ctx context.Context, scope tenancy.Scope, actor, id string, to workforce.Status, reason, idem, hash string, at time.Time) (workforce.Assignment, error) {
	v := workforce.Assignment{ID: id, Scope: scope}
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		permission := "hr.shifts.manage"
		if to == workforce.Approved || to == workforce.Rejected {
			permission = "hr.shifts.approve"
		}
		if x := workforceAuth(ctx, tx, scope, actor, permission); x != nil {
			return x
		}
		op := "hr.shift-assignment.transition." + string(to) + ".v1"
		acquired, _, x := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if x != nil {
			return x
		}
		if !acquired {
			v.Status = to
			return nil
		}
		var from workforce.Status
		if x = tx.tx.QueryRow(ctx, `SELECT employee_id::text,shift_template_id::text,status,starts_on,ends_on,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM shift_assignments WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&v.EmployeeID, &v.ShiftTemplateID, &from, &v.StartsOn, &v.EndsOn, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); x != nil {
			return normalizeError(x)
		}
		valid := from == workforce.Draft && to == workforce.Submitted || from == workforce.Submitted && (to == workforce.Approved || to == workforce.Rejected)
		if !valid {
			return workforce.ErrInvalidTransition
		}
		if (to == workforce.Approved || to == workforce.Rejected) && v.CreatedBy == actor {
			return workforce.ErrSeparationOfDuties
		}
		if to == workforce.Approved {
			var overlap bool
			if x = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shift_assignments WHERE tenant_id=$1 AND company_id=$2 AND employee_id=$3 AND status='APPROVED' AND id<>$4 AND NOT(ends_on<$5 OR starts_on>$6))`, scope.TenantID, scope.CompanyID, v.EmployeeID, id, v.StartsOn, v.EndsOn).Scan(&overlap); x != nil {
				return normalizeError(x)
			}
			if overlap {
				return workforce.ErrOverlap
			}
		}
		_, x = tx.tx.Exec(ctx, `UPDATE shift_assignments SET status=$1,approved_by=CASE WHEN $1 IN('APPROVED','REJECTED') THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1 IN('APPROVED','REJECTED') THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if x != nil {
			return normalizeError(x)
		}
		if _, x = tx.tx.Exec(ctx, `INSERT INTO workforce_transitions(tenant_id,company_id,entity_type,entity_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,'SHIFT_ASSIGNMENT',$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at); x != nil {
			return normalizeError(x)
		}
		if x = tx.financialEvidence(ctx, scope, actor, "hr.shift_assignment_"+string(to), "shift_assignment", id, id, at); x != nil {
			return x
		}
		if x = tx.CompleteIdempotency(ctx, scope, op, idem, id); x != nil {
			return x
		}
		v.Status = to
		if to == workforce.Approved {
			v.ApprovedBy = actor
		}
		return nil
	})
	return v, e
}

func (s *Store) RegisterEmployeeDocument(ctx context.Context, v workforce.EmployeeDocument, idem, hash string) (workforce.EmployeeDocument, error) {
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if x := workforceAuth(ctx, tx, v.Scope, v.CreatedBy, "hr.documents.manage"); x != nil {
			return x
		}
		acquired, id, x := tx.ClaimIdempotency(ctx, v.Scope, "hr.employee-document.register.v1", idem, hash)
		if x != nil {
			return x
		}
		if !acquired {
			v.ID = id
			return nil
		}
		result, x := tx.tx.Exec(ctx, `INSERT INTO employee_documents(id,tenant_id,company_id,employee_id,document_type,title,object_key,sha256,media_type,classification,issued_on,expires_on,reason,created_by,created_at) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15 WHERE EXISTS(SELECT 1 FROM employees WHERE tenant_id=$2 AND company_id=$3 AND id=$4)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.EmployeeID, v.DocumentType, v.Title, v.ObjectKey, v.SHA256, v.MediaType, v.Classification, v.IssuedOn, v.ExpiresOn, v.Reason, v.CreatedBy, v.CreatedAt)
		if x != nil {
			return normalizeError(x)
		}
		if result.RowsAffected() != 1 {
			return sales.ErrNotFound
		}
		if x = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.employee_document_registered", "employee_document", v.ID, v.ID, v.CreatedAt); x != nil {
			return x
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.employee-document.register.v1", idem, v.ID)
	})
	return v, e
}
func (s *Store) GeneratePayrollArtifact(ctx context.Context, v workforce.PayrollArtifact, idem, hash string) (workforce.PayrollArtifact, error) {
	e := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if x := workforceAuth(ctx, tx, v.Scope, v.CreatedBy, "hr.payroll.export"); x != nil {
			return x
		}
		acquired, id, x := tx.ClaimIdempotency(ctx, v.Scope, "hr.payroll-export.generate.v1", idem, hash)
		if x != nil {
			return x
		}
		if !acquired {
			return tx.tx.QueryRow(ctx, `SELECT id::text,payroll_run_id::text,configuration_id::text,format,file_name,media_type,sha256,configuration_sha256,row_count,content,created_by::text,created_at FROM payroll_export_artifacts WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, id).Scan(&v.ID, &v.PayrollRunID, &v.ConfigurationID, &v.Format, &v.FileName, &v.MediaType, &v.SHA256, &v.ConfigurationSHA256, &v.RowCount, &v.Content, &v.CreatedBy, &v.CreatedAt)
		}
		var status, currency, reference string
		var payment time.Time
		if x = tx.tx.QueryRow(ctx, `SELECT status,currency,reference,payment_date FROM payroll_runs WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.PayrollRunID).Scan(&status, &currency, &reference, &payment); x != nil {
			return normalizeError(x)
		}
		if status != "POSTED" {
			return workforce.ErrPayrollNotPosted
		}
		var raw []byte
		var category, configStatus string
		var from time.Time
		var to *time.Time
		if x = tx.tx.QueryRow(ctx, `SELECT category,status,effective_from,effective_to,value FROM configuration_versions WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.ConfigurationID).Scan(&category, &configStatus, &from, &to, &raw); x != nil {
			return normalizeError(x)
		}
		if configStatus != "ACTIVE" || (category != "HR" && category != "INTEGRATIONS") || from.After(payment) || (to != nil && to.Before(payment)) {
			return workforce.ErrConfiguration
		}
		rows, x := tx.tx.Query(ctx, `SELECT e.employee_number,e.full_name,l.gross_minor,l.other_deductions_minor,l.loan_deduction_minor,l.net_minor FROM payroll_lines l JOIN employees e ON e.tenant_id=l.tenant_id AND e.company_id=l.company_id AND e.id=l.employee_id WHERE l.tenant_id=$1 AND l.company_id=$2 AND l.payroll_run_id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.PayrollRunID)
		if x != nil {
			return normalizeError(x)
		}
		data := []workforce.ExportRow{}
		for rows.Next() {
			row := workforce.ExportRow{Currency: currency, PaymentDate: payment.Format("2006-01-02"), PayrollReference: reference}
			if x = rows.Scan(&row.EmployeeNumber, &row.FullName, &row.GrossMinor, &row.OtherDeductionsMinor, &row.LoanDeductionMinor, &row.NetMinor); x != nil {
				rows.Close()
				return normalizeError(x)
			}
			data = append(data, row)
		}
		rows.Close()
		content, prefix, x := workforce.BuildPayrollCSV(raw, v.Format, data)
		if x != nil {
			return x
		}
		sum := sha256.Sum256([]byte(content))
		cfg := sha256.Sum256(raw)
		v.Content = content
		v.FileName = workforce.PayrollFileName(prefix, reference)
		v.SHA256 = hex.EncodeToString(sum[:])
		v.ConfigurationSHA256 = hex.EncodeToString(cfg[:])
		v.RowCount = int64(len(data))
		_, x = tx.tx.Exec(ctx, `INSERT INTO payroll_export_artifacts(id,tenant_id,company_id,payroll_run_id,configuration_id,format,file_name,media_type,sha256,configuration_sha256,row_count,content,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.PayrollRunID, v.ConfigurationID, v.Format, v.FileName, v.MediaType, v.SHA256, v.ConfigurationSHA256, v.RowCount, v.Content, v.CreatedBy, v.CreatedAt)
		if x != nil {
			return normalizeError(x)
		}
		if x = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "hr.payroll_export_generated", "payroll_export", v.ID, v.ID, v.CreatedAt); x != nil {
			return x
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "hr.payroll-export.generate.v1", idem, v.ID)
	})
	return v, e
}
