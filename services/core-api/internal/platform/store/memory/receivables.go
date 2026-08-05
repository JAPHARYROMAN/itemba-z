package memory

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (t *transaction) CustomerCreditPolicy(_ context.Context, scope tenancy.Scope, customerID string, at time.Time) (customers.CreditPolicy, error) {
	return activeMemoryCreditPolicy(t.state, scope, customerID, at)
}

func activeMemoryCreditPolicy(current *state, scope tenancy.Scope, customerID string, at time.Time) (customers.CreditPolicy, error) {
	var selected customers.CreditPolicy
	found := false
	for _, value := range current.creditPolicies {
		if value.Scope.TenantID != scope.TenantID || value.Scope.CompanyID != scope.CompanyID || value.CustomerID != customerID || value.EffectiveFrom.After(at) {
			continue
		}
		if !found || value.EffectiveFrom.After(selected.EffectiveFrom) {
			selected, found = value, true
		}
	}
	if !found {
		return customers.CreditPolicy{}, customers.ErrCreditPolicyMissing
	}
	selected.Scope = scope
	return selected, nil
}

func (t *transaction) CustomerReceivableAging(_ context.Context, scope tenancy.Scope, customerID string, at time.Time) (customers.ReceivableAging, error) {
	return memoryReceivableAging(t.state, scope, customerID, at), nil
}

func memoryReceivableAging(current *state, scope tenancy.Scope, customerID string, at time.Time) customers.ReceivableAging {
	result := customers.ReceivableAging{}
	for _, entry := range current.ledger {
		if entry.TenantID == scope.TenantID && entry.CompanyID == scope.CompanyID && entry.CustomerID == customerID {
			result.LedgerBalanceMinor += entry.AmountMinor
		}
	}
	for _, item := range current.receivableItems {
		if item.TenantID != scope.TenantID || item.CompanyID != scope.CompanyID || item.CustomerID != customerID {
			continue
		}
		outstanding := receivableOutstanding(current, item)
		if item.Kind != customers.ReceivableInvoice {
			result.UnappliedCreditMinor += outstanding
			continue
		}
		result.OpenInvoiceMinor += outstanding
		if item.DueAt == nil || outstanding == 0 || !item.DueAt.Before(at) {
			result.Buckets.CurrentMinor += outstanding
			continue
		}
		days := int64(at.UTC().Truncate(24*time.Hour).Sub(item.DueAt.UTC().Truncate(24*time.Hour)) / (24 * time.Hour))
		if days < 1 {
			days = 1
		}
		result.OverdueMinor += outstanding
		if days > result.OldestOverdueDays {
			result.OldestOverdueDays = days
		}
		switch {
		case days <= 30:
			result.Buckets.Days1To30 += outstanding
		case days <= 60:
			result.Buckets.Days31To60 += outstanding
		case days <= 90:
			result.Buckets.Days61To90 += outstanding
		default:
			result.Buckets.DaysOver90 += outstanding
		}
	}
	result.CalculatedExposure = result.OpenInvoiceMinor - result.UnappliedCreditMinor
	result.Reconciled = result.CalculatedExposure == result.LedgerBalanceMinor
	return result
}

func (t *transaction) AppendReceivableItem(_ context.Context, value customers.ReceivableItem) error {
	key := companyEntityKey(value.TenantID, value.CompanyID, value.ID)
	if _, exists := t.state.receivableItems[key]; exists {
		return sales.ErrInvalidCommand
	}
	for _, found := range t.state.receivableItems {
		if found.TenantID == value.TenantID && found.CompanyID == value.CompanyID && found.SourceType == value.SourceType && found.SourceID == value.SourceID {
			return sales.ErrInvalidCommand
		}
	}
	t.state.receivableItems[key] = value
	return nil
}

func (t *transaction) ReceivableItemBySource(_ context.Context, scope tenancy.Scope, sourceType, sourceID string) (customers.ReceivableItem, error) {
	for _, value := range t.state.receivableItems {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.SourceType == sourceType && value.SourceID == sourceID {
			value.OutstandingMinor = receivableOutstanding(t.state, value)
			return value, nil
		}
	}
	return customers.ReceivableItem{}, sales.ErrNotFound
}

func (t *transaction) AppendReceivableAllocation(_ context.Context, value customers.ReceivableAllocation) error {
	debit, debitOK := t.state.receivableItems[companyEntityKey(value.TenantID, value.CompanyID, value.DebitItemID)]
	credit, creditOK := t.state.receivableItems[companyEntityKey(value.TenantID, value.CompanyID, value.CreditItemID)]
	if !debitOK || !creditOK || debit.CustomerID != value.CustomerID || credit.CustomerID != value.CustomerID ||
		debit.Kind != customers.ReceivableInvoice || credit.Kind == customers.ReceivableInvoice || debit.Currency != credit.Currency ||
		value.AmountMinor <= 0 || value.AmountMinor > receivableOutstanding(t.state, debit) || value.AmountMinor > receivableOutstanding(t.state, credit) {
		return sales.ErrInvalidCommand
	}
	for _, found := range t.state.receivableAllocations {
		if found.ID == value.ID || (found.TenantID == value.TenantID && found.CompanyID == value.CompanyID && found.DebitItemID == value.DebitItemID && found.CreditItemID == value.CreditItemID) {
			return sales.ErrInvalidCommand
		}
	}
	t.state.receivableAllocations = append(t.state.receivableAllocations, value)
	return nil
}

