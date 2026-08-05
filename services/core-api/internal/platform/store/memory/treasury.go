package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

func (s *Store) ListFacilities(_ context.Context, scope tenancy.Scope, actor string) (treasury.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := treasury.Page{Items: []treasury.Facility{}}
	if !s.state.permissions[permissionKey(scope, actor, "finance.treasury.read")] {
		return r, sales.ErrForbidden
	}
	for _, f := range s.state.facilities {
		if f.Scope.TenantID == scope.TenantID && f.Scope.CompanyID == scope.CompanyID {
			f.Transactions = append([]treasury.Transaction(nil), f.Transactions...)
			r.Items = append(r.Items, f)
		}
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].Reference < r.Items[j].Reference })
	return r, nil
}
func (s *Store) CreateFacility(_ context.Context, f treasury.Facility, idem, hash string) (treasury.Facility, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(f.Scope, f.CreatedBy, "finance.treasury.manage")] {
		return f, sales.ErrForbidden
	}
	ik := idempotencyKey(f.Scope, "finance.treasury.facility.create.v1", idem)
	if v, ok := s.state.idempotencies[ik]; ok {
		if v.RequestHash != hash {
			return f, sales.ErrIdempotencyConflict
		}
		return s.state.facilities[companyEntityKey(f.Scope.TenantID, f.Scope.CompanyID, v.ResultID)], nil
	}
	k := companyEntityKey(f.Scope.TenantID, f.Scope.CompanyID, f.ID)
	s.state.facilities[k] = f
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: f.ID}
	return f, nil
}
func (s *Store) TransitionFacility(_ context.Context, scope tenancy.Scope, actor, id string, to treasury.Status, _ string, idem, hash string, at time.Time) (treasury.Facility, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "finance.treasury.manage"
	if to == treasury.Active || to == treasury.Rejected || to == treasury.Closed {
		permission = "finance.treasury.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, permission)] {
		return treasury.Facility{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, id)
	f, ok := s.state.facilities[k]
	if !ok {
		return f, sales.ErrNotFound
	}
	ik := idempotencyKey(scope, "finance.treasury.facility.transition."+string(to)+".v1", idem)
	if v, ok := s.state.idempotencies[ik]; ok {
		if v.RequestHash != hash {
			return f, sales.ErrIdempotencyConflict
		}
		return s.state.facilities[k], nil
	}
	valid := f.Status == treasury.Draft && to == treasury.Submitted || f.Status == treasury.Submitted && (to == treasury.Active || to == treasury.Rejected) || f.Status == treasury.Active && to == treasury.Closed && f.OutstandingPrincipalMinor == 0 && f.AccruedInterestMinor == 0
	if !valid {
		return f, treasury.ErrInvalidTransition
	}
	if (to == treasury.Active || to == treasury.Rejected) && f.CreatedBy == actor {
		return f, treasury.ErrSeparationOfDuties
	}
	f.Status = to
	if to == treasury.Active || to == treasury.Rejected {
		f.ApprovedBy = actor
	}
	if to == treasury.Closed {
		f.ClosedAt = &at
	}
	s.state.facilities[k] = f
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	return f, nil
}
func (s *Store) PostTransaction(_ context.Context, scope tenancy.Scope, actor, facilityID string, t treasury.Transaction, idem, hash string) (treasury.Facility, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.treasury.transact")] {
		return treasury.Facility{}, sales.ErrForbidden
	}
	k := companyEntityKey(scope.TenantID, scope.CompanyID, facilityID)
	f, ok := s.state.facilities[k]
	if !ok {
		return f, sales.ErrNotFound
	}
	ik := idempotencyKey(scope, "finance.treasury.transaction."+string(t.Type)+".v1", idem)
	if v, ok := s.state.idempotencies[ik]; ok {
		if v.RequestHash != hash {
			return f, sales.ErrIdempotencyConflict
		}
		return s.state.facilities[k], nil
	}
	if f.Status != treasury.Active {
		return f, treasury.ErrInvalidTransition
	}
	if t.OccurredAt.Before(f.StartDate) || !t.OccurredAt.Before(f.MaturityDate.AddDate(0, 0, 1)) {
		return f, treasury.ErrInvalidTransition
	}
	periodOpen := false
	for _, period := range s.state.periods {
		if period.TenantID == scope.TenantID && period.CompanyID == scope.CompanyID && !t.OccurredAt.Before(period.StartsAt) && t.OccurredAt.Before(period.EndsAt) {
			periodOpen = period.Open
			break
		}
	}
	if !periodOpen {
		return f, treasury.ErrInvalidTransition
	}
	entries := []finance.JournalEntry{}
	switch t.Type {
	case treasury.Drawdown:
		if t.AmountMinor > f.AvailableMinor {
			return f, treasury.ErrLimitExceeded
		}
		f.OutstandingPrincipalMinor += t.AmountMinor
		entries = []finance.JournalEntry{{AccountID: f.BankAccountID, DebitMinor: t.AmountMinor}, {AccountID: f.PrincipalAccountID, CreditMinor: t.AmountMinor}}
	case treasury.PrincipalRepayment:
		if t.AmountMinor > f.OutstandingPrincipalMinor {
			return f, treasury.ErrLimitExceeded
		}
		f.OutstandingPrincipalMinor -= t.AmountMinor
		entries = []finance.JournalEntry{{AccountID: f.PrincipalAccountID, DebitMinor: t.AmountMinor}, {AccountID: f.BankAccountID, CreditMinor: t.AmountMinor}}
	case treasury.InterestAccrual:
		f.AccruedInterestMinor += t.AmountMinor
		entries = []finance.JournalEntry{{AccountID: f.InterestExpenseAccountID, DebitMinor: t.AmountMinor}, {AccountID: f.AccruedInterestAccountID, CreditMinor: t.AmountMinor}}
	case treasury.InterestPayment:
		if t.AmountMinor > f.AccruedInterestMinor {
			return f, treasury.ErrLimitExceeded
		}
		f.AccruedInterestMinor -= t.AmountMinor
		entries = []finance.JournalEntry{{AccountID: f.AccruedInterestAccountID, DebitMinor: t.AmountMinor}, {AccountID: f.BankAccountID, CreditMinor: t.AmountMinor}}
	}
	f.AvailableMinor = f.LimitMinor - f.OutstandingPrincipalMinor
	f.Transactions = append(f.Transactions, t)
	s.state.journals = append(s.state.journals, finance.Journal{ID: t.JournalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "TREASURY_" + string(t.Type), SourceID: t.ID, Currency: f.Currency, OccurredAt: t.OccurredAt, Entries: entries})
	s.state.facilities[k] = f
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: t.ID}
	return f, nil
}
