package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) ListIntercompany(_ context.Context, scope tenancy.Scope, actor string) (groupfinance.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := groupfinance.Page{Items: []groupfinance.Transaction{}}
	if !s.state.permissions[permissionKey(scope, actor, "finance.intercompany.read")] {
		return r, sales.ErrForbidden
	}
	for _, v := range s.state.intercompany {
		if v.Scope.TenantID == scope.TenantID && (v.Scope.CompanyID == scope.CompanyID || v.CounterpartyCompanyID == scope.CompanyID) {
			r.Items = append(r.Items, v)
		}
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].CreatedAt.After(r.Items[j].CreatedAt) })
	return r, nil
}
func (s *Store) CreateIntercompany(_ context.Context, v groupfinance.Transaction, idem, hash string) (groupfinance.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "finance.intercompany.manage")] {
		return v, sales.ErrForbidden
	}
	ik := idempotencyKey(v.Scope, "finance.intercompany.create.v1", idem)
	if old, ok := s.state.idempotencies[ik]; ok {
		if old.RequestHash != hash {
			return v, sales.ErrIdempotencyConflict
		}
		return s.state.intercompany[old.ResultID], nil
	}
	s.state.intercompany[v.ID] = v
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: v.ID}
	return v, nil
}
func (s *Store) TransitionIntercompany(_ context.Context, view tenancy.Scope, actor, id string, to groupfinance.Status, _ string, sourceJournal, counterJournal, idem, hash string, at time.Time) (groupfinance.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "finance.intercompany.manage"
	if to != groupfinance.Submitted {
		permission = "finance.intercompany.approve"
	}
	if !s.state.permissions[permissionKey(view, actor, permission)] {
		return groupfinance.Transaction{}, sales.ErrForbidden
	}
	v, ok := s.state.intercompany[id]
	if !ok || v.Scope.TenantID != view.TenantID || (v.Scope.CompanyID != view.CompanyID && v.CounterpartyCompanyID != view.CompanyID) {
		return v, sales.ErrNotFound
	}
	ik := idempotencyKey(view, "finance.intercompany.transition."+string(to)+".v1", idem)
	if old, ok := s.state.idempotencies[ik]; ok {
		if old.RequestHash != hash {
			return v, sales.ErrIdempotencyConflict
		}
		return s.state.intercompany[id], nil
	}
	source := view.CompanyID == v.Scope.CompanyID
	counter := view.CompanyID == v.CounterpartyCompanyID
	valid := v.Status == groupfinance.Draft && to == groupfinance.Submitted && source || v.Status == groupfinance.Submitted && (to == groupfinance.SourceApproved || to == groupfinance.Rejected) && source || v.Status == groupfinance.SourceApproved && (to == groupfinance.Posted || to == groupfinance.Rejected) && counter
	if !valid {
		return v, groupfinance.ErrInvalidTransition
	}
	if to == groupfinance.SourceApproved && actor == v.CreatedBy {
		return v, groupfinance.ErrSeparationOfDuties
	}
	if to == groupfinance.Posted && (actor == v.CreatedBy || actor == v.SourceApprovedBy) {
		return v, groupfinance.ErrSeparationOfDuties
	}
	if to == groupfinance.Posted {
		open := map[string]bool{}
		for _, p := range s.state.periods {
			if p.TenantID == v.Scope.TenantID && (p.CompanyID == v.Scope.CompanyID || p.CompanyID == v.CounterpartyCompanyID) && !v.OccurredAt.Before(p.StartsAt) && v.OccurredAt.Before(p.EndsAt) {
				open[p.CompanyID] = p.Open
			}
		}
		if !open[v.Scope.CompanyID] || !open[v.CounterpartyCompanyID] {
			return v, groupfinance.ErrInvalidTransition
		}
		s.state.journals = append(s.state.journals, finance.Journal{ID: sourceJournal, TenantID: v.Scope.TenantID, CompanyID: v.Scope.CompanyID, SourceType: "INTERCOMPANY_" + string(v.Type), SourceID: v.ID, Currency: v.Currency, OccurredAt: v.OccurredAt, Entries: []finance.JournalEntry{{AccountID: v.SourceDebitAccountID, DebitMinor: v.AmountMinor}, {AccountID: v.SourceCreditAccountID, CreditMinor: v.AmountMinor}}}, finance.Journal{ID: counterJournal, TenantID: v.Scope.TenantID, CompanyID: v.CounterpartyCompanyID, SourceType: "INTERCOMPANY_" + string(v.Type), SourceID: v.ID, Currency: v.Currency, OccurredAt: v.OccurredAt, Entries: []finance.JournalEntry{{AccountID: v.CounterpartyDebitAccountID, DebitMinor: v.AmountMinor}, {AccountID: v.CounterpartyCreditAccountID, CreditMinor: v.AmountMinor}}})
		v.SourceJournalID = sourceJournal
		v.CounterpartyJournalID = counterJournal
		v.PostedBy = actor
	}
	if to == groupfinance.SourceApproved {
		v.SourceApprovedBy = actor
	}
	v.Status = to
	s.state.intercompany[id] = v
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	_ = at
	return v, nil
}
func (s *Store) Consolidation(_ context.Context, scope tenancy.Scope, actor string, asOf time.Time) (groupfinance.Consolidation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := groupfinance.Consolidation{Currency: "TZS", AsOf: asOf, Companies: []groupfinance.CompanySummary{}}
	if !s.state.permissions[permissionKey(scope, actor, "finance.consolidation.read")] {
		return r, sales.ErrForbidden
	}
	byCompany := map[string]*groupfinance.CompanySummary{}
	for _, j := range s.state.journals {
		if j.TenantID != scope.TenantID || j.OccurredAt.After(asOf) {
			continue
		}
		summary := byCompany[j.CompanyID]
		if summary == nil {
			summary = &groupfinance.CompanySummary{CompanyID: j.CompanyID, CompanyName: j.CompanyID}
			byCompany[j.CompanyID] = summary
		}
		for _, line := range j.Entries {
			a := s.state.glAccounts[companyEntityKey(scope.TenantID, j.CompanyID, line.AccountID)]
			switch a.Type {
			case "ASSET":
				summary.AssetsMinor += line.DebitMinor - line.CreditMinor
			case "LIABILITY":
				summary.LiabilitiesMinor += line.CreditMinor - line.DebitMinor
			case "EQUITY":
				summary.EquityMinor += line.CreditMinor - line.DebitMinor
			case "REVENUE":
				summary.RevenueMinor += line.CreditMinor - line.DebitMinor
			case "EXPENSE":
				summary.ExpenseMinor += line.DebitMinor - line.CreditMinor
			}
		}
	}
	for _, v := range byCompany {
		r.Companies = append(r.Companies, *v)
		r.AssetsBeforeMinor += v.AssetsMinor
		r.LiabilitiesBeforeMinor += v.LiabilitiesMinor
		r.EquityMinor += v.EquityMinor
		r.RevenueBeforeMinor += v.RevenueMinor
		r.ExpenseBeforeMinor += v.ExpenseMinor
	}
	for _, v := range s.state.intercompany {
		if v.Scope.TenantID == scope.TenantID && v.Status == groupfinance.Posted && !v.OccurredAt.After(asOf) {
			r.IntercompanyBalanceEliminationMinor += v.AmountMinor
			if v.Type == groupfinance.CostAllocation {
				r.IntercompanyActivityEliminationMinor += v.AmountMinor
			}
		}
	}
	r.AssetsMinor = r.AssetsBeforeMinor - r.IntercompanyBalanceEliminationMinor
	r.LiabilitiesMinor = r.LiabilitiesBeforeMinor - r.IntercompanyBalanceEliminationMinor
	r.RevenueMinor = r.RevenueBeforeMinor - r.IntercompanyActivityEliminationMinor
	r.ExpenseMinor = r.ExpenseBeforeMinor - r.IntercompanyActivityEliminationMinor
	r.NetProfitMinor = r.RevenueMinor - r.ExpenseMinor
	r.Balanced = r.AssetsMinor == r.LiabilitiesMinor+r.EquityMinor+r.NetProfitMinor
	return r, nil
}