func receivableOutstanding(current *state, item customers.ReceivableItem) int64 {
	allocated := int64(0)
	for _, value := range current.receivableAllocations {
		if value.TenantID != item.TenantID || value.CompanyID != item.CompanyID {
			continue
		}
		if item.Kind == customers.ReceivableInvoice && value.DebitItemID == item.ID {
			allocated += value.AmountMinor
		} else if item.Kind != customers.ReceivableInvoice && value.CreditItemID == item.ID {
			allocated += value.AmountMinor
		}
	}
	return item.AmountMinor - allocated
}

func (s *Store) CustomerAccountDetail(_ context.Context, scope tenancy.Scope, actorID, customerID string, at time.Time) (customers.AccountDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "customers.accounts.read")] {
		return customers.AccountDetail{}, sales.ErrForbidden
	}
	account, ok := s.state.customers[companyEntityKey(scope.TenantID, scope.CompanyID, customerID)]
	if !ok {
		return customers.AccountDetail{}, sales.ErrNotFound
	}
	policy, err := activeMemoryCreditPolicy(s.state, scope, customerID, at)
	if err != nil {
		return customers.AccountDetail{}, err
	}
	detail := customers.AccountDetail{Customer: account, ActivePolicy: policy, Aging: memoryReceivableAging(s.state, scope, customerID, at)}
	for _, candidate := range s.state.creditPolicies {
		if candidate.Scope.TenantID == scope.TenantID && candidate.Scope.CompanyID == scope.CompanyID && candidate.CustomerID == customerID && candidate.EffectiveFrom.After(at) {
			candidate.Scope = scope
			detail.ScheduledPolicies = append(detail.ScheduledPolicies, candidate)
		}
	}
	sort.Slice(detail.ScheduledPolicies, func(i, j int) bool {
		return detail.ScheduledPolicies[i].EffectiveFrom.Before(detail.ScheduledPolicies[j].EffectiveFrom)
	})
	for _, item := range s.state.receivableItems {
		if item.TenantID == scope.TenantID && item.CompanyID == scope.CompanyID && item.CustomerID == customerID {
			item.OutstandingMinor = receivableOutstanding(s.state, item)
			if item.OutstandingMinor > 0 {
				detail.OpenItems = append(detail.OpenItems, item)
			}
		}
	}
	sort.Slice(detail.OpenItems, func(i, j int) bool { return detail.OpenItems[i].DocumentAt.After(detail.OpenItems[j].DocumentAt) })
	return detail, nil
}

func (s *Store) ScheduleCustomerCreditPolicy(_ context.Context, policy customers.CreditPolicy, policyAudit audit.Event, policyEvent outbox.Event, commandAt time.Time) (customers.CreditPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(policy.Scope, policy.ApprovedBy, "customers.credit.manage")] {
		return customers.CreditPolicy{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(policy.Scope, receivables.OperationName(), policy.IdempotencyKey)
	if found, ok := s.state.idempotencies[idemKey]; ok {
		if found.RequestHash != policy.RequestHash {
			return customers.CreditPolicy{}, sales.ErrIdempotencyConflict
		}
		for _, existing := range s.state.creditPolicies {
			if existing.ID == found.ResultID {
				existing.Scope = policy.Scope
				return existing, nil
			}
		}
		return customers.CreditPolicy{}, sales.ErrNotFound
	}
	account, ok := s.state.customers[companyEntityKey(policy.Scope.TenantID, policy.Scope.CompanyID, policy.CustomerID)]
	if !ok {
		return customers.CreditPolicy{}, sales.ErrNotFound
	}
	if account.General && (policy.CreditEnabled || policy.CreditLimitMinor != 0) {
		return customers.CreditPolicy{}, sales.ErrGeneralCustomerCredit
	}
	if policy.EffectiveFrom.Before(commandAt.Add(-5 * time.Minute)) {
		return customers.CreditPolicy{}, customers.ErrCreditPolicyBackdated
	}
	for _, existing := range s.state.creditPolicies {
		if existing.Scope.TenantID == policy.Scope.TenantID && existing.Scope.CompanyID == policy.Scope.CompanyID && existing.CustomerID == policy.CustomerID && !policy.EffectiveFrom.After(existing.EffectiveFrom) {
			return customers.CreditPolicy{}, customers.ErrCreditPolicySequence
		}
	}
	payload, err := json.Marshal(policy)
	if err != nil {
		return customers.CreditPolicy{}, err
	}
	policyAudit.Data, policyEvent.Payload = payload, payload
	s.state.creditPolicies = append(s.state.creditPolicies, policy)
	s.state.audits = append(s.state.audits, policyAudit)
	s.state.outbox = append(s.state.outbox, policyEvent)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: policy.RequestHash, ResultID: policy.ID}
	return policy, nil
}
