package postgres

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) ListIntercompany(ctx context.Context, scope tenancy.Scope, actor string) (groupfinance.Page, error) {
	r := groupfinance.Page{Items: []groupfinance.Transaction{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.intercompany.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text FROM intercompany_transactions WHERE tenant_id=$1 AND (source_company_id=$2 OR counterparty_company_id=$2) ORDER BY created_at DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return normalizeError(e)
			}
			v, e := tx.intercompany(ctx, scope, id)
			if e != nil {
				return e
			}
			r.Items = append(r.Items, v)
		}
		return normalizeError(rows.Err())
	})
	return r, err
}

func (t *transaction) intercompany(ctx context.Context, view tenancy.Scope, id string) (groupfinance.Transaction, error) {
	v := groupfinance.Transaction{}
	var sourceCompany string
	err := t.tx.QueryRow(ctx, `SELECT x.id::text,x.tenant_id::text,x.source_company_id::text,x.source_branch_id::text,x.source_warehouse_id::text,x.counterparty_company_id::text,c.name,x.reference,x.transaction_type,x.status,x.currency,x.amount_minor,x.occurred_at,x.source_debit_account_id,x.source_credit_account_id,x.counterparty_debit_account_id,x.counterparty_credit_account_id,x.reason,x.created_by::text,x.created_at,COALESCE(x.source_approved_by::text,''),COALESCE(x.posted_by::text,''),COALESCE(x.source_journal_id::text,''),COALESCE(x.counterparty_journal_id::text,'') FROM intercompany_transactions x JOIN legal_companies c ON c.tenant_id=x.tenant_id AND c.id=x.counterparty_company_id WHERE x.tenant_id=$1 AND x.id=$2 AND (x.source_company_id=$3 OR x.counterparty_company_id=$3)`, view.TenantID, id, view.CompanyID).Scan(&v.ID, &v.Scope.TenantID, &sourceCompany, &v.Scope.BranchID, &v.Scope.WarehouseID, &v.CounterpartyCompanyID, &v.CounterpartyCompanyName, &v.Reference, &v.Type, &v.Status, &v.Currency, &v.AmountMinor, &v.OccurredAt, &v.SourceDebitAccountID, &v.SourceCreditAccountID, &v.CounterpartyDebitAccountID, &v.CounterpartyCreditAccountID, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.SourceApprovedBy, &v.PostedBy, &v.SourceJournalID, &v.CounterpartyJournalID)
	v.Scope.CompanyID = sourceCompany
	return v, normalizeError(err)
}

func (s *Store) CreateIntercompany(ctx context.Context, v groupfinance.Transaction, idem, hash string) (groupfinance.Transaction, error) {
	var r groupfinance.Transaction
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, v.Scope, v.CreatedBy, "finance.intercompany.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, v.Scope, "finance.intercompany.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.intercompany(ctx, v.Scope, resultID)
			return e
		}
		var currencies int
		if e = tx.tx.QueryRow(ctx, `SELECT count(DISTINCT base_currency) FROM legal_companies WHERE tenant_id=$1 AND id IN($2,$3)`, v.Scope.TenantID, v.Scope.CompanyID, v.CounterpartyCompanyID).Scan(&currencies); e != nil {
			return normalizeError(e)
		}
		var companyCount int
		if e = tx.tx.QueryRow(ctx, `SELECT count(*) FROM legal_companies WHERE tenant_id=$1 AND id IN($2,$3) AND base_currency=$4`, v.Scope.TenantID, v.Scope.CompanyID, v.CounterpartyCompanyID, v.Currency).Scan(&companyCount); e != nil {
			return normalizeError(e)
		}
		if currencies != 1 || companyCount != 2 {
			return groupfinance.ErrInvalidCommand
		}
		expected := []string{"ASSET", "ASSET", "ASSET", "LIABILITY"}
		if v.Type == groupfinance.CostAllocation {
			expected = []string{"ASSET", "REVENUE", "EXPENSE", "LIABILITY"}
		}
		var governed int
		if e = tx.tx.QueryRow(ctx, `SELECT count(*) FROM (VALUES($2::uuid,$3::text,$7::text),($2,$4,$8),($5::uuid,$6,$9),($5,$10,$11)) v(company_id,account_id,kind) JOIN gl_accounts g ON g.tenant_id=$1 AND g.company_id=v.company_id AND g.id=v.account_id AND g.status='ACTIVE' AND g.account_type=v.kind`, v.Scope.TenantID, v.Scope.CompanyID, v.SourceDebitAccountID, v.SourceCreditAccountID, v.CounterpartyCompanyID, v.CounterpartyDebitAccountID, expected[0], expected[1], expected[2], v.CounterpartyCreditAccountID, expected[3]).Scan(&governed); e != nil {
			return normalizeError(e)
		}
		if governed != 4 {
			return groupfinance.ErrInvalidCommand
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO intercompany_transactions(id,tenant_id,source_company_id,source_branch_id,source_warehouse_id,counterparty_company_id,reference,transaction_type,status,currency,amount_minor,occurred_at,source_debit_account_id,source_credit_account_id,counterparty_debit_account_id,counterparty_credit_account_id,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'DRAFT',$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, v.CounterpartyCompanyID, v.Reference, v.Type, v.Currency, v.AmountMinor, v.OccurredAt, v.SourceDebitAccountID, v.SourceCreditAccountID, v.CounterpartyDebitAccountID, v.CounterpartyCreditAccountID, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "intercompany.created", "intercompany_transaction", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, v.Scope, "finance.intercompany.create.v1", idem, v.ID); e != nil {
			return e
		}
		r = v
		return nil
	})
	return r, err
}

