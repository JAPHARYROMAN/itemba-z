package postgres

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) ListBudgets(ctx context.Context, scope tenancy.Scope, actor string) (advancedfinance.Page[advancedfinance.Budget], error) {
	r := advancedfinance.Page[advancedfinance.Budget]{Items: []advancedfinance.Budget{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.budgets.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text FROM budgets WHERE tenant_id=$1 AND company_id=$2 ORDER BY fiscal_year DESC,created_at DESC`, scope.TenantID, scope.CompanyID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return normalizeError(e)
			}
			v, e := tx.budget(ctx, scope, id)
			if e != nil {
				return e
			}
			r.Items = append(r.Items, v)
		}
		return normalizeError(rows.Err())
	})
	return r, err
}
func (t *transaction) budget(ctx context.Context, scope tenancy.Scope, id string) (advancedfinance.Budget, error) {
	v := advancedfinance.Budget{Scope: scope, Lines: []advancedfinance.BudgetLine{}}
	err := t.tx.QueryRow(ctx, `SELECT id::text,name,fiscal_year,currency,status,reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM budgets WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id).Scan(&v.ID, &v.Name, &v.FiscalYear, &v.Currency, &v.Status, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy)
	if err != nil {
		return v, normalizeError(err)
	}
	rows, err := t.tx.Query(ctx, `SELECT account_id,month,amount_minor FROM budget_lines WHERE tenant_id=$1 AND company_id=$2 AND budget_id=$3 ORDER BY month,account_id`, scope.TenantID, scope.CompanyID, id)
	if err != nil {
		return v, normalizeError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var l advancedfinance.BudgetLine
		if err = rows.Scan(&l.AccountID, &l.Month, &l.AmountMinor); err != nil {
			return v, normalizeError(err)
		}
		v.Lines = append(v.Lines, l)
	}
	return v, normalizeError(rows.Err())
}
func (s *Store) CreateBudget(ctx context.Context, b advancedfinance.Budget, idem, hash string) (advancedfinance.Budget, error) {
	var r advancedfinance.Budget
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, b.Scope, b.CreatedBy, "finance.budgets.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, b.Scope, "finance.budget.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.budget(ctx, b.Scope, resultID)
			return e
		}
		var currency string
		if e = tx.tx.QueryRow(ctx, `SELECT base_currency FROM legal_companies WHERE tenant_id=$1 AND id=$2`, b.Scope.TenantID, b.Scope.CompanyID).Scan(&currency); e != nil {
			return normalizeError(e)
		}
		if b.Currency != currency {
			return advancedfinance.ErrInvalidCommand
		}
		accountIDs := make([]string, 0, len(b.Lines))
		for _, l := range b.Lines {
			accountIDs = append(accountIDs, l.AccountID)
		}
		var governed int
		if e = tx.tx.QueryRow(ctx, `SELECT count(DISTINCT id) FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=ANY($3::text[]) AND status='ACTIVE' AND account_type IN('REVENUE','EXPENSE')`, b.Scope.TenantID, b.Scope.CompanyID, accountIDs).Scan(&governed); e != nil {
			return normalizeError(e)
		}
		unique := map[string]bool{}
		for _, id := range accountIDs {
			unique[id] = true
		}
		if governed != len(unique) {
			return advancedfinance.ErrInvalidCommand
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO budgets(id,tenant_id,company_id,name,fiscal_year,currency,status,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8,$9)`, b.ID, b.Scope.TenantID, b.Scope.CompanyID, b.Name, b.FiscalYear, b.Currency, b.Reason, b.CreatedBy, b.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		for _, l := range b.Lines {
			if _, e = tx.tx.Exec(ctx, `INSERT INTO budget_lines(tenant_id,company_id,budget_id,account_id,month,amount_minor) VALUES($1,$2,$3,$4,$5,$6)`, b.Scope.TenantID, b.Scope.CompanyID, b.ID, l.AccountID, l.Month, l.AmountMinor); e != nil {
				return normalizeError(e)
			}
		}
		if e = tx.financialEvidence(ctx, b.Scope, b.CreatedBy, "budget.created", "budget", b.ID, b.ID, b.CreatedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, b.Scope, "finance.budget.create.v1", idem, b.ID); e != nil {
			return e
		}
		r = b
		return nil
	})
	return r, err
}
func (s *Store) TransitionBudget(ctx context.Context, scope tenancy.Scope, actor, id string, to advancedfinance.Status, reason, idem, hash string, at time.Time) (advancedfinance.Budget, error) {
	var r advancedfinance.Budget
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		permission := "finance.budgets.manage"
		if to == advancedfinance.Approved || to == advancedfinance.Rejected {
			permission = "finance.budgets.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, permission)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "finance.budget.transition." + string(to) + ".v1"
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.budget(ctx, scope, resultID)
			return e
		}
		b, e := tx.budget(ctx, scope, id)
		if e != nil {
			return e
		}
		valid := b.Status == advancedfinance.Draft && to == advancedfinance.Submitted || b.Status == advancedfinance.Submitted && (to == advancedfinance.Approved || to == advancedfinance.Rejected)
		if !valid {
			return advancedfinance.ErrInvalidTransition
		}
		if (to == advancedfinance.Approved || to == advancedfinance.Rejected) && b.CreatedBy == actor {
			return advancedfinance.ErrSeparationOfDuties
		}
		_, e = tx.tx.Exec(ctx, `UPDATE budgets SET status=$1,approved_by=CASE WHEN $1 IN('APPROVED','REJECTED') THEN $2 ELSE approved_by END,approved_at=CASE WHEN $1 IN('APPROVED','REJECTED') THEN $3 ELSE approved_at END WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO budget_transitions(id,tenant_id,company_id,budget_id,from_status,to_status,reason,actor_id,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, b.Status, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "budget."+string(to), "budget", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		r, e = tx.budget(ctx, scope, id)
		return e
	})
	return r, err
}
func (s *Store) BudgetActual(ctx context.Context, scope tenancy.Scope, actor, id string, at time.Time) (advancedfinance.BudgetActual, error) {
	r := advancedfinance.BudgetActual{BudgetID: id, Lines: []advancedfinance.BudgetActualLine{}, GeneratedAt: at}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.budgets.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		var status string
		if e = tx.tx.QueryRow(ctx, `SELECT currency,status FROM budgets WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, id).Scan(&r.Currency, &status); e != nil {
			return normalizeError(e)
		}
		if status != "APPROVED" {
			return advancedfinance.ErrInvalidTransition
		}
		rows, e := tx.tx.Query(ctx, `SELECT bl.account_id,a.code,a.name,bl.month,bl.amount_minor,COALESCE(sum(CASE WHEN j.id IS NULL THEN 0 WHEN a.account_type='REVENUE' THEN jl.credit_minor-jl.debit_minor ELSE jl.debit_minor-jl.credit_minor END),0) FROM budget_lines bl JOIN gl_accounts a ON a.tenant_id=bl.tenant_id AND a.company_id=bl.company_id AND a.id=bl.account_id JOIN legal_companies c ON c.tenant_id=bl.tenant_id AND c.id=bl.company_id LEFT JOIN journal_lines jl ON jl.tenant_id=bl.tenant_id AND jl.company_id=bl.company_id AND jl.account_id=bl.account_id LEFT JOIN journals j ON j.id=jl.journal_id AND j.currency=$4 AND date_trunc('month',timezone(c.business_timezone,j.occurred_at))::date=bl.month WHERE bl.tenant_id=$1 AND bl.company_id=$2 AND bl.budget_id=$3 GROUP BY bl.account_id,a.code,a.name,a.account_type,bl.month,bl.amount_minor ORDER BY bl.month,a.code`, scope.TenantID, scope.CompanyID, id, r.Currency)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var l advancedfinance.BudgetActualLine
			if e = rows.Scan(&l.AccountID, &l.Code, &l.Name, &l.Month, &l.BudgetMinor, &l.ActualMinor); e != nil {
				return normalizeError(e)
			}
			l.VarianceMinor = l.ActualMinor - l.BudgetMinor
			r.TotalBudgetMinor += l.BudgetMinor
			r.TotalActualMinor += l.ActualMinor
			r.Lines = append(r.Lines, l)
		}
		r.TotalVarianceMinor = r.TotalActualMinor - r.TotalBudgetMinor
		return normalizeError(rows.Err())
	})
	return r, err
}

func (s *Store) ListAssets(ctx context.Context, scope tenancy.Scope, actor string) (advancedfinance.Page[advancedfinance.Asset], error) {
	r := advancedfinance.Page[advancedfinance.Asset]{Items: []advancedfinance.Asset{}}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.assets.read")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text FROM fixed_assets WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 ORDER BY code`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID)
		if e != nil {
			return normalizeError(e)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return normalizeError(e)
			}
			a, e := tx.asset(ctx, scope, id)
			if e != nil {
				return e
			}
			r.Items = append(r.Items, a)
		}
		return normalizeError(rows.Err())
	})
	return r, err
}
func (t *transaction) asset(ctx context.Context, scope tenancy.Scope, id string) (advancedfinance.Asset, error) {
	a := advancedfinance.Asset{Scope: scope}
	err := t.tx.QueryRow(ctx, `SELECT id::text,code,name,category,status,currency,acquired_at,cost_minor,residual_minor,useful_life_months,accumulated_depreciation_minor,asset_account_id,accumulated_depreciation_account_id,depreciation_expense_account_id,capitalization_offset_account_id,disposal_gain_account_id,disposal_loss_account_id,reason,created_by::text,created_at,COALESCE(approved_by::text,''),disposed_at FROM fixed_assets WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, id).Scan(&a.ID, &a.Code, &a.Name, &a.Category, &a.Status, &a.Currency, &a.AcquiredAt, &a.CostMinor, &a.ResidualMinor, &a.UsefulLifeMonths, &a.AccumulatedDepreciationMinor, &a.AssetAccountID, &a.AccumulatedDepreciationAccountID, &a.DepreciationExpenseAccountID, &a.CapitalizationOffsetAccountID, &a.DisposalGainAccountID, &a.DisposalLossAccountID, &a.Reason, &a.CreatedBy, &a.CreatedAt, &a.ApprovedBy, &a.DisposedAt)
	a.NetBookValueMinor = a.CostMinor - a.AccumulatedDepreciationMinor
	return a, normalizeError(err)
}
func (s *Store) CreateAsset(ctx context.Context, a advancedfinance.Asset, idem, hash string) (advancedfinance.Asset, error) {
	var r advancedfinance.Asset
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, a.Scope, a.CreatedBy, "finance.assets.manage")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, a.Scope, "finance.asset.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.asset(ctx, a.Scope, resultID)
			return e
		}
		var currency string
		if e = tx.tx.QueryRow(ctx, `SELECT base_currency FROM legal_companies WHERE tenant_id=$1 AND id=$2`, a.Scope.TenantID, a.Scope.CompanyID).Scan(&currency); e != nil {
			return normalizeError(e)
		}
		if currency != a.Currency {
			return advancedfinance.ErrInvalidCommand
		}
		var governed int
		if e = tx.tx.QueryRow(ctx, `SELECT count(*) FROM (VALUES($3::text,'ASSET'),($4,'ASSET'),($5,'EXPENSE'),($6,'ANY'),($7,'REVENUE'),($8,'EXPENSE')) v(id,kind) JOIN gl_accounts g ON g.tenant_id=$1 AND g.company_id=$2 AND g.id=v.id AND g.status='ACTIVE' AND (v.kind='ANY' OR g.account_type=v.kind)`, a.Scope.TenantID, a.Scope.CompanyID, a.AssetAccountID, a.AccumulatedDepreciationAccountID, a.DepreciationExpenseAccountID, a.CapitalizationOffsetAccountID, a.DisposalGainAccountID, a.DisposalLossAccountID).Scan(&governed); e != nil {
			return normalizeError(e)
		}
		if governed != 6 {
			return advancedfinance.ErrInvalidCommand
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO fixed_assets(id,tenant_id,company_id,branch_id,warehouse_id,code,name,category,status,currency,acquired_at,cost_minor,residual_minor,useful_life_months,asset_account_id,accumulated_depreciation_account_id,depreciation_expense_account_id,capitalization_offset_account_id,disposal_gain_account_id,disposal_loss_account_id,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'DRAFT',$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`, a.ID, a.Scope.TenantID, a.Scope.CompanyID, a.Scope.BranchID, a.Scope.WarehouseID, a.Code, a.Name, a.Category, a.Currency, a.AcquiredAt, a.CostMinor, a.ResidualMinor, a.UsefulLifeMonths, a.AssetAccountID, a.AccumulatedDepreciationAccountID, a.DepreciationExpenseAccountID, a.CapitalizationOffsetAccountID, a.DisposalGainAccountID, a.DisposalLossAccountID, a.Reason, a.CreatedBy, a.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, a.Scope, a.CreatedBy, "asset.created", "fixed_asset", a.ID, a.ID, a.CreatedAt); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, a.Scope, "finance.asset.create.v1", idem, a.ID); e != nil {
			return e
		}
		r = a
		return nil
	})
	return r, err
}
func (s *Store) TransitionAsset(ctx context.Context, scope tenancy.Scope, actor, id string, to advancedfinance.Status, reason, journalID, idem, hash string, at time.Time) (advancedfinance.Asset, error) {
	var r advancedfinance.Asset
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		permission := "finance.assets.manage"
		if to == advancedfinance.Active || to == advancedfinance.Rejected {
			permission = "finance.assets.approve"
		}
		ok, e := tx.Authorize(ctx, scope, actor, permission)
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		op := "finance.asset.transition." + string(to) + ".v1"
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.asset(ctx, scope, resultID)
			return e
		}
		a, e := tx.asset(ctx, scope, id)
		if e != nil {
			return e
		}
		valid := a.Status == advancedfinance.Draft && to == advancedfinance.Submitted || a.Status == advancedfinance.Submitted && (to == advancedfinance.Active || to == advancedfinance.Rejected)
		if !valid {
			return advancedfinance.ErrInvalidTransition
		}
		if (to == advancedfinance.Active || to == advancedfinance.Rejected) && a.CreatedBy == actor {
			return advancedfinance.ErrSeparationOfDuties
		}
		if to == advancedfinance.Active {
			if e = tx.requireOpenPeriod(ctx, scope, a.AcquiredAt); e != nil {
				return e
			}
			j := finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FIXED_ASSET_CAPITALIZATION", SourceID: a.ID, Currency: a.Currency, OccurredAt: a.AcquiredAt, Entries: []finance.JournalEntry{{AccountID: a.AssetAccountID, DebitMinor: a.CostMinor, Memo: a.Name}, {AccountID: a.CapitalizationOffsetAccountID, CreditMinor: a.CostMinor, Memo: a.Name}}}
			if e = tx.CreateJournal(ctx, j); e != nil {
				return e
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE fixed_assets SET status=$1,approved_by=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $2 ELSE approved_by END,approved_at=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $3 ELSE approved_at END,capitalization_journal_id=CASE WHEN $1='ACTIVE' THEN $4 ELSE capitalization_journal_id END WHERE tenant_id=$5 AND company_id=$6 AND id=$7`, to, actor, at, nullUUID(journalID), scope.TenantID, scope.CompanyID, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO fixed_asset_transitions(id,tenant_id,company_id,asset_id,from_status,to_status,reason,actor_id,occurred_at) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, a.Status, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "asset."+string(to), "fixed_asset", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		r, e = tx.asset(ctx, scope, id)
		return e
	})
	return r, err
}
func (t *transaction) requireOpenPeriod(ctx context.Context, scope tenancy.Scope, at time.Time) error {
	var open bool
	if err := t.tx.QueryRow(ctx, `SELECT is_open FROM fiscal_periods WHERE tenant_id=$1 AND company_id=$2 AND starts_at<=$3 AND ends_at>$3`, scope.TenantID, scope.CompanyID, at).Scan(&open); err != nil {
		return normalizeError(err)
	}
	if !open {
		return advancedfinance.ErrInvalidTransition
	}
	return nil
}
func (s *Store) PostDepreciation(ctx context.Context, scope tenancy.Scope, actor, assetID, id, journalID string, period time.Time, idem, hash string, at time.Time) (advancedfinance.Depreciation, error) {
	var r advancedfinance.Depreciation
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.assets.depreciate")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "finance.asset.depreciate.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			return tx.tx.QueryRow(ctx, `SELECT id::text,asset_id::text,period,amount_minor,journal_id::text,posted_by::text,posted_at FROM fixed_asset_depreciation WHERE tenant_id=$1 AND company_id=$2 AND id=$3`, scope.TenantID, scope.CompanyID, resultID).Scan(&r.ID, &r.AssetID, &r.Period, &r.AmountMinor, &r.JournalID, &r.PostedBy, &r.PostedAt)
		}
		a, e := tx.asset(ctx, scope, assetID)
		if e != nil {
			return e
		}
		if a.Status != advancedfinance.Active || period.Before(time.Date(a.AcquiredAt.Year(), a.AcquiredAt.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			return advancedfinance.ErrInvalidTransition
		}
		if e = tx.requireOpenPeriod(ctx, scope, period); e != nil {
			return e
		}
		remaining := a.CostMinor - a.ResidualMinor - a.AccumulatedDepreciationMinor
		if remaining <= 0 {
			return advancedfinance.ErrAssetFullyDepreciated
		}
		amount := (a.CostMinor - a.ResidualMinor) / int64(a.UsefulLifeMonths)
		if amount < 1 {
			amount = 1
		}
		if amount > remaining {
			amount = remaining
		}
		j := finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FIXED_ASSET_DEPRECIATION", SourceID: id, Currency: a.Currency, OccurredAt: period, Entries: []finance.JournalEntry{{AccountID: a.DepreciationExpenseAccountID, DebitMinor: amount, Memo: a.Name}, {AccountID: a.AccumulatedDepreciationAccountID, CreditMinor: amount, Memo: a.Name}}}
		if e = tx.CreateJournal(ctx, j); e != nil {
			return e
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO fixed_asset_depreciation(id,tenant_id,company_id,asset_id,period,amount_minor,journal_id,posted_by,posted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, scope.TenantID, scope.CompanyID, assetID, period, amount, journalID, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `UPDATE fixed_assets SET accumulated_depreciation_minor=accumulated_depreciation_minor+$1 WHERE tenant_id=$2 AND company_id=$3 AND id=$4`, amount, scope.TenantID, scope.CompanyID, assetID)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "asset.depreciated", "fixed_asset", assetID, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "finance.asset.depreciate.v1", idem, id); e != nil {
			return e
		}
		r = advancedfinance.Depreciation{ID: id, AssetID: assetID, Period: period, AmountMinor: amount, JournalID: journalID, PostedBy: actor, PostedAt: at}
		return nil
	})
	return r, err
}
func (s *Store) DisposeAsset(ctx context.Context, scope tenancy.Scope, actor, assetID string, proceeds int64, proceedsAccount, reason, journalID, idem, hash string, at time.Time) (advancedfinance.Asset, error) {
	var r advancedfinance.Asset
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		ok, e := tx.Authorize(ctx, scope, actor, "finance.assets.dispose")
		if e != nil {
			return e
		}
		if !ok {
			return sales.ErrForbidden
		}
		acquired, resultID, e := tx.ClaimIdempotency(ctx, scope, "finance.asset.dispose.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acquired {
			r, e = tx.asset(ctx, scope, resultID)
			return e
		}
		a, e := tx.asset(ctx, scope, assetID)
		if e != nil {
			return e
		}
		if a.Status != advancedfinance.Active {
			return advancedfinance.ErrInvalidTransition
		}
		if e = tx.requireOpenPeriod(ctx, scope, at); e != nil {
			return e
		}
		var valid bool
		if e = tx.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gl_accounts WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND status='ACTIVE' AND account_type='ASSET')`, scope.TenantID, scope.CompanyID, proceedsAccount).Scan(&valid); e != nil {
			return normalizeError(e)
		}
		if !valid {
			return advancedfinance.ErrInvalidCommand
		}
		entries := []finance.JournalEntry{{AccountID: a.AccumulatedDepreciationAccountID, DebitMinor: a.AccumulatedDepreciationMinor, Memo: reason}, {AccountID: a.AssetAccountID, CreditMinor: a.CostMinor, Memo: reason}}
		if proceeds > 0 {
			entries = append(entries, finance.JournalEntry{AccountID: proceedsAccount, DebitMinor: proceeds, Memo: reason})
		}
		nbv := a.CostMinor - a.AccumulatedDepreciationMinor
		if proceeds > nbv {
			entries = append(entries, finance.JournalEntry{AccountID: a.DisposalGainAccountID, CreditMinor: proceeds - nbv, Memo: reason})
		} else if proceeds < nbv {
			entries = append(entries, finance.JournalEntry{AccountID: a.DisposalLossAccountID, DebitMinor: nbv - proceeds, Memo: reason})
		}
		j := finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FIXED_ASSET_DISPOSAL", SourceID: assetID, Currency: a.Currency, OccurredAt: at, Entries: entries}
		if e = tx.CreateJournal(ctx, j); e != nil {
			return e
		}
		_, e = tx.tx.Exec(ctx, `UPDATE fixed_assets SET status='DISPOSED',disposal_journal_id=$1,disposed_by=$2,disposed_at=$3 WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, journalID, actor, at, scope.TenantID, scope.CompanyID, assetID)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "asset.disposed", "fixed_asset", assetID, assetID, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, "finance.asset.dispose.v1", idem, assetID); e != nil {
			return e
		}
		r, e = tx.asset(ctx, scope, assetID)
		return e
	})
	return r, err
}
