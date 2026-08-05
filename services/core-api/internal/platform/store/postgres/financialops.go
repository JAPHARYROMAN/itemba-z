package postgres

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) CreateFinancialDocument(ctx context.Context, d financialops.Document, idem, hash string) (financialops.Document, error) {
	var result financialops.Document
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		permission := "finance.journals.manage"
		if d.Type == financialops.CashTransfer {
			permission = "finance.cash.transfer"
		}
		if d.Type == financialops.BankAdjustment {
			permission = "finance.bank.adjust"
		}
		ok, err := tx.Authorize(ctx, d.Scope, d.CreatedBy, permission)
		if err != nil {
			return err
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, d.Scope, "financial.document.create.v1", idem, hash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.financialDocument(ctx, d.Scope, resultID)
			return err
		}
		d.Number = "FIN-" + time.Now().UTC().Format("20060102") + "-" + d.ID[len(d.ID)-8:]
		if d.Type == financialops.Reversal {
			var originalStatus, originalCurrency string
			err = tx.tx.QueryRow(ctx, `SELECT status,currency FROM financial_documents WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5 FOR UPDATE`, d.Scope.TenantID, d.Scope.CompanyID, d.Scope.BranchID, d.Scope.WarehouseID, d.ReversesDocumentID).Scan(&originalStatus, &originalCurrency)
			if err != nil {
				return normalizeError(err)
			}
			if originalStatus != "POSTED" {
				return financialops.ErrInvalidTransition
			}
			d.Currency = originalCurrency
			var exists bool
			if err = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM financial_documents WHERE tenant_id=$1 AND company_id=$2 AND reverses_document_id=$3)`, d.Scope.TenantID, d.Scope.CompanyID, d.ReversesDocumentID).Scan(&exists); err != nil {
				return normalizeError(err)
			}
			if exists {
				return financialops.ErrAlreadyReversed
			}
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO financial_documents(id,tenant_id,company_id,branch_id,warehouse_id,number,document_type,status,currency,accounting_at,reason,from_account_id,to_account_id,reverses_document_id,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,'DRAFT',$8,$9,$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,NULLIF($13,'')::uuid,$14,$15)`, d.ID, d.Scope.TenantID, d.Scope.CompanyID, d.Scope.BranchID, d.Scope.WarehouseID, d.Number, d.Type, d.Currency, d.AccountingAt, d.Reason, d.FromAccountID, d.ToAccountID, d.ReversesDocumentID, d.CreatedBy, d.CreatedAt)
		if err != nil {
			return normalizeError(err)
		}
		if d.Type == financialops.Reversal {
			rows, e := tx.tx.Query(ctx, `SELECT jl.account_id,jl.credit_minor,jl.debit_minor,'Reversal: '||jl.memo FROM financial_documents fd JOIN journal_lines jl ON jl.journal_id=fd.journal_id WHERE fd.tenant_id=$1 AND fd.company_id=$2 AND fd.id=$3 ORDER BY jl.id`, d.Scope.TenantID, d.Scope.CompanyID, d.ReversesDocumentID)
			if e != nil {
				return normalizeError(e)
			}
			defer rows.Close()
			for rows.Next() {
				var l finance.JournalEntry
				if e = rows.Scan(&l.AccountID, &l.DebitMinor, &l.CreditMinor, &l.Memo); e != nil {
					return normalizeError(e)
				}
				d.Lines = append(d.Lines, l)
			}
			if len(d.Lines) < 2 {
				return financialops.ErrInvalidTransition
			}
		} else if d.Type == financialops.CashTransfer {
			d.Lines[0].AccountID = "__AMOUNT__"
		}
		for _, l := range d.Lines {
			if _, err = tx.tx.Exec(ctx, `INSERT INTO financial_document_lines(tenant_id,company_id,document_id,account_id,debit_minor,credit_minor,memo) VALUES($1,$2,$3,$4,$5,$6,$7)`, d.Scope.TenantID, d.Scope.CompanyID, d.ID, l.AccountID, l.DebitMinor, l.CreditMinor, l.Memo); err != nil {
				return normalizeError(err)
			}
		}
		if err = tx.financialEvidence(ctx, d.Scope, d.CreatedBy, "financial.document_created", "financial_document", d.ID, d.ID, d.CreatedAt); err != nil {
			return err
		}
		if err = tx.CompleteIdempotency(ctx, d.Scope, "financial.document.create.v1", idem, d.ID); err != nil {
			return err
		}
		result = d
		return nil
	})
	return result, err
}

