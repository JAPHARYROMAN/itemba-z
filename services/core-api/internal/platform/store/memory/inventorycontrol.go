package memory

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/inventorycontrol"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) InventoryControlWorkspace(_ context.Context, scope tenancy.Scope, actor string, _ time.Time) (inventorycontrol.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actor, "inventory.planning.read")] {
		return inventorycontrol.Workspace{}, sales.ErrForbidden
	}
	w := inventorycontrol.Workspace{Policies: []inventorycontrol.Policy{}, Registrations: []inventorycontrol.LotRegistration{}, Lots: []inventorycontrol.LotBalance{}, Replenishments: []inventorycontrol.Replenishment{}, CostHistory: []inventorycontrol.CostHistory{}}
	for _, v := range s.state.inventoryPolicies {
		if v.Scope == scope {
			w.Policies = append(w.Policies, v)
			if v.Status == inventorycontrol.PolicyActive {
				var onHand, reserved int64
				for _, m := range s.state.movements {
					if m.TenantID == scope.TenantID && m.CompanyID == scope.CompanyID && m.BranchID == scope.BranchID && m.WarehouseID == scope.WarehouseID && m.ProductID == v.ProductID {
						onHand += m.Quantity
					}
				}
				for _, m := range s.state.reservations {
					if m.TenantID == scope.TenantID && m.CompanyID == scope.CompanyID && m.BranchID == scope.BranchID && m.WarehouseID == scope.WarehouseID && m.ProductID == v.ProductID {
						reserved += m.Quantity
					}
				}
				available := onHand - reserved
				recommended := int64(0)
				if available <= v.ReorderPoint {
					recommended = v.ReorderQuantity
					if gap := v.MaximumStock - available; gap > recommended {
						recommended = gap
					}
				}
				w.Replenishments = append(w.Replenishments, inventorycontrol.Replenishment{ProductID: v.ProductID, OnHand: onHand, Reserved: reserved, Available: available, Projected: available, ReorderPoint: v.ReorderPoint, RecommendedQuantity: recommended, PreferredSupplierID: v.PreferredSupplierID, ActionRequired: recommended > 0})
			}
		}
	}
	for _, v := range s.state.lotRegistrations {
		if v.Scope == scope {
			w.Registrations = append(w.Registrations, v)
		}
	}
	for _, v := range s.state.lotBalances {
		w.Lots = append(w.Lots, v)
	}
	for _, v := range s.state.inventoryCostHistory {
		w.CostHistory = append(w.CostHistory, v)
	}
	sort.Slice(w.Replenishments, func(i, j int) bool { return w.Replenishments[i].ActionRequired && !w.Replenishments[j].ActionRequired })
	return w, nil
}
func (s *Store) CreateInventoryPolicy(_ context.Context, v inventorycontrol.Policy, idem, hash string) (inventorycontrol.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "inventory.planning.manage")] {
		return v, sales.ErrForbidden
	}
	if p, ok, e := s.memoryIdem(v.Scope, "inventory.policy.create.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.inventoryPolicies[commercialKey(v.Scope, p)], nil
	}
	if _, ok := s.state.products[commercialKey(v.Scope, v.ProductID)]; !ok {
		return v, sales.ErrNotFound
	}
	s.state.inventoryPolicies[commercialKey(v.Scope, v.ID)] = v
	s.putIdem(v.Scope, "inventory.policy.create.v1", idem, hash, v.ID)
	return v, nil
}
func (s *Store) TransitionInventoryPolicy(_ context.Context, scope tenancy.Scope, actor, id string, to inventorycontrol.PolicyStatus, _ string, idem, hash string, _ time.Time) (inventorycontrol.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	perm := "inventory.planning.manage"
	if to == inventorycontrol.PolicyActive || to == inventorycontrol.PolicyRejected {
		perm = "inventory.planning.approve"
	}
	if !s.state.permissions[permissionKey(scope, actor, perm)] {
		return inventorycontrol.Policy{}, sales.ErrForbidden
	}
	k := commercialKey(scope, id)
	v, ok := s.state.inventoryPolicies[k]
	if !ok {
		return v, sales.ErrNotFound
	}
	op := "inventory.policy.transition." + string(to) + ".v1"
	if _, hit, e := s.memoryIdem(scope, op, idem, hash); e != nil {
		return v, e
	} else if hit {
		return v, nil
	}
	valid := v.Status == inventorycontrol.PolicyDraft && to == inventorycontrol.PolicySubmitted || v.Status == inventorycontrol.PolicySubmitted && (to == inventorycontrol.PolicyActive || to == inventorycontrol.PolicyRejected)
	if !valid {
		return v, inventorycontrol.ErrInvalidTransition
	}
	if (to == inventorycontrol.PolicyActive || to == inventorycontrol.PolicyRejected) && v.CreatedBy == actor {
		return v, inventorycontrol.ErrSeparationOfDuties
	}
	if to == inventorycontrol.PolicyActive {
		if v.LotControlled {
			var stock, lots int64
			for _, m := range s.state.movements {
				if m.TenantID == scope.TenantID && m.CompanyID == scope.CompanyID && m.BranchID == scope.BranchID && m.WarehouseID == scope.WarehouseID && m.ProductID == v.ProductID {
					stock += m.Quantity
				}
			}
			for _, l := range s.state.lotBalances {
				if l.ProductID == v.ProductID {
					lots += l.Quantity
				}
			}
			if stock != lots {
				return v, inventorycontrol.ErrLotReconciliation
			}
		}
		for key, x := range s.state.inventoryPolicies {
			if key != k && x.Scope == scope && x.ProductID == v.ProductID && x.Status == inventorycontrol.PolicyActive {
				x.Status = inventorycontrol.PolicyRejected
				s.state.inventoryPolicies[key] = x
			}
		}
		v.ApprovedBy = actor
	}
	v.Status = to
	s.state.inventoryPolicies[k] = v
	s.putIdem(scope, op, idem, hash, id)
	return v, nil
}
func (s *Store) RegisterReceiptLots(_ context.Context, v inventorycontrol.LotRegistration, idem, hash string) (inventorycontrol.LotRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(v.Scope, v.CreatedBy, "inventory.lots.manage")] {
		return v, sales.ErrForbidden
	}
	if p, ok, e := s.memoryIdem(v.Scope, "inventory.lots.register.v1", idem, hash); e != nil {
		return v, e
	} else if ok {
		return s.state.lotRegistrations[commercialKey(v.Scope, p)], nil
	}
	doc, ok := s.state.operationDocuments[operationKey(v.Scope, v.GoodsReceiptID)]
	if !ok || doc.Type != operations.GoodsReceipt || doc.Status != operations.Approved {
		return v, inventorycontrol.ErrLotAllocation
	}
	receipt := map[string]int64{}
	for _, l := range doc.Lines {
		receipt[l.ProductID] = l.Quantity
	}
	allocated := map[string]int64{}
	for _, l := range v.Lines {
		if _, ok := receipt[l.ProductID]; !ok {
			return v, inventorycontrol.ErrLotAllocation
		}
		allocated[l.ProductID] += l.Quantity
	}
	for product, quantity := range receipt {
		controlled := false
		for _, p := range s.state.inventoryPolicies {
			if p.Scope == v.Scope && p.ProductID == product && p.Status == inventorycontrol.PolicyActive {
				controlled = p.LotControlled
			}
		}
		if controlled && allocated[product] != quantity {
			return v, inventorycontrol.ErrLotAllocation
		}
	}
	s.state.lotRegistrations[commercialKey(v.Scope, v.ID)] = v
	s.putIdem(v.Scope, "inventory.lots.register.v1", idem, hash, v.ID)
	return v, nil
}
