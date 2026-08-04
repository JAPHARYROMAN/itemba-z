package memory

import (
	"context"
	"sort"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) RecordReconciliationCase(_ context.Context, value mobile.ReconciliationCase, caseAudit audit.Event, caseEvent outbox.Event) (mobile.ReconciliationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(value.Scope, value.CreatedBy, "mobile.sales.sync")] {
		return mobile.ReconciliationCase{}, sales.ErrForbidden
	}
	device, ok := s.state.devices[deviceKey(value.Scope.TenantID, value.DeviceID)]
	if !ok {
		return mobile.ReconciliationCase{}, devices.ErrNotEnrolled
	}
	if device.Scope != value.Scope || device.ActorID != value.CreatedBy {
		return mobile.ReconciliationCase{}, devices.ErrScopeMismatch
	}
	if device.Status != devices.StatusActive {
		return mobile.ReconciliationCase{}, devices.ErrNotActive
	}
	commandKey := reconciliationCommandKey(value.Scope, value.DeviceID, value.ClientTransactionID)
	if existingID, found := s.state.reconciliationByCommand[commandKey]; found {
		existing := s.state.reconciliationCases[existingID]
		if existing.CommandHash != value.CommandHash {
			return mobile.ReconciliationCase{}, sales.ErrIdempotencyConflict
		}
		return cloneReconciliationCase(existing), nil
	}
	s.state.reconciliationCases[value.ID] = cloneReconciliationCase(value)
	s.state.reconciliationByCommand[commandKey] = value.ID
	s.state.audits = append(s.state.audits, caseAudit)
	s.state.outbox = append(s.state.outbox, caseEvent)
	return cloneReconciliationCase(value), nil
}

func (s *Store) ListReconciliationCases(_ context.Context, scope tenancy.Scope, actorID string, status mobile.ReconciliationStatus, afterID string, limit int) ([]mobile.ReconciliationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "mobile.reconciliation.read")] {
		return nil, sales.ErrForbidden
	}
	items := make([]mobile.ReconciliationCase, 0)
	for _, value := range s.state.reconciliationCases {
		if value.Scope != scope || value.ID <= afterID || (status != "" && value.Status != status) {
			continue
		}
		items = append(items, cloneReconciliationCase(value))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *Store) ReconciliationCase(_ context.Context, scope tenancy.Scope, actorID, caseID string) (mobile.ReconciliationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "mobile.reconciliation.read")] {
		return mobile.ReconciliationCase{}, sales.ErrForbidden
	}
	value, found := s.state.reconciliationCases[caseID]
	if !found || value.Scope != scope {
		return mobile.ReconciliationCase{}, sales.ErrNotFound
	}
	return cloneReconciliationCase(value), nil
}

func (s *Store) ResolveReconciliationCase(_ context.Context, scope tenancy.Scope, actorID string, resolution mobile.ReconciliationResolution, resolutionAudit audit.Event, resolutionEvent outbox.Event) (mobile.ReconciliationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "mobile.reconciliation.resolve")] {
		return mobile.ReconciliationCase{}, sales.ErrForbidden
	}
	value, found := s.state.reconciliationCases[resolution.CaseID]
	if !found || value.Scope != scope {
		return mobile.ReconciliationCase{}, sales.ErrNotFound
	}
	idempotencyKey := companyKey(scope.TenantID, scope.CompanyID) + "\x00" + resolution.IdempotencyKey
	if existingCaseID, used := s.state.reconciliationIdempotency[idempotencyKey]; used {
		existing := s.state.reconciliationCases[existingCaseID]
		if existingCaseID != resolution.CaseID || existing.Resolution == nil || existing.Resolution.RequestHash != resolution.RequestHash {
			return mobile.ReconciliationCase{}, sales.ErrIdempotencyConflict
		}
		return cloneReconciliationCase(existing), nil
	}
	if value.Resolution != nil {
		return mobile.ReconciliationCase{}, mobile.ErrReconciliationResolved
	}
	copyResolution := resolution
	value.Status = mobile.ReconciliationResolved
	value.Resolution = &copyResolution
	s.state.reconciliationCases[value.ID] = value
	s.state.reconciliationIdempotency[idempotencyKey] = value.ID
	s.state.audits = append(s.state.audits, resolutionAudit)
	s.state.outbox = append(s.state.outbox, resolutionEvent)
	return cloneReconciliationCase(value), nil
}

func reconciliationCommandKey(scope tenancy.Scope, deviceID, clientTransactionID string) string {
	return scopeKey(scope) + "\x00" + deviceID + "\x00" + clientTransactionID
}
