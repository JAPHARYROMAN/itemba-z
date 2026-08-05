package postgres

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) AccountActivity(ctx context.Context, scope tenancy.Scope, actor string, from, to time.Time) ([]reporting.AccountActivity, string, error) {
	result := []reporting.AccountActivity{}
	currency := ""
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "reports.financial.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		from, to, e = tx.reportingBounds(ctx, scope, from, to)
		if e != nil {
			return e
		}
		if e = tx.tx.QueryRow(ctx, `SELECT base_currency FROM legal_companies WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.CompanyID).Scan(&currency); e != nil {
			return normalizeError(e)
		}
		var fromValue any = from
		if from.IsZero() {
			fromValue = "-infinity"
		}
		rows, e := tx.tx.Query(ctx, `SELECT a.id,a.code,a.name,a.account_type,COALESCE(a.parent_account_id,''),COALESCE(sum(jl.debit_minor-jl.credit_minor) FILTER(WHERE j.occurred_at<$3::timestamptz),0),COALESCE(sum(jl.debit_minor) FILTER(WHERE j.occurred_at>=$3::timestamptz AND j.occurred_at<$4),0),COALESCE(sum(jl.credit_minor) FILTER(WHERE j.occurred_at>=$3::timestamptz AND j.occurred_at<$4),0) FROM gl_accounts a LEFT JOIN journal_lines jl ON jl.tenant_id=a.tenant_id AND jl.company_id=a.company_id AND jl.account_id=a.id LEFT JOIN journals j ON j.id=jl.journal_id AND j.currency=$5 WHERE a.tenant_id=$1 AND a.company_id=$2 AND a.status IN('ACTIVE','INACTIVE') GROUP BY a.id,a.code,a.name,a.account_type,a.parent_account_id ORDER BY a.code`, scope.TenantID, scope.CompanyID, fromValue, to, currency)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var v reporting.AccountActivity
			if e = rows.Scan(&v.AccountID, &v.Code, &v.Name, &v.Type, &v.ParentAccountID, &v.OpeningMinor, &v.DebitMinor, &v.CreditMinor); e != nil {
				return normalizeError(e)
			}
			result = append(result, v)
		}
		return normalizeError(rows.Err())
	})
	return result, currency, err
}

func (s *Store) LedgerEntries(ctx context.Context, scope tenancy.Scope, actor, accountID string, from, to time.Time) (reporting.GeneralLedger, error) {
	var result reporting.GeneralLedger
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "reports.financial.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		from, to, e = tx.reportingBounds(ctx, scope, from, to)
		if e != nil {
			return e
		}
		if e = tx.tx.QueryRow(ctx, `SELECT a.code,a.name,c.base_currency,COALESCE((SELECT sum(l.debit_minor-l.credit_minor) FROM journal_lines l JOIN journals j ON j.id=l.journal_id WHERE l.tenant_id=a.tenant_id AND l.company_id=a.company_id AND l.account_id=a.id AND j.currency=c.base_currency AND j.occurred_at<$4),0) FROM gl_accounts a JOIN legal_companies c ON c.tenant_id=a.tenant_id AND c.id=a.company_id WHERE a.tenant_id=$1 AND a.company_id=$2 AND a.id=$3 AND a.status IN('ACTIVE','INACTIVE')`, scope.TenantID, scope.CompanyID, accountID, from).Scan(&result.AccountCode, &result.AccountName, &result.Currency, &result.OpeningBalanceMinor); e != nil {
			return normalizeError(e)
		}
		result.AccountID = accountID
		running := result.OpeningBalanceMinor
		rows, e := tx.tx.Query(ctx, `SELECT jl.id,j.id::text,j.occurred_at,j.source_type,j.source_id::text,jl.memo,jl.debit_minor,jl.credit_minor FROM journal_lines jl JOIN journals j ON j.id=jl.journal_id WHERE jl.tenant_id=$1 AND jl.company_id=$2 AND jl.account_id=$3 AND j.currency=$4 AND j.occurred_at>=$5 AND j.occurred_at<$6 ORDER BY j.occurred_at,jl.id`, scope.TenantID, scope.CompanyID, accountID, result.Currency, from, to)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		result.Entries = []reporting.LedgerEntry{}
		for rows.Next() {
			var v reporting.LedgerEntry
			if e = rows.Scan(&v.JournalLineID, &v.JournalID, &v.OccurredAt, &v.SourceType, &v.SourceID, &v.Memo, &v.DebitMinor, &v.CreditMinor); e != nil {
				return normalizeError(e)
			}
			running += v.DebitMinor - v.CreditMinor
			v.RunningBalanceMinor = running
			result.TotalDebitMinor += v.DebitMinor
			result.TotalCreditMinor += v.CreditMinor
			result.Entries = append(result.Entries, v)
		}
		result.ClosingBalanceMinor = running
		return normalizeError(rows.Err())
	})
	return result, err
}

func (s *Store) CashActivity(ctx context.Context, scope tenancy.Scope, actor string, from, to time.Time) (int64, int64, []reporting.CashMovement, string, error) {
	opening := int64(0)
	closing := int64(0)
	currency := ""
	result := []reporting.CashMovement{}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "reports.financial.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		from, to, e = tx.reportingBounds(ctx, scope, from, to)
		if e != nil {
			return e
		}
		if e = tx.tx.QueryRow(ctx, `SELECT base_currency FROM legal_companies WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.CompanyID).Scan(&currency); e != nil {
			return normalizeError(e)
		}
		cashCTE := `WITH cash_accounts AS (SELECT DISTINCT gl_account_id account_id FROM bank_accounts WHERE tenant_id=$1 AND company_id=$2 AND active UNION SELECT DISTINCT account_id FROM posting_mappings WHERE tenant_id=$1 AND company_id=$2 AND status='ACTIVE' AND mapping_key LIKE 'PAYMENT_%' AND effective_from<$5) `
		if e = tx.tx.QueryRow(ctx, cashCTE+`SELECT COALESCE(sum(jl.debit_minor-jl.credit_minor) FILTER(WHERE j.occurred_at<$4),0),COALESCE(sum(jl.debit_minor-jl.credit_minor) FILTER(WHERE j.occurred_at<$5),0) FROM journal_lines jl JOIN journals j ON j.id=jl.journal_id JOIN cash_accounts c ON c.account_id=jl.account_id WHERE jl.tenant_id=$1 AND jl.company_id=$2 AND j.currency=$3`, scope.TenantID, scope.CompanyID, currency, from, to).Scan(&opening, &closing); e != nil {
			return normalizeError(e)
		}
		rows, e := tx.tx.Query(ctx, cashCTE+`SELECT j.id::text,j.occurred_at,CASE WHEN j.source_type='FINANCIAL_DOCUMENT' THEN COALESCE(fd.document_type,j.source_type) ELSE j.source_type END,j.source_id::text,string_agg(DISTINCT jl.memo,'; '),sum(jl.debit_minor-jl.credit_minor) amount FROM journal_lines jl JOIN journals j ON j.id=jl.journal_id JOIN cash_accounts c ON c.account_id=jl.account_id LEFT JOIN financial_documents fd ON fd.tenant_id=j.tenant_id AND fd.company_id=j.company_id AND fd.id=j.source_id WHERE jl.tenant_id=$1 AND jl.company_id=$2 AND j.currency=$3 AND j.occurred_at>=$4 AND j.occurred_at<$5 GROUP BY j.id,j.occurred_at,j.source_type,j.source_id,fd.document_type HAVING sum(jl.debit_minor-jl.credit_minor)<>0 ORDER BY j.occurred_at,j.id`, scope.TenantID, scope.CompanyID, currency, from, to)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var v reporting.CashMovement
			if e = rows.Scan(&v.JournalID, &v.OccurredAt, &v.SourceType, &v.SourceID, &v.Memo, &v.AmountMinor); e != nil {
				return normalizeError(e)
			}
			result = append(result, v)
		}
		return normalizeError(rows.Err())
	})
	return opening, closing, result, currency, err
}

