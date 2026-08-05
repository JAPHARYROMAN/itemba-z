package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) ListBudgets(_ context.Context, scope tenancy.Scope, actor string) (advancedfinance.Page[advancedfinance.Budget], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := advancedfinance.Page[advancedfinance.Budget]{Items: []advancedfinance.Budget{}}
	if !s.state.permissions[permissionKey(scope, actor, "finance.budgets.read")] {
		return r, sales.ErrForbidden
	}
	for _, v := range s.state.budgets {
		if v.Scope.TenantID == scope.TenantID && v.Scope.CompanyID == scope.CompanyID {
			v.Lines = append([]advancedfinance.BudgetLine(nil), v.Lines...)
			r.Items = append(r.Items, v)
		}
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].FiscalYear > r.Items[j].FiscalYear })
	return r, nil
}
func (s *Store) CreateBudget(_ context.Context, b advancedfinance.Budget, idem, hash string) (advancedfinance.Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(b.Scope, b.CreatedBy, "finance.budgets.manage")] {
		return advancedfinance.Budget{}, sales.ErrForbidden
	}
	key := idempotencyKey(b.Scope, "finance.budget.create.v1", idem)
	if v, ok := s.state.idempotencies[key]; ok {
		if v.RequestHash != hash {
			return advancedfinance.Budget{}, sales.ErrIdempotencyConflict
		}
		return s.state.budgets[companyEntityKey(b.Scope.TenantID, b.Scope.CompanyID, v.ResultID)], nil
	}
	s.state.idempotencies[key] = idempotency{RequestHash: hash, ResultID: b.ID}
	s.state.budgets[companyEntityKey(b.Scope.TenantID, b.Scope.CompanyID, b.ID)] = b
	return b, nil
}
func (s *Store) TransitionBudget(_ context.Context, scope tenancy.Scope, actor, id string, to advancedfinance.Status, _, idem, hash string, at time.Time) (advancedfinance.Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "finance.budgets.manage"
	if to == advancedfinance.Approved || to == advancedfinance.Rejected {
		permission = "finance.budgets.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, permission)] {
		return advancedfinance.Budget{}, sales.ErrForbidden
	}
	key := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	b, ok := s.state.budgets[key]
	if !ok {
		return b, sales.ErrNotFound
	}
	idemKey := idempotencyKey(scope, "finance.budget.transition."+string(to)+".v1", idem)
	if v, ok := s.state.idempotencies[idemKey]; ok {
		if v.RequestHash != hash {
			return b, sales.ErrIdempotencyConflict
		}
		return s.state.budgets[key], nil
	}
	valid := b.Status == advancedfinance.Draft && to == advancedfinance.Submitted || b.Status == advancedfinance.Submitted && (to == advancedfinance.Approved || to == advancedfinance.Rejected)
	if !valid {
		return b, advancedfinance.ErrInvalidTransition
	}
	if (to == advancedfinance.Approved || to == advancedfinance.Rejected) && b.CreatedBy == actor {
		return b, advancedfinance.ErrSeparationOfDuties
	}
	b.Status = to
	if to == advancedfinance.Approved || to == advancedfinance.Rejected {
		b.ApprovedBy = actor
	}
	s.state.budgets[key] = b
	s.state.idempotencies[idemKey] = idempotency{RequestHash: hash, ResultID: id}
	_ = at
	return b, nil
}
func (s *Store) BudgetActual(_ context.Context, scope tenancy.Scope, actor, id string, at time.Time) (advancedfinance.BudgetActual, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := advancedfinance.BudgetActual{BudgetID: id, Lines: []advancedfinance.BudgetActualLine{}, GeneratedAt: at}
	if !s.state.permissions[permissionKey(scope, actor, "finance.budgets.read")] {
		return r, sales.ErrForbidden
	}
	b, ok := s.state.budgets[companyEntityKey(scope.TenantID, scope.CompanyID, id)]
	if !ok {
		return r, sales.ErrNotFound
	}
	if b.Status != advancedfinance.Approved {
		return r, advancedfinance.ErrInvalidTransition
	}
	r.Currency = b.Currency
	for _, bl := range b.Lines {
		l := advancedfinance.BudgetActualLine{AccountID: bl.AccountID, Month: bl.Month, BudgetMinor: bl.AmountMinor}
		if a, ok := s.state.glAccounts[companyEntityKey(scope.TenantID, scope.CompanyID, bl.AccountID)]; ok {
			l.Code, l.Name = a.Code, a.Name
			for _, j := range s.state.journals {
				if j.TenantID != scope.TenantID || j.CompanyID != scope.CompanyID || j.Currency != b.Currency || j.OccurredAt.Year() != bl.Month.Year() || j.OccurredAt.Month() != bl.Month.Month() {
					continue
				}
				for _, e := range j.Entries {
					if e.AccountID == bl.AccountID {
						if a.Type == "REVENUE" {
							l.ActualMinor += e.CreditMinor - e.DebitMinor
						} else {
							l.ActualMinor += e.DebitMinor - e.CreditMinor
						}
					}
				}
			}
		}
		l.VarianceMinor = l.ActualMinor - l.BudgetMinor
		r.TotalBudgetMinor += l.BudgetMinor
		r.TotalActualMinor += l.ActualMinor
		r.Lines = append(r.Lines, l)
	}
	r.TotalVarianceMinor = r.TotalActualMinor - r.TotalBudgetMinor
	return r, nil
}

