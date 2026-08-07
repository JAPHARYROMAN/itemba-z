package postgres

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

func (s *Store) ListFacilities(ctx context.Context, scope tenancy.Scope, actor string) (treasury.Page, error) {
	r := treasury.Page{Items: []treasury.Facility{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.treasury.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text FROM treasury_facilities WHERE tenant_id=$1 AND company_id=$2 ORDER BY reference`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return normalizeError(e)
			}
			f, e := tx.treasuryFacility(ctx, scope, id)
			if e != nil {
				return e
			}
			r.Items = append(r.Items, f)
		}
		return normalizeError(rows.Err())
	})
	return r, err
}

func (t *transaction) treasuryFacility(ctx context.Context, scope tenancy.Scope, id string) (treasury.Facility, error) {
	f := treasury.Facility{Scope: scope, Transactions: []treasury.Transaction{}}
	err := t.tx.QueryRow(ctx, `SELECT id::text,reference,lender,facility_type,status,currency,limit_minor,annual_interest_basis_points,start_date,maturity_date,bank_account_id,principal_account_id,interest_expense_account_id,accrued_interest_account_id,reason,created_by::text,created_at,COALESCE(approved_by::text,''),closed_at FROM treasury_facilities WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id).Scan(&f.ID, &f.Reference, &f.Lender, &f.Type, &f.Status, &f.Currency, &f.LimitMinor, &f.AnnualInterestBasisPoints, &f.StartDate, &f.MaturityDate, &f.BankAccountID, &f.PrincipalAccountID, &f.InterestExpenseAccountID, &f.AccruedInterestAccountID, &f.Reason, &f.CreatedBy, &f.CreatedAt, &f.ApprovedBy, &f.ClosedAt)
	if err != nil {
		return f, normalizeError(err)
	}
	rows, err := t.tx.Query(ctx, `SELECT id::text,facility_id::text,transaction_type,amount_minor,occurred_at,reason,journal_id::text,posted_by::text,posted_at FROM treasury_transactions WHERE tenant_id=$1 AND company_id=$2 AND facility_id=$3 ORDER BY occurred_at,id`, scope.TenantID, scope.CompanyID, id)
	if err != nil {
		return f, normalizeError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x treasury.Transaction
		if err = rows.Scan(&x.ID, &x.FacilityID, &x.Type, &x.AmountMinor, &x.OccurredAt, &x.Reason, &x.JournalID, &x.PostedBy, &x.PostedAt); err != nil {
			return f, normalizeError(err)
		}
		f.Transactions = append(f.Transactions, x)
		switch x.Type {
		case treasury.Drawdown:
			f.OutstandingPrincipalMinor += x.AmountMinor
		case treasury.PrincipalRepayment:
			f.OutstandingPrincipalMinor -= x.AmountMinor
		case treasury.InterestAccrual:
			f.AccruedInterestMinor += x.AmountMinor
		case treasury.InterestPayment:
			f.AccruedInterestMinor -= x.AmountMinor
		}
	}
	f.AvailableMinor = f.LimitMinor - f.OutstandingPrincipalMinor
	return f, normalizeError(rows.Err())
}

func (s *Store) CreateFacility(ctx context.Context, f treasury.Facility, idem, hash string) (treasury.Facility, error) {
	var r treasury.Facility
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, f.Scope, f.CreatedBy, "finance.treasury.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, f.Scope, "finance.treasury.facility.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.treasuryFacility(ctx, f.Scope, resultID)
			return e
		}
		var currency string
		if e = tx.tx.QueryRow(ctx, `SELECT base_currency FROM legal_companies WHERE tenant_id=$1 AND id=$2`, f.Scope.TenantID, f.Scope.CompanyID).Scan(&currency); e != nil {
			return normalizeError(e)
		}
		if currency != f.Currency {
			return treasury.ErrInvalidCommand
		}
		var governed int
		if e = tx.tx.QueryRow(ctx, `SELECT count(*) FROM (VALUES($3::text,'ASSET'),($4,'LIABILITY'),($5,'EXPENSE'),($6,'LIABILITY')) v(id,kind) JOIN gl_accounts g ON g.tenant_id=$1 AND g.company_id=$2 AND g.id=v.id AND g.status='ACTIVE' AND g.account_type=v.kind`, f.Scope.TenantID, f.Scope.CompanyID, f.BankAccountID, f.PrincipalAccountID, f.InterestExpenseAccountID, f.AccruedInterestAccountID).Scan(&governed); e != nil {
			return normalizeError(e)
		}
		if governed != 4 {
			return treasury.ErrInvalidCommand
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO treasury_facilities(id,tenant_id,company_id,reference,lender,facility_type,status,currency,limit_minor,annual_interest_basis_points,start_date,maturity_date,bank_account_id,principal_account_id,interest_expense_account_id,accrued_interest_account_id,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, f.ID, f.Scope.TenantID, f.Scope.CompanyID, f.Reference, f.Lender, f.Type, f.Currency, f.LimitMinor, f.AnnualInterestBasisPoints, f.StartDate, f.MaturityDate, f.BankAccountID, f.PrincipalAccountID, f.InterestExpenseAccountID, f.AccruedInterestAccountID, f.Reason, f.CreatedBy, f.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, f.Scope, f.CreatedBy, "treasury.facility_created", "treasury_facility", f.ID, f.ID, f.CreatedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, f.Scope, "finance.treasury.facility.create.v1", idem, f.ID); e != nil {
			return e
		}
		r = f
		return nil
	})
	return r, err
}