func (s *Store) ListFinancialDocuments(ctx context.Context, scope tenancy.Scope, actor, cursor string, limit int) (financialops.Page, error) {
	result := financialops.Page{Items: []financialops.Document{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.journals.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text FROM financial_documents WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND ($5='' OR id::text<$5) ORDER BY id DESC LIMIT $6`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, cursor, limit+1)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return e
			}
			ids = append(ids, id)
		}
		for _, id := range ids {
			d, e := tx.financialDocument(ctx, scope, id)
			if e != nil {
				return e
			}
			result.Items = append(result.Items, d)
		}
		if len(result.Items) > limit {
			next := result.Items[limit-1].ID
			result.NextCursor = &next
			result.Items = result.Items[:limit]
		}
		return nil
	})
	return result, err
}
func (s *Store) FinancialDocument(ctx context.Context, scope tenancy.Scope, actor, id string) (financialops.Document, error) {
	var result financialops.Document
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.journals.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		result, e = tx.financialDocument(ctx, scope, id)
		return e
	})
	return result, err
}
func (t *transaction) financialDocument(ctx context.Context, scope tenancy.Scope, id string) (financialops.Document, error) {
	d := financialops.Document{Scope: scope}
	err := t.tx.QueryRow(ctx, `SELECT id::text,number,document_type,status,currency,accounting_at,reason,COALESCE(from_account_id::text,''),COALESCE(to_account_id::text,''),COALESCE(reverses_document_id::text,''),COALESCE(journal_id::text,''),created_by::text,created_at,COALESCE(submitted_by::text,''),COALESCE(posted_by::text,'') FROM financial_documents WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, id).Scan(&d.ID, &d.Number, &d.Type, &d.Status, &d.Currency, &d.AccountingAt, &d.Reason, &d.FromAccountID, &d.ToAccountID, &d.ReversesDocumentID, &d.JournalID, &d.CreatedBy, &d.CreatedAt, &d.SubmittedBy, &d.PostedBy)
	if err != nil {
		return d, normalizeError(err)
	}
	rows, err := t.tx.Query(ctx, `SELECT account_id,debit_minor,credit_minor,memo FROM financial_document_lines WHERE tenant_id=$1 AND company_id=$2 AND document_id=$3 ORDER BY id`, scope.TenantID, scope.CompanyID, id)
	if err != nil {
		return d, normalizeError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var l finance.JournalEntry
		if err = rows.Scan(&l.AccountID, &l.DebitMinor, &l.CreditMinor, &l.Memo); err != nil {
			return d, normalizeError(err)
		}
		d.Lines = append(d.Lines, l)
	}
	return d, normalizeError(rows.Err())
}

func (s *Store) TransitionFinancialDocument(ctx context.Context, scope tenancy.Scope, actor, id string, to financialops.Status, reason, journalID, idem, hash string, at time.Time) (financialops.Document, error) {
	var result financialops.Document
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		permission := "finance.journals.manage"
		if to == financialops.Posted || to == financialops.Rejected {
			permission = "finance.journals.post"
		}
		ok, e := tx.Authorize(ctx, scope, actor, permission)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		operation := "financial.document.transition." + string(to) + ".v1"
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, operation, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			result, e = tx.financialDocument(ctx, scope, resultID)
			return e
		}
		d, e := tx.financialDocument(ctx, scope, id)
		if e != nil {
			return e
		}
		valid := d.Status == financialops.Draft && to == financialops.Submitted || d.Status == financialops.Submitted && (to == financialops.Posted || to == financialops.Rejected)
		if !valid {
			return financialops.ErrInvalidTransition
		}
		if (to == financialops.Posted || to == financialops.Rejected) && d.CreatedBy == actor {
			return financialops.ErrSeparationOfDuties
		}
		if to == financialops.Posted {
			var open bool
			e = tx.tx.QueryRow(ctx, `SELECT is_open FROM fiscal_periods WHERE tenant_id=$1 AND company_id=$2 AND starts_at<=$3 AND ends_at>$3`, scope.TenantID, scope.CompanyID, d.AccountingAt).Scan(&open)
			if e != nil {
				return normalizeError(e)
			}
			if !open {
				return financialops.ErrPeriodClosed
			}
			entries, e := tx.postingEntries(ctx, d)
			if e != nil {
				return e
			}
			j := finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FINANCIAL_DOCUMENT", SourceID: d.ID, Currency: d.Currency, OccurredAt: d.AccountingAt, Entries: entries}
			if e = tx.CreateJournal(ctx, j); e != nil {
				return e
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE financial_documents SET status=$1,submitted_by=CASE WHEN $1='SUBMITTED' THEN $2 ELSE submitted_by END,posted_by=CASE WHEN $1 IN ('POSTED','REJECTED') THEN $2 ELSE posted_by END,journal_id=CASE WHEN $1='POSTED' THEN $3 ELSE journal_id END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, nullUUID(journalID), scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO financial_document_transitions(id,tenant_id,company_id,document_id,from_status,to_status,actor_id,reason,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, d.Status, to, actor, reason, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "financial.document_"+string(to), "financial_document", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, operation, idem, id); e != nil {
			return e
		}
		result, e = tx.financialDocument(ctx, scope, id)
		return e
	})
	return result, err
}
func nullUUID(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (t *transaction) financialEvidence(ctx context.Context, scope tenancy.Scope, actor, action, entityType, entityID, causation string, at time.Time) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO audit_events(id,tenant_id,company_id,actor_id,action,entity_type,entity_id,correlation_id,causation_id,data,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$6,$7,jsonb_build_object('action',$4::text),$8)`, scope.TenantID, scope.CompanyID, actor, action, entityType, entityID, causation, at)
	if err != nil {
		return normalizeError(err)
	}
	_, err = t.tx.Exec(ctx, `INSERT INTO outbox_events(id,tenant_id,company_id,aggregate_type,aggregate_id,event_type,version,correlation_id,causation_id,payload,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,1,$4,$6,jsonb_build_object('action',$5::text),$7)`, scope.TenantID, scope.CompanyID, entityType, entityID, action, causation, at)
	return normalizeError(err)
}
func (t *transaction) postingEntries(ctx context.Context, d financialops.Document) ([]finance.JournalEntry, error) {
	if d.Type == financialops.ManualJournal || d.Type == financialops.Reversal {
		return d.Lines, nil
	}
	var fromGL, fromCurrency string
	var active bool
	if err := t.tx.QueryRow(ctx, `SELECT gl_account_id,currency,active FROM bank_accounts WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`, d.Scope.TenantID, d.Scope.CompanyID, d.Scope.BranchID, d.Scope.WarehouseID, d.FromAccountID).Scan(&fromGL, &fromCurrency, &active); err != nil {
		return nil, normalizeError(err)
	}
	if !active || fromCurrency != d.Currency {
		return nil, financialops.ErrInvalidCommand
	}
	amount := d.Lines[0].DebitMinor
	if d.Type == financialops.CashTransfer {
		var toGL, toCurrency string
		var toActive bool
		if err := t.tx.QueryRow(ctx, `SELECT gl_account_id,currency,active FROM bank_accounts WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`, d.Scope.TenantID, d.Scope.CompanyID, d.Scope.BranchID, d.Scope.WarehouseID, d.ToAccountID).Scan(&toGL, &toCurrency, &toActive); err != nil {
			return nil, normalizeError(err)
		}
		if !toActive || toCurrency != d.Currency {
			return nil, financialops.ErrInvalidCommand
		}
		return []finance.JournalEntry{{AccountID: toGL, DebitMinor: amount, Memo: d.Reason}, {AccountID: fromGL, CreditMinor: amount, Memo: d.Reason}}, nil
	}
	offset := d.Lines[0]
	if offset.DebitMinor > 0 {
		return []finance.JournalEntry{{AccountID: fromGL, DebitMinor: offset.DebitMinor, Memo: d.Reason}, {AccountID: offset.AccountID, CreditMinor: offset.DebitMinor, Memo: d.Reason}}, nil
	}
	return []finance.JournalEntry{{AccountID: offset.AccountID, DebitMinor: offset.CreditMinor, Memo: d.Reason}, {AccountID: fromGL, CreditMinor: offset.CreditMinor, Memo: d.Reason}}, nil
}

func (s *Store) ListFiscalPeriods(ctx context.Context, scope tenancy.Scope, actor string) ([]financialops.FiscalPeriod, error) {
	var result []financialops.FiscalPeriod
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.periods.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,tenant_id::text,company_id::text,starts_at,ends_at,is_open FROM fiscal_periods WHERE tenant_id=$1 AND company_id=$2 ORDER BY starts_at DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var p financialops.FiscalPeriod
			if e = rows.Scan(&p.ID, &p.TenantID, &p.CompanyID, &p.StartsAt, &p.EndsAt, &p.Open); e != nil {
				return e
			}
			result = append(result, p)
		}
		return rows.Err()
	})
	return result, err
}
func (s *Store) ListFiscalPeriodActions(ctx context.Context, scope tenancy.Scope, actor string) (financialops.PeriodActionPage, error) {
	result := financialops.PeriodActionPage{Items: []financialops.PeriodActionRequest{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.periods.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,period_id::text,action,reason,requested_by::text,requested_at,COALESCE(approved_by::text,''),approved_at FROM fiscal_period_action_requests WHERE tenant_id=$1 AND company_id=$2 ORDER BY requested_at DESC,id DESC LIMIT 200`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var v financialops.PeriodActionRequest
			if e = rows.Scan(&v.ID, &v.PeriodID, &v.Action, &v.Reason, &v.RequestedBy, &v.RequestedAt, &v.ApprovedBy, &v.ApprovedAt); e != nil {
				return normalizeError(e)
			}
			result.Items = append(result.Items, v)
		}
		return normalizeError(rows.Err())
	})
	return result, err
}
func (s *Store) RequestFiscalPeriodAction(ctx context.Context, scope tenancy.Scope, actor, periodID string, action financialops.PeriodAction, reason, requestID, idem, hash string, at time.Time) (financialops.PeriodActionRequest, error) {
	var result financialops.PeriodActionRequest
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		perm := "finance.periods.close"
		if action == financialops.ReopenPeriod {
			perm = "finance.periods.reopen"
		}
		ok, e := tx.Authorize(ctx, scope, actor, perm)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "fiscal.period.request.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			return tx.tx.QueryRow(ctx, `SELECT id::text,period_id::text,action,reason,requested_by::text,requested_at,COALESCE(approved_by::text,''),approved_at FROM fiscal_period_action_requests WHERE id=$1`, resultID).Scan(&result.ID, &result.PeriodID, &result.Action, &result.Reason, &result.RequestedBy, &result.RequestedAt, &result.ApprovedBy, &result.ApprovedAt)
		}
		var open bool
		if e = tx.tx.QueryRow(ctx, `SELECT is_open FROM fiscal_periods WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, periodID).Scan(&open); e != nil {
			return normalizeError(e)
		}
		if (action == financialops.ClosePeriod && !open) || (action == financialops.ReopenPeriod && open) {
			return financialops.ErrInvalidTransition
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO fiscal_period_action_requests(id,tenant_id,company_id,period_id,action,reason,requested_by,requested_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, requestID, scope.TenantID, scope.CompanyID, periodID, action, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "fiscal_period.action_requested", "fiscal_period_action", requestID, requestID, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "fiscal.period.request.v1", idem, requestID); e != nil {
			return e
		}
		result = financialops.PeriodActionRequest{ID: requestID, PeriodID: periodID, Action: action, Reason: reason, RequestedBy: actor, RequestedAt: at}
		return nil
	})
	return result, err
}
func (s *Store) ApproveFiscalPeriodAction(ctx context.Context, scope tenancy.Scope, actor, requestID, reason, idem, hash string, at time.Time) (financialops.PeriodActionRequest, error) {
	var result financialops.PeriodActionRequest
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		if e := tx.ensureScope(ctx, scope); e != nil {
			return e
		}
		var periodID, requestedBy string
		var action financialops.PeriodAction
		var approvedAt *time.Time
		e := tx.tx.QueryRow(ctx, `SELECT period_id::text,action,requested_by::text,approved_at FROM fiscal_period_action_requests WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, requestID).Scan(&periodID, &action, &requestedBy, &approvedAt)
		if e != nil {
			return normalizeError(e)
		}
		perm := "finance.periods.close"
		if action == financialops.ReopenPeriod {
			perm = "finance.periods.reopen"
		}
		ok, e := tx.Authorize(ctx, scope, actor, perm)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		if actor == requestedBy {
			return financialops.ErrSeparationOfDuties
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "fiscal.period.approve.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			return tx.tx.QueryRow(ctx, `SELECT id::text,period_id::text,action,reason,requested_by::text,requested_at,COALESCE(approved_by::text,''),approved_at FROM fiscal_period_action_requests WHERE id=$1`, resultID).Scan(&result.ID, &result.PeriodID, &result.Action, &result.Reason, &result.RequestedBy, &result.RequestedAt, &result.ApprovedBy, &result.ApprovedAt)
		}
		if approvedAt != nil {
			return financialops.ErrInvalidTransition
		}
		if action == financialops.ClosePeriod {
			var blocked bool
			e = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bank_statements s LEFT JOIN bank_statement_reconciliations r ON r.statement_id=s.id WHERE s.tenant_id=$1 AND s.company_id=$2 AND s.period_end>(SELECT starts_at FROM fiscal_periods WHERE id=$3) AND s.period_end<=(SELECT ends_at FROM fiscal_periods WHERE id=$3) AND r.id IS NULL) OR EXISTS(SELECT 1 FROM financial_documents d JOIN fiscal_periods p ON p.id=$3 WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.accounting_at>=p.starts_at AND d.accounting_at<p.ends_at AND d.status NOT IN ('POSTED','REJECTED'))`, scope.TenantID, scope.CompanyID, periodID).Scan(&blocked)
			if e != nil {
				return normalizeError(e)
			}
			if blocked {
				return financialops.ErrPeriodCloseBlocked
			}
		}
		open := action == financialops.ReopenPeriod
		tag, e := tx.tx.Exec(ctx, `UPDATE fiscal_periods SET is_open=$1 WHERE tenant_id=$2 AND company_id=$3 AND id=$4`, open, scope.TenantID, scope.CompanyID, periodID)
		if e != nil {
			return normalizeError(e)
		}
		if tag.RowsAffected() != 1 {
			return sales.ErrNotFound
		}
		_, e = tx.tx.Exec(ctx, `UPDATE fiscal_period_action_requests SET approved_by=$1,approved_at=$2,reason=reason||E'\nApproval: '||$3 WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, actor, at, reason, scope.TenantID, scope.CompanyID, requestID)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "fiscal_period.action_approved", "fiscal_period_action", requestID, requestID, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "fiscal.period.approve.v1", idem, requestID); e != nil {
			return e
		}
		return tx.tx.QueryRow(ctx, `SELECT id::text,period_id::text,action,reason,requested_by::text,requested_at,approved_by::text,approved_at FROM fiscal_period_action_requests WHERE id=$1`, requestID).Scan(&result.ID, &result.PeriodID, &result.Action, &result.Reason, &result.RequestedBy, &result.RequestedAt, &result.ApprovedBy, &result.ApprovedAt)
	})
	return result, err
}