func (s *Store) TransitionIntercompany(ctx context.Context, view tenancy.Scope, actor, id string, to groupfinance.Status, reason, sourceJournal, counterJournal, idem, hash string, at time.Time) (groupfinance.Transaction, error) {
	var r groupfinance.Transaction
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		permission := "finance.intercompany.manage"
		if to != groupfinance.Submitted {
			permission = "finance.intercompany.approve"
		}
		ok, e := tx.Authorize(ctx, view, actor, permission)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "finance.intercompany.transition." + string(to) + ".v1"
		acquired, resultID, e := tx.ClaimIdempotency(ctx, view, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.intercompany(ctx, view, resultID)
			return e
		}
		v, e := tx.intercompany(ctx, view, id)
		if e != nil {
			return e
		}
		source := view.CompanyID == v.Scope.CompanyID
		counter := view.CompanyID == v.CounterpartyCompanyID
		valid := v.Status == groupfinance.Draft && to == groupfinance.Submitted && source || v.Status == groupfinance.Submitted && (to == groupfinance.SourceApproved || to == groupfinance.Rejected) && source || v.Status == groupfinance.SourceApproved && (to == groupfinance.Posted || to == groupfinance.Rejected) && counter
		if !valid {
			return groupfinance.ErrInvalidTransition
		}
		if (to == groupfinance.SourceApproved || to == groupfinance.Rejected && v.Status == groupfinance.Submitted) && actor == v.CreatedBy {
			return groupfinance.ErrSeparationOfDuties
		}
		if (to == groupfinance.Posted || to == groupfinance.Rejected && v.Status == groupfinance.SourceApproved) && (actor == v.CreatedBy || actor == v.SourceApprovedBy) {
			return groupfinance.ErrSeparationOfDuties
		}
		if to == groupfinance.Posted {
			if e = tx.requireGroupOpenPeriods(ctx, v); e != nil {
				return e
			}
			sourceEntries := []finance.JournalEntry{{AccountID: v.SourceDebitAccountID, DebitMinor: v.AmountMinor, Memo: v.Reason}, {AccountID: v.SourceCreditAccountID, CreditMinor: v.AmountMinor, Memo: v.Reason}}
			counterEntries := []finance.JournalEntry{{AccountID: v.CounterpartyDebitAccountID, DebitMinor: v.AmountMinor, Memo: v.Reason}, {AccountID: v.CounterpartyCreditAccountID, CreditMinor: v.AmountMinor, Memo: v.Reason}}
			if e = tx.CreateJournal(ctx, finance.Journal{ID: sourceJournal, TenantID: v.Scope.TenantID, CompanyID: v.Scope.CompanyID, SourceType: "INTERCOMPANY_" + string(v.Type), SourceID: v.ID, Currency: v.Currency, OccurredAt: v.OccurredAt, Entries: sourceEntries}); e != nil {
				return e
			}
			if e = tx.CreateJournal(ctx, finance.Journal{ID: counterJournal, TenantID: v.Scope.TenantID, CompanyID: v.CounterpartyCompanyID, SourceType: "INTERCOMPANY_" + string(v.Type), SourceID: v.ID, Currency: v.Currency, OccurredAt: v.OccurredAt, Entries: counterEntries}); e != nil {
				return e
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE intercompany_transactions SET status=$1,source_approved_by=CASE WHEN $1='SOURCE_APPROVED' THEN $2 ELSE source_approved_by END,source_approved_at=CASE WHEN $1='SOURCE_APPROVED' THEN $3 ELSE source_approved_at END,posted_by=CASE WHEN $1='POSTED' THEN $2 ELSE posted_by END,posted_at=CASE WHEN $1='POSTED' THEN $3 ELSE posted_at END,source_journal_id=CASE WHEN $1='POSTED' THEN $4 ELSE source_journal_id END,counterparty_journal_id=CASE WHEN $1='POSTED' THEN $5 ELSE counterparty_journal_id END WHERE tenant_id=$6 AND source_company_id=$7 AND id=$8`, to, actor, at, nullUUID(sourceJournal), nullUUID(counterJournal), v.Scope.TenantID, v.Scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO intercompany_transitions(id,tenant_id,source_company_id,transaction_id,acting_company_id,from_status,to_status,reason,actor_id,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.Scope.TenantID, v.Scope.CompanyID, id, view.CompanyID, v.Status, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, view, actor, "intercompany."+string(to), "intercompany_transaction", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, view, op, idem, id); e != nil {
			return e
		}
		r, e = tx.intercompany(ctx, view, id)
		return e
	})
	return r, err
}

func (t *transaction) requireGroupOpenPeriods(ctx context.Context, v groupfinance.Transaction) error {
	var count int
	if err := t.tx.QueryRow(ctx, `SELECT count(*) FROM fiscal_periods WHERE tenant_id=$1 AND company_id IN($2,$3) AND starts_at<=$4 AND ends_at>$4 AND is_open`, v.Scope.TenantID, v.Scope.CompanyID, v.CounterpartyCompanyID, v.OccurredAt).Scan(&count); err != nil {
		return normalizeError(err)
	}
	if count != 2 {
		return groupfinance.ErrInvalidTransition
	}
	return nil
}

func (s *Store) Consolidation(ctx context.Context, scope tenancy.Scope, actor string, asOf time.Time) (groupfinance.Consolidation, error) {
	r := groupfinance.Consolidation{AsOf: asOf, Companies: []groupfinance.CompanySummary{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.consolidation.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		var currencyCount int
		if e = tx.tx.QueryRow(ctx, `SELECT min(base_currency),count(DISTINCT base_currency) FROM legal_companies WHERE tenant_id=$1`, scope.TenantID).Scan(&r.Currency, &currencyCount); e != nil {
			return normalizeError(e)
		}
		if currencyCount != 1 {
			return groupfinance.ErrInvalidCommand
		}
		rows, e := tx.tx.Query(ctx, `SELECT c.id::text,c.name,COALESCE(sum(CASE WHEN a.account_type='ASSET' THEN jl.debit_minor-jl.credit_minor ELSE 0 END),0),COALESCE(sum(CASE WHEN a.account_type='LIABILITY' THEN jl.credit_minor-jl.debit_minor ELSE 0 END),0),COALESCE(sum(CASE WHEN a.account_type='EQUITY' THEN jl.credit_minor-jl.debit_minor ELSE 0 END),0),COALESCE(sum(CASE WHEN a.account_type='REVENUE' THEN jl.credit_minor-jl.debit_minor ELSE 0 END),0),COALESCE(sum(CASE WHEN a.account_type='EXPENSE' THEN jl.debit_minor-jl.credit_minor ELSE 0 END),0) FROM legal_companies c LEFT JOIN journals j ON j.tenant_id=c.tenant_id AND j.company_id=c.id AND j.occurred_at<=$2 LEFT JOIN journal_lines jl ON jl.journal_id=j.id LEFT JOIN gl_accounts a ON a.tenant_id=jl.tenant_id AND a.company_id=jl.company_id AND a.id=jl.account_id WHERE c.tenant_id=$1 GROUP BY c.id,c.name ORDER BY c.name`, scope.TenantID, asOf)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var v groupfinance.CompanySummary
			if e = rows.Scan(&v.CompanyID, &v.CompanyName, &v.AssetsMinor, &v.LiabilitiesMinor, &v.EquityMinor, &v.RevenueMinor, &v.ExpenseMinor); e != nil {
				return normalizeError(e)
			}
			r.Companies = append(r.Companies, v)
			r.AssetsBeforeMinor += v.AssetsMinor
			r.LiabilitiesBeforeMinor += v.LiabilitiesMinor
			r.EquityMinor += v.EquityMinor
			r.RevenueBeforeMinor += v.RevenueMinor
			r.ExpenseBeforeMinor += v.ExpenseMinor
		}
		if e = rows.Err(); e != nil {
			return normalizeError(e)
		}
		if e = tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0),COALESCE(sum(CASE WHEN transaction_type='COST_ALLOCATION' THEN amount_minor ELSE 0 END),0) FROM intercompany_transactions WHERE tenant_id=$1 AND status='POSTED' AND occurred_at<=$2`, scope.TenantID, asOf).Scan(&r.IntercompanyBalanceEliminationMinor, &r.IntercompanyActivityEliminationMinor); e != nil {
			return normalizeError(e)
		}
		r.AssetsMinor = r.AssetsBeforeMinor - r.IntercompanyBalanceEliminationMinor
		r.LiabilitiesMinor = r.LiabilitiesBeforeMinor - r.IntercompanyBalanceEliminationMinor
		r.RevenueMinor = r.RevenueBeforeMinor - r.IntercompanyActivityEliminationMinor
		r.ExpenseMinor = r.ExpenseBeforeMinor - r.IntercompanyActivityEliminationMinor
		r.NetProfitMinor = r.RevenueMinor - r.ExpenseMinor
		r.Balanced = r.AssetsMinor == r.LiabilitiesMinor+r.EquityMinor+r.NetProfitMinor
		return nil
	})
	return r, err
}