func (t *transaction) reportingBounds(ctx context.Context, scope tenancy.Scope, from, to time.Time) (time.Time, time.Time, error) {
	var zoneName string
	if err := t.tx.QueryRow(ctx, `SELECT business_timezone FROM legal_companies WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.CompanyID).Scan(&zoneName); err != nil {
		return time.Time{}, time.Time{}, normalizeError(err)
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local := func(value time.Time) time.Time {
		if value.IsZero() {
			return value
		}
		year, month, date := value.Date()
		return time.Date(year, month, date, 0, 0, 0, 0, zone).UTC()
	}
	return local(from), local(to), nil
}

func (s *Store) SaveReportExport(ctx context.Context, scope tenancy.Scope, actor string, artifact reporting.ExportArtifact, idem, hash string) (reporting.ExportArtifact, error) {
	var result reporting.ExportArtifact
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "reports.financial.export")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "report.export.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			return tx.tx.QueryRow(ctx, `SELECT id::text,report_type,filename,media_type,content_base64,generated_at FROM report_exports WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, resultID).Scan(&result.ID, &result.ReportType, &result.Filename, &result.MediaType, &result.ContentBase64, &result.GeneratedAt)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO report_exports(id,tenant_id,company_id,report_type,filename,media_type,content_base64,generated_by,generated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, artifact.ID, scope.TenantID, scope.CompanyID, artifact.ReportType, artifact.Filename, artifact.MediaType, artifact.ContentBase64, actor, artifact.GeneratedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "report.exported", "report_export", artifact.ID, artifact.ID, artifact.GeneratedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "report.export.v1", idem, artifact.ID); e != nil {
			return e
		}
		result = artifact
		return nil
	})
	return result, err
}