func (s *Store) TransitionFacility(ctx context.Context, scope tenancy.Scope, actor, id string, to treasury.Status, reason, idem, hash string, at time.Time) (treasury.Facility, error) {
	var r treasury.Facility
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		permission := "finance.treasury.manage"
		if to == treasury.Active || to == treasury.Rejected || to == treasury.Closed {
			permission = "finance.treasury.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, permission)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "finance.treasury.facility.transition." + string(to) + ".v1"
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.treasuryFacility(ctx, scope, resultID)
			return e
		}
		f, e := tx.treasuryFacility(ctx, scope, id)
		if e != nil {
			return e
		}
		valid := f.Status == treasury.Draft && to == treasury.Submitted || f.Status == treasury.Submitted && (to == treasury.Active || to == treasury.Rejected) || f.Status == treasury.Active && to == treasury.Closed && f.OutstandingPrincipalMinor == 0 && f.AccruedInterestMinor == 0
		if !valid {
			return treasury.ErrInvalidTransition
		}
		if (to == treasury.Active || to == treasury.Rejected) && f.CreatedBy == actor {
			return treasury.ErrSeparationOfDuties
		}
		_, e = tx.tx.Exec(ctx, `UPDATE treasury_facilities SET status=$1,approved_by=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $2 ELSE approved_by END,approved_at=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $3 ELSE approved_at END,closed_by=CASE WHEN $1='CLOSED' THEN $2 ELSE closed_by END,closed_at=CASE WHEN $1='CLOSED' THEN $3 ELSE closed_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO treasury_facility_transitions(id,tenant_id,company_id,facility_id,from_status,to_status,reason,actor_id,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, f.Status, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "treasury.facility_"+string(to), "treasury_facility", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		r, e = tx.treasuryFacility(ctx, scope, id)
		return e
	})
	return r, err
}

func (s *Store) PostTransaction(ctx context.Context, scope tenancy.Scope, actor, facilityID string, x treasury.Transaction, idem, hash string) (treasury.Facility, error) {
	var r treasury.Facility
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.treasury.transact")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "finance.treasury.transaction." + string(x.Type) + ".v1"
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			var fid string
			if e = tx.tx.QueryRow(ctx, `SELECT facility_id::text FROM treasury_transactions WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, resultID).Scan(&fid); e != nil {
				return normalizeError(e)
			}
			r, e = tx.treasuryFacility(ctx, scope, fid)
			return e
		}
		f, e := tx.treasuryFacility(ctx, scope, facilityID)
		if e != nil {
			return e
		}
		if f.Status != treasury.Active || x.OccurredAt.Before(f.StartDate) || !x.OccurredAt.Before(f.MaturityDate.AddDate(0, 0, 1)) {
			return treasury.ErrInvalidTransition
		}
		if e = tx.requireOpenPeriod(ctx, scope, x.OccurredAt); e != nil {
			return e
		}
		entries := []finance.JournalEntry{}
		switch x.Type {
		case treasury.Drawdown:
			if x.AmountMinor > f.AvailableMinor {
				return treasury.ErrLimitExceeded
			}
			entries = []finance.JournalEntry{{AccountID: f.BankAccountID, DebitMinor: x.AmountMinor, Memo: x.Reason}, {AccountID: f.PrincipalAccountID, CreditMinor: x.AmountMinor, Memo: x.Reason}}
		case treasury.PrincipalRepayment:
			if x.AmountMinor > f.OutstandingPrincipalMinor {
				return treasury.ErrLimitExceeded
			}
			entries = []finance.JournalEntry{{AccountID: f.PrincipalAccountID, DebitMinor: x.AmountMinor, Memo: x.Reason}, {AccountID: f.BankAccountID, CreditMinor: x.AmountMinor, Memo: x.Reason}}
		case treasury.InterestAccrual:
			entries = []finance.JournalEntry{{AccountID: f.InterestExpenseAccountID, DebitMinor: x.AmountMinor, Memo: x.Reason}, {AccountID: f.AccruedInterestAccountID, CreditMinor: x.AmountMinor, Memo: x.Reason}}
		case treasury.InterestPayment:
			if x.AmountMinor > f.AccruedInterestMinor {
				return treasury.ErrLimitExceeded
			}
			entries = []finance.JournalEntry{{AccountID: f.AccruedInterestAccountID, DebitMinor: x.AmountMinor, Memo: x.Reason}, {AccountID: f.BankAccountID, CreditMinor: x.AmountMinor, Memo: x.Reason}}
		}
		if e = tx.CreateJournal(ctx, finance.Journal{ID: x.JournalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "TREASURY_" + string(x.Type), SourceID: x.ID, Currency: f.Currency, OccurredAt: x.OccurredAt, Entries: entries}); e != nil {
			return e
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO treasury_transactions(id,tenant_id,company_id,facility_id,transaction_type,amount_minor,occurred_at,reason,journal_id,posted_by,posted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, x.ID, scope.TenantID, scope.CompanyID, facilityID, x.Type, x.AmountMinor, x.OccurredAt, x.Reason, x.JournalID, actor, x.PostedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "treasury.transaction_"+string(x.Type), "treasury_transaction", x.ID, facilityID, x.PostedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, x.ID); e != nil {
			return e
		}
		r, e = tx.treasuryFacility(ctx, scope, facilityID)
		return e
	})
	return r, err
}
