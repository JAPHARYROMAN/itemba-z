package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func financialKey(scope tenancy.Scope, id string) string { return scopeKey(scope) + ":" + id }
func cloneFinancial(d financialops.Document) financialops.Document {
	d.Lines = append([]finance.JournalEntry(nil), d.Lines...)
	return d
}
func (s *Store) CreateFinancialDocument(_ context.Context, d financialops.Document, idem, hash string) (financialops.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permission := "finance.journals.manage"
	if d.Type == financialops.CashTransfer {
		permission = "finance.cash.transfer"
	}
	if d.Type == financialops.BankAdjustment {
		permission = "finance.bank.adjust"
	}
	if !s.state.permissions[permissionKey(d.Scope, d.CreatedBy, permission)] {
		return d, sales.ErrForbidden
	}
	ik := idempotencyKey(d.Scope, "financial.document.create.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return d, sales.ErrIdempotencyConflict
		}
		return cloneFinancial(s.state.financialDocuments[financialKey(d.Scope, prior.ResultID)]), nil
	}
	if d.Type == financialops.Reversal {
		orig, ok := s.state.financialDocuments[financialKey(d.Scope, d.ReversesDocumentID)]
		if !ok {
			return d, sales.ErrNotFound
		}
		if orig.Status != financialops.Posted {
			return d, financialops.ErrInvalidTransition
		}
		for _, v := range s.state.financialDocuments {
			if v.ReversesDocumentID == orig.ID {
				return d, financialops.ErrAlreadyReversed
			}
		}
		d.Currency = orig.Currency
		for _, j := range s.state.journals {
			if j.ID == orig.JournalID {
				d.Lines = j.Reversed("x", "x", "x", d.AccountingAt).Entries
			}
		}
	}
	if d.Type == financialops.ManualJournal {
		for _, line := range d.Lines {
			account, ok := s.state.glAccounts[accountKey(d.Scope, line.AccountID)]
			if !ok || account.Status != financialops.GovernanceActive || !account.AllowManualPosting || account.ControlAccount {
				return d, financialops.ErrAccountGovernance
			}
		}
	}
	d.Number = "FIN-" + d.ID[len(d.ID)-8:]
	if d.Type == financialops.CashTransfer {
		d.Lines[0].AccountID = "__AMOUNT__"
	}
	s.state.financialDocuments[financialKey(d.Scope, d.ID)] = cloneFinancial(d)
	appendFinancialEvidence(s.state, d.Scope, d.CreatedBy, "financial.document_created", "financial_document", d.ID, d.CreatedAt)
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: d.ID}
	return d, nil
}
func (s *Store) ListFinancialDocuments(_ context.Context, scope tenancy.Scope, actor, cursor string, limit int) (financialops.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.journals.read")] {
		return financialops.Page{}, sales.ErrForbidden
	}
	var items []financialops.Document
	for _, d := range s.state.financialDocuments {
		if d.Scope == scope && (cursor == "" || d.ID < cursor) {
			items = append(items, cloneFinancial(d))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	p := financialops.Page{Items: items}
	if len(items) > limit {
		next := items[limit-1].ID
		p.NextCursor = &next
		p.Items = items[:limit]
	}
	return p, nil
}
func (s *Store) FinancialDocument(_ context.Context, scope tenancy.Scope, actor, id string) (financialops.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.journals.read")] {
		return financialops.Document{}, sales.ErrForbidden
	}
	d, ok := s.state.financialDocuments[financialKey(scope, id)]
	if !ok {
		return d, sales.ErrNotFound
	}
	return cloneFinancial(d), nil
}
func (s *Store) TransitionFinancialDocument(_ context.Context, scope tenancy.Scope, actor, id string, to financialops.Status, reason, journalID, idem, hash string, at time.Time) (financialops.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "finance.journals.manage"
	if to == financialops.Posted || to == financialops.Rejected {
		perm = "finance.journals.post"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return financialops.Document{}, sales.ErrForbidden
	}
	op := "financial.document.transition." + string(to) + ".v1"
	ik := idempotencyKey(scope, op, idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return financialops.Document{}, sales.ErrIdempotencyConflict
		}
		return cloneFinancial(s.state.financialDocuments[financialKey(scope, prior.ResultID)]), nil
	}
	d, ok := s.state.financialDocuments[financialKey(scope, id)]
	if !ok {
		return d, sales.ErrNotFound
	}
	valid := d.Status == financialops.Draft && to == financialops.Submitted || d.Status == financialops.Submitted && (to == financialops.Posted || to == financialops.Rejected)
	if !valid {
		return d, financialops.ErrInvalidTransition
	}
	if (to == financialops.Posted || to == financialops.Rejected) && d.CreatedBy == actor {
		return d, financialops.ErrSeparationOfDuties
	}
	if to == financialops.Posted {
		open := false
		for _, p := range s.state.periods {
			if p.TenantID == scope.TenantID && p.CompanyID == scope.CompanyID && !d.AccountingAt.Before(p.StartsAt) && d.AccountingAt.Before(p.EndsAt) {
				open = p.Open
			}
		}
		if !open {
			return d, financialops.ErrPeriodClosed
		}
		entries, err := memoryPostingEntries(s.state, d)
		if err != nil {
			return d, err
		}
		j := finance.Journal{ID: journalID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SourceType: "FINANCIAL_DOCUMENT", SourceID: d.ID, Currency: d.Currency, OccurredAt: d.AccountingAt, Entries: entries}
		if err = j.Validate(); err != nil {
			return d, err
		}
		s.state.journals = append(s.state.journals, j)
		d.JournalID = journalID
		d.PostedBy = actor
	}
	if to == financialops.Submitted {
		d.SubmittedBy = actor
	}
	d.Status = to
	s.state.financialDocuments[financialKey(scope, id)] = cloneFinancial(d)
	appendFinancialEvidence(s.state, scope, actor, "financial.document_"+string(to), "financial_document", id, at)
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	return d, nil
}
func memoryPostingEntries(st *state, d financialops.Document) ([]finance.JournalEntry, error) {
	if d.Type == financialops.ManualJournal || d.Type == financialops.Reversal {
		return d.Lines, nil
	}
	from, ok := st.bankAccounts[scopeKey(d.Scope)+":"+d.FromAccountID]
	if !ok || !from.Active || from.Currency != d.Currency {
		return nil, financialops.ErrInvalidCommand
	}
	amount := d.Lines[0].DebitMinor
	if d.Type == financialops.CashTransfer {
		to, ok := st.bankAccounts[scopeKey(d.Scope)+":"+d.ToAccountID]
		if !ok || !to.Active || to.Currency != d.Currency {
			return nil, financialops.ErrInvalidCommand
		}
		return []finance.JournalEntry{{AccountID: to.GLAccountID, DebitMinor: amount}, {AccountID: from.GLAccountID, CreditMinor: amount}}, nil
	}
	l := d.Lines[0]
	if l.DebitMinor > 0 {
		return []finance.JournalEntry{{AccountID: from.GLAccountID, DebitMinor: l.DebitMinor}, {AccountID: l.AccountID, CreditMinor: l.DebitMinor}}, nil
	}
	return []finance.JournalEntry{{AccountID: l.AccountID, DebitMinor: l.CreditMinor}, {AccountID: from.GLAccountID, CreditMinor: l.CreditMinor}}, nil
}
func (s *Store) ListFiscalPeriods(_ context.Context, scope tenancy.Scope, actor string) ([]financialops.FiscalPeriod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.periods.read")] {
		return nil, sales.ErrForbidden
	}
	var out []financialops.FiscalPeriod
	for _, p := range s.state.periods {
		if p.TenantID == scope.TenantID && p.CompanyID == scope.CompanyID {
			out = append(out, financialops.FiscalPeriod{ID: p.ID, TenantID: p.TenantID, CompanyID: p.CompanyID, StartsAt: p.StartsAt, EndsAt: p.EndsAt, Open: p.Open})
		}
	}
	return out, nil
}
func (s *Store) ListFiscalPeriodActions(_ context.Context, scope tenancy.Scope, actor string) (financialops.PeriodActionPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.periods.read")] {
		return financialops.PeriodActionPage{}, sales.ErrForbidden
	}
	result := financialops.PeriodActionPage{Items: []financialops.PeriodActionRequest{}}
	for key, value := range s.state.periodActions {
		if len(key) >= len(scopeKey(scope)) && key[:len(scopeKey(scope))] == scopeKey(scope) {
			result.Items = append(result.Items, value)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].RequestedAt.After(result.Items[j].RequestedAt) })
	return result, nil
}
func (s *Store) RequestFiscalPeriodAction(_ context.Context, scope tenancy.Scope, actor, periodID string, action financialops.PeriodAction, reason, requestID, idem, hash string, at time.Time) (financialops.PeriodActionRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "finance.periods.close"
	if action == financialops.ReopenPeriod {
		perm = "finance.periods.reopen"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return financialops.PeriodActionRequest{}, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "fiscal.period.request.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return financialops.PeriodActionRequest{}, sales.ErrIdempotencyConflict
		}
		return s.state.periodActions[financialKey(scope, prior.ResultID)], nil
	}
	found, open := false, false
	for _, p := range s.state.periods {
		if p.ID == periodID && p.TenantID == scope.TenantID && p.CompanyID == scope.CompanyID {
			found, open = true, p.Open
		}
	}
	if !found {
		return financialops.PeriodActionRequest{}, sales.ErrNotFound
	}
	if (action == financialops.ClosePeriod && !open) || (action == financialops.ReopenPeriod && open) {
		return financialops.PeriodActionRequest{}, financialops.ErrInvalidTransition
	}
	r := financialops.PeriodActionRequest{ID: requestID, PeriodID: periodID, Action: action, Reason: reason, RequestedBy: actor, RequestedAt: at}
	s.state.periodActions[financialKey(scope, requestID)] = r
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: requestID}
	appendFinancialEvidence(s.state, scope, actor, "fiscal_period.action_requested", "fiscal_period_action", requestID, at)
	return r, nil
}
func (s *Store) ApproveFiscalPeriodAction(_ context.Context, scope tenancy.Scope, actor, requestID, reason, idem, hash string, at time.Time) (financialops.PeriodActionRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.state.periodActions[financialKey(scope, requestID)]
	if !ok {
		return r, sales.ErrNotFound
	}
	perm := "finance.periods.close"
	if r.Action == financialops.ReopenPeriod {
		perm = "finance.periods.reopen"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return r, sales.ErrForbidden
	}
	if r.RequestedBy == actor {
		return r, financialops.ErrSeparationOfDuties
	}
	ik := idempotencyKey(scope, "fiscal.period.approve.v1", idem)
	if prior, found := s.state.idempotencies[ik]; found {
		if prior.RequestHash != hash {
			return r, sales.ErrIdempotencyConflict
		}
		return s.state.periodActions[financialKey(scope, prior.ResultID)], nil
	}
	if r.ApprovedAt != nil {
		return r, financialops.ErrInvalidTransition
	}
	if r.Action == financialops.ClosePeriod {
		var startsAt, endsAt time.Time
		for _, p := range s.state.periods {
			if p.ID == r.PeriodID && p.TenantID == scope.TenantID && p.CompanyID == scope.CompanyID {
				startsAt, endsAt = p.StartsAt, p.EndsAt
			}
		}
		if startsAt.IsZero() {
			return r, sales.ErrNotFound
		}
		for _, b := range s.state.bankStatements {
			if b.Scope.TenantID == scope.TenantID && b.Scope.CompanyID == scope.CompanyID && b.PeriodEnd.After(startsAt) && !b.PeriodEnd.After(endsAt) && b.Status != banking.Reconciled {
				return r, financialops.ErrPeriodCloseBlocked
			}
		}
		for _, d := range s.state.financialDocuments {
			if d.Scope.TenantID == scope.TenantID && d.Scope.CompanyID == scope.CompanyID && !d.AccountingAt.Before(startsAt) && d.AccountingAt.Before(endsAt) && d.Status != financialops.Posted && d.Status != financialops.Rejected {
				return r, financialops.ErrPeriodCloseBlocked
			}
		}
	}
	for i, p := range s.state.periods {
		if p.ID == r.PeriodID && p.TenantID == scope.TenantID && p.CompanyID == scope.CompanyID {
			s.state.periods[i].Open = r.Action == financialops.ReopenPeriod
		}
	}
	r.ApprovedBy = actor
	r.ApprovedAt = &at
	s.state.periodActions[financialKey(scope, requestID)] = r
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: requestID}
	appendFinancialEvidence(s.state, scope, actor, "fiscal_period.action_approved", "fiscal_period_action", requestID, at)
	return r, nil
}

func appendFinancialEvidence(st *state, scope tenancy.Scope, actor, action, entityType, entityID string, at time.Time) {
	id := entityID + ":" + action
	st.audits = append(st.audits, audit.Event{ID: id, TenantID: scope.TenantID, CompanyID: scope.CompanyID, ActorID: actor, Action: action, EntityType: entityType, EntityID: entityID, CorrelationID: entityID, CausationID: entityID, Data: []byte(`{}`), OccurredAt: at})
	st.outbox = append(st.outbox, outbox.Event{ID: id, TenantID: scope.TenantID, CompanyID: scope.CompanyID, AggregateType: entityType, AggregateID: entityID, EventType: action, Version: 1, CorrelationID: entityID, CausationID: entityID, Payload: []byte(`{}`), OccurredAt: at})
}