func (s *Store) ListAssets(_ context.Context, scope tenancy.Scope, actor string) (advancedfinance.Page[advancedfinance.Asset], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := advancedfinance.Page[advancedfinance.Asset]{Items: []advancedfinance.Asset{}}
	if !s.state.permissions[permissionKey(scope, actor, "finance.assets.read")] {
		return r, sales.ErrForbidden
	}
	for _, a := range s.state.assets {
		if a.Scope == scope {
			r.Items = append(r.Items, a)
		}
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].Code < r.Items[j].Code })
	return r, nil
}
func (s *Store) CreateAsset(_ context.Context, a advancedfinance.Asset, idem, hash string) (advancedfinance.Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(a.Scope, a.CreatedBy, "finance.assets.manage")] {
		return advancedfinance.Asset{}, sales.ErrForbidden
	}
	k := idempotencyKey(a.Scope, "finance.asset.create.v1", idem)
	if v, ok := s.state.idempotencies[k]; ok {
		if v.RequestHash != hash {
			return a, sales.ErrIdempotencyConflict
		}
		return s.state.assets[companyEntityKey(a.Scope.TenantID, a.Scope.CompanyID, v.ResultID)], nil
	}
	s.state.assets[companyEntityKey(a.Scope.TenantID, a.Scope.CompanyID, a.ID)] = a
	s.state.idempotencies[k] = idempotency{RequestHash: hash, ResultID: a.ID}
	return a, nil
}
func (s *Store) TransitionAsset(_ context.Context, scope tenancy.Scope, actor, id string, to advancedfinance.Status, _, journalID, idem, hash string, at time.Time) (advancedfinance.Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "finance.assets.manage"
	if to == advancedfinance.Active || to == advancedfinance.Rejected {
		permission = "finance.assets.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, permission)] {
		return advancedfinance.Asset{}, sales.ErrForbidden
	}
	key := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	a, ok := s.state.assets[key]
	if !ok {
		return a, sales.ErrNotFound
	}
	ik := idempotencyKey(scope, "finance.asset.transition."+string(to)+".v1", idem)
	if v, ok := s.state.idempotencies[ik]; ok {
		if v.RequestHash != hash {
			return a, sales.ErrIdempotencyConflict
		}
		return s.state.assets[key], nil
	}
	valid := a.Status == advancedfinance.Draft && to == advancedfinance.Submitted || a.Status == advancedfinance.Submitted && (to == advancedfinance.Active || to == advancedfinance.Rejected)
	if !valid {
		return a, advancedfinance.ErrInvalidTransition
	}
	if (to == advancedfinance.Active || to == advancedfinance.Rejected) && a.CreatedBy == actor {
		return a, advancedfinance.ErrSeparationOfDuties
	}
	if to == advancedfinance.Active {
		s.state.journals = append(s.state.journals, finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FIXED_ASSET_CAPITALIZATION", SourceID: id, Currency: a.Currency, OccurredAt: a.AcquiredAt, Entries: []finance.JournalEntry{{AccountID: a.AssetAccountID, DebitMinor: a.CostMinor}, {AccountID: a.CapitalizationOffsetAccountID, CreditMinor: a.CostMinor}}})
	}
	a.Status = to
	a.ApprovedBy = actor
	s.state.assets[key] = a
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	_ = at
	return a, nil
}
func (s *Store) PostDepreciation(_ context.Context, scope tenancy.Scope, actor, assetID, id, journalID string, period time.Time, idem, hash string, at time.Time) (advancedfinance.Depreciation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.assets.depreciate")] {
		return advancedfinance.Depreciation{}, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "finance.asset.depreciate.v1", idem)
	if v, ok := s.state.idempotencies[ik]; ok {
		if v.RequestHash != hash {
			return advancedfinance.Depreciation{}, sales.ErrIdempotencyConflict
		}
		return s.state.depreciation[v.ResultID], nil
	}
	key := companyEntityKey(scope.TenantID, scope.CompanyID, assetID)
	a, ok := s.state.assets[key]
	if !ok {
		return advancedfinance.Depreciation{}, sales.ErrNotFound
	}
	if a.Status != advancedfinance.Active {
		return advancedfinance.Depreciation{}, advancedfinance.ErrInvalidTransition
	}
	remaining := a.CostMinor - a.ResidualMinor - a.AccumulatedDepreciationMinor
	if remaining <= 0 {
		return advancedfinance.Depreciation{}, advancedfinance.ErrAssetFullyDepreciated
	}
	amount := (a.CostMinor - a.ResidualMinor) / int64(a.UsefulLifeMonths)
	if amount < 1 {
		amount = 1
	}
	if amount > remaining {
		amount = remaining
	}
	r := advancedfinance.Depreciation{ID: id, AssetID: assetID, Period: period, AmountMinor: amount, JournalID: journalID, PostedBy: actor, PostedAt: at}
	s.state.journals = append(s.state.journals, finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FIXED_ASSET_DEPRECIATION", SourceID: id, Currency: a.Currency, OccurredAt: period, Entries: []finance.JournalEntry{{AccountID: a.DepreciationExpenseAccountID, DebitMinor: amount}, {AccountID: a.AccumulatedDepreciationAccountID, CreditMinor: amount}}})
	a.AccumulatedDepreciationMinor += amount
	a.NetBookValueMinor = a.CostMinor - a.AccumulatedDepreciationMinor
	s.state.assets[key] = a
	s.state.depreciation[id] = r
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	return r, nil
}
func (s *Store) DisposeAsset(_ context.Context, scope tenancy.Scope, actor, assetID string, proceeds int64, proceedsAccount, reason, journalID, idem, hash string, at time.Time) (advancedfinance.Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.assets.dispose")] {
		return advancedfinance.Asset{}, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "finance.asset.dispose.v1", idem)
	key := companyEntityKey(scope.TenantID, scope.CompanyID, assetID)
	if v, ok := s.state.idempotencies[ik]; ok {
		if v.RequestHash != hash {
			return advancedfinance.Asset{}, sales.ErrIdempotencyConflict
		}
		return s.state.assets[key], nil
	}
	a, ok := s.state.assets[key]
	if !ok {
		return a, sales.ErrNotFound
	}
	if a.Status != advancedfinance.Active {
		return a, advancedfinance.ErrInvalidTransition
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
	s.state.journals = append(s.state.journals, finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FIXED_ASSET_DISPOSAL", SourceID: assetID, Currency: a.Currency, OccurredAt: at, Entries: entries})
	a.Status = advancedfinance.Disposed
	a.DisposedAt = &at
	s.state.assets[key] = a
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: assetID}
	return a, nil
}
