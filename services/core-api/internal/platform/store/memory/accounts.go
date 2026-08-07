package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func accountKey(scope tenancy.Scope, id string) string {
	return companyEntityKey(scope.TenantID, scope.CompanyID, id)
}
func mappingKey(scope tenancy.Scope, id string) string {
	return companyEntityKey(scope.TenantID, scope.CompanyID, id)
}

func (s *Store) SeedGLAccount(value financialops.GLAccount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.glAccounts[accountKey(tenancy.Scope{TenantID: value.TenantID, CompanyID: value.CompanyID}, value.ID)] = value
}

func (s *Store) ListGLAccounts(_ context.Context, scope tenancy.Scope, actor string) (financialops.GLAccountPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.accounts.read")] {
		return financialops.GLAccountPage{}, sales.ErrForbidden
	}
	result := financialops.GLAccountPage{Items: []financialops.GLAccount{}}
	for _, value := range s.state.glAccounts {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID {
			result.Items = append(result.Items, value)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].Code < result.Items[j].Code })
	return result, nil
}

func (s *Store) CreateGLAccount(_ context.Context, value financialops.GLAccount, scope tenancy.Scope, idem, hash string) (financialops.GLAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, value.CreatedBy, "finance.accounts.manage")] {
		return value, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "finance.account.create.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return value, sales.ErrIdempotencyConflict
		}
		return s.state.glAccounts[accountKey(scope, prior.ResultID)], nil
	}
	if _, exists := s.state.glAccounts[accountKey(scope, value.ID)]; exists {
		return value, financialops.ErrAccountGovernance
	}
	if value.ParentAccountID != "" {
		parent, ok := s.state.glAccounts[accountKey(scope, value.ParentAccountID)]
		if !ok || parent.Status != financialops.GovernanceActive {
			return value, financialops.ErrAccountGovernance
		}
	}
	s.state.glAccounts[accountKey(scope, value.ID)] = value
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: value.ID}
	appendFinancialEvidence(s.state, scope, value.CreatedBy, "finance.account_submitted", "gl_account", value.RecordID, value.CreatedAt)
	return value, nil
}

func (s *Store) DecideGLAccount(_ context.Context, scope tenancy.Scope, actor, id string, status financialops.GovernanceStatus, _ string, idem, hash string, at time.Time) (financialops.GLAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.accounts.approve")] {
		return financialops.GLAccount{}, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "finance.account.decide.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return financialops.GLAccount{}, sales.ErrIdempotencyConflict
		}
		return s.state.glAccounts[accountKey(scope, prior.ResultID)], nil
	}
	value, ok := s.state.glAccounts[accountKey(scope, id)]
	if !ok {
		return value, sales.ErrNotFound
	}
	if value.Status != financialops.GovernanceSubmitted || value.CreatedBy == actor {
		return value, financialops.ErrAccountGovernance
	}
	value.Status, value.ApprovedBy, value.ApprovedAt = status, actor, &at
	s.state.glAccounts[accountKey(scope, id)] = value
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	appendFinancialEvidence(s.state, scope, actor, "finance.account_"+string(status), "gl_account", value.RecordID, at)
	return value, nil
}

func (s *Store) ListPostingMappings(_ context.Context, scope tenancy.Scope, actor string) (financialops.PostingMappingPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.accounts.read")] {
		return financialops.PostingMappingPage{}, sales.ErrForbidden
	}
	result := financialops.PostingMappingPage{Items: []financialops.PostingMapping{}}
	for _, value := range s.state.postingMappings {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID {
			result.Items = append(result.Items, value)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].EffectiveFrom.After(result.Items[j].EffectiveFrom) })
	return result, nil
}

func (s *Store) CreatePostingMapping(_ context.Context, value financialops.PostingMapping, scope tenancy.Scope, idem, hash string) (financialops.PostingMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, value.CreatedBy, "finance.accounts.manage")] {
		return value, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "finance.mapping.create.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return value, sales.ErrIdempotencyConflict
		}
		return s.state.postingMappings[mappingKey(scope, prior.ResultID)], nil
	}
	account, ok := s.state.glAccounts[accountKey(scope, value.AccountID)]
	if !ok || account.Status != financialops.GovernanceActive || !mappingCompatibleMemory(value.Key, account.Type) {
		return value, financialops.ErrAccountGovernance
	}
	for _, existing := range s.state.postingMappings {
		if existing.TenantID == scope.TenantID && existing.CompanyID == scope.CompanyID && existing.Key == value.Key && existing.EffectiveFrom.Equal(value.EffectiveFrom) {
			return value, financialops.ErrAccountGovernance
		}
	}
	s.state.postingMappings[mappingKey(scope, value.ID)] = value
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: value.ID}
	appendFinancialEvidence(s.state, scope, value.CreatedBy, "finance.mapping_submitted", "posting_mapping", value.ID, value.CreatedAt)
	return value, nil
}

func (s *Store) DecidePostingMapping(_ context.Context, scope tenancy.Scope, actor, id string, status financialops.GovernanceStatus, _ string, idem, hash string, at time.Time) (financialops.PostingMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "finance.accounts.approve")] {
		return financialops.PostingMapping{}, sales.ErrForbidden
	}
	ik := idempotencyKey(scope, "finance.mapping.decide.v1", idem)
	if prior, ok := s.state.idempotencies[ik]; ok {
		if prior.RequestHash != hash {
			return financialops.PostingMapping{}, sales.ErrIdempotencyConflict
		}
		return s.state.postingMappings[mappingKey(scope, prior.ResultID)], nil
	}
	value, ok := s.state.postingMappings[mappingKey(scope, id)]
	if !ok {
		return value, sales.ErrNotFound
	}
	if value.Status != financialops.GovernanceSubmitted || value.CreatedBy == actor {
		return value, financialops.ErrAccountGovernance
	}
	value.Status, value.ApprovedBy, value.ApprovedAt = status, actor, &at
	s.state.postingMappings[mappingKey(scope, id)] = value
	s.state.idempotencies[ik] = idempotency{RequestHash: hash, ResultID: id}
	appendFinancialEvidence(s.state, scope, actor, "finance.mapping_"+string(status), "posting_mapping", id, at)
	return value, nil
}

func mappingCompatibleMemory(key financialops.MappingKey, accountType financialops.AccountType) bool {
	switch key {
	case financialops.MapSalesReceivable, financialops.MapPaymentCash, financialops.MapPaymentMobileMoney, financialops.MapPaymentBankCard, financialops.MapPaymentBankTransfer, financialops.MapStockInTransit:
		return accountType == financialops.AccountAsset
	case financialops.MapSalesTaxPayable, financialops.MapProcurementGRNI, financialops.MapProcurementPayable:
		return accountType == financialops.AccountLiability
	case financialops.MapInventoryAdjustment:
		return accountType == financialops.AccountExpense
	}
	return false
}
