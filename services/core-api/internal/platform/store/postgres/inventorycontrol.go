package postgres

import (
	"context"
	"sort"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/inventorycontrol"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) InventoryControlWorkspace(ctx context.Context, scope tenancy.Scope, actor string, at time.Time) (inventorycontrol.Workspace, error) {
	w := inventorycontrol.Workspace{Policies: []inventorycontrol.Policy{}, Registrations: []inventorycontrol.LotRegistration{}, Lots: []inventorycontrol.LotBalance{}, Replenishments: []inventorycontrol.Replenishment{}, CostHistory: []inventorycontrol.CostHistory{}}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if e := inventoryControlAuth(ctx, tx, scope, actor, "inventory.planning.read"); e != nil {
			return e
		}
		rows, e := tx.tx.Query(ctx, `SELECT id::text,product_id::text,status,cost_method,lot_controlled,reorder_point,reorder_quantity,maximum_stock,safety_stock,lead_time_days,COALESCE(preferred_supplier_id::text,''),reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM inventory_policies WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 ORDER BY created_at DESC`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			v := inventorycontrol.Policy{Scope: scope}
			if e = rows.Scan(&v.ID, &v.ProductID, &v.Status, &v.CostMethod, &v.LotControlled, &v.ReorderPoint, &v.ReorderQuantity, &v.MaximumStock, &v.SafetyStock, &v.LeadTimeDays, &v.PreferredSupplierID, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			w.Policies = append(w.Policies, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT r.id::text,r.goods_receipt_id::text,r.reason,r.created_by::text,r.created_at,l.id::text,l.product_id::text,l.lot_number,l.quantity,l.manufactured_at,l.expires_at FROM inventory_lot_registrations r JOIN inventory_lot_registration_lines l ON l.tenant_id=r.tenant_id AND l.company_id=r.company_id AND l.registration_id=r.id WHERE r.tenant_id=$1 AND r.company_id=$2 AND r.branch_id=$3 AND r.warehouse_id=$4 ORDER BY r.created_at DESC,l.lot_number`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID)
		if e != nil {
			return normalizeError(e)
		}
		registrationIndex := map[string]int{}
		for rows.Next() {
			var registration inventorycontrol.LotRegistration
			var line inventorycontrol.LotRegistrationLine
			if e = rows.Scan(&registration.ID, &registration.GoodsReceiptID, &registration.Reason, &registration.CreatedBy, &registration.CreatedAt, &line.ID, &line.ProductID, &line.LotNumber, &line.Quantity, &line.ManufacturedAt, &line.ExpiresAt); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			if index, ok := registrationIndex[registration.ID]; ok {
				w.Registrations[index].Lines = append(w.Registrations[index].Lines, line)
			} else {
				registration.Scope = scope
				registration.Lines = []inventorycontrol.LotRegistrationLine{line}
				registrationIndex[registration.ID] = len(w.Registrations)
				w.Registrations = append(w.Registrations, registration)
			}
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT l.id::text,l.product_id::text,l.lot_number,l.manufactured_at,l.expires_at,COALESCE(sum(x.quantity),0)::bigint FROM inventory_lots l LEFT JOIN inventory_lot_ledger x ON x.tenant_id=l.tenant_id AND x.company_id=l.company_id AND x.lot_id=l.id WHERE l.tenant_id=$1 AND l.company_id=$2 AND l.branch_id=$3 AND l.warehouse_id=$4 GROUP BY l.id ORDER BY l.expires_at NULLS LAST,l.lot_number`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			var v inventorycontrol.LotBalance
			if e = rows.Scan(&v.LotID, &v.ProductID, &v.LotNumber, &v.ManufacturedAt, &v.ExpiresAt, &v.Quantity); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			w.Lots = append(w.Lots, v)
		}
		rows.Close()
		rows, e = tx.tx.Query(ctx, `SELECT id::text,product_id::text,source_id::text,cost_method,quantity_before,quantity_received,cost_before_minor,receipt_cost_minor,cost_after_minor,occurred_at FROM inventory_cost_history WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 ORDER BY occurred_at DESC`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			var v inventorycontrol.CostHistory
			if e = rows.Scan(&v.ID, &v.ProductID, &v.SourceID, &v.Method, &v.QuantityBefore, &v.QuantityReceived, &v.CostBeforeMinor, &v.ReceiptCostMinor, &v.CostAfterMinor, &v.OccurredAt); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			w.CostHistory = append(w.CostHistory, v)
		}
		rows.Close()
		for _, p := range w.Policies {
			if p.Status != inventorycontrol.PolicyActive {
				continue
			}
			var onHand, reserved, incoming int64
			if e = tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM inventory_stock_ledger WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, p.ProductID).Scan(&onHand); e != nil {
				return normalizeError(e)
			}
			if e = tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM inventory_reservation_ledger WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, p.ProductID).Scan(&reserved); e != nil {
				return normalizeError(e)
			}
			if e = tx.tx.QueryRow(ctx, `WITH received AS (SELECT gr.source_document_id AS purchase_order_id,gl.product_id,sum(gl.quantity)::bigint AS quantity FROM operation_documents gr JOIN operation_document_lines gl ON gl.tenant_id=gr.tenant_id AND gl.company_id=gr.company_id AND gl.document_id=gr.id WHERE gr.tenant_id=$1 AND gr.company_id=$2 AND gr.document_type='GOODS_RECEIPT' AND gr.status='POSTED' GROUP BY gr.source_document_id,gl.product_id) SELECT COALESCE(sum(GREATEST(l.quantity-COALESCE(r.quantity,0),0)),0)::bigint FROM operation_documents d JOIN operation_document_lines l ON l.tenant_id=d.tenant_id AND l.company_id=d.company_id AND l.document_id=d.id LEFT JOIN received r ON r.purchase_order_id=d.id AND r.product_id=l.product_id WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.branch_id=$3 AND d.warehouse_id=$4 AND d.document_type='PURCHASE_ORDER' AND d.status='APPROVED' AND l.product_id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, p.ProductID).Scan(&incoming); e != nil {
				return normalizeError(e)
			}
			available := onHand - reserved
			projected := available + incoming
			recommended := int64(0)
			if projected <= p.ReorderPoint {
				recommended = p.ReorderQuantity
				if gap := p.MaximumStock - projected; gap > recommended {
					recommended = gap
				}
			}
			w.Replenishments = append(w.Replenishments, inventorycontrol.Replenishment{ProductID: p.ProductID, OnHand: onHand, Reserved: reserved, Available: available, Incoming: incoming, Projected: projected, ReorderPoint: p.ReorderPoint, RecommendedQuantity: recommended, PreferredSupplierID: p.PreferredSupplierID, ActionRequired: recommended > 0})
		}
		sort.Slice(w.Replenishments, func(i, j int) bool { return w.Replenishments[i].ActionRequired && !w.Replenishments[j].ActionRequired })
		_ = at
		return nil
	})
	return w, err
}

func (s *Store) CreateInventoryPolicy(ctx context.Context, v inventorycontrol.Policy, idem, hash string) (inventorycontrol.Policy, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if e := inventoryControlAuth(ctx, tx, v.Scope, v.CreatedBy, "inventory.planning.manage"); e != nil {
			return e
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "inventory.policy.create.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO inventory_policies(id,tenant_id,company_id,branch_id,warehouse_id,product_id,status,cost_method,lot_controlled,reorder_point,reorder_quantity,maximum_stock,safety_stock,lead_time_days,preferred_supplier_id,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid,$16,$17,$18)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, v.ProductID, v.Status, v.CostMethod, v.LotControlled, v.ReorderPoint, v.ReorderQuantity, v.MaximumStock, v.SafetyStock, v.LeadTimeDays, v.PreferredSupplierID, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "inventory.policy_created", "inventory_policy", v.ID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "inventory.policy.create.v1", idem, v.ID)
	})
	return v, err
}
func (s *Store) TransitionInventoryPolicy(ctx context.Context, scope tenancy.Scope, actor, id string, to inventorycontrol.PolicyStatus, reason, idem, hash string, at time.Time) (inventorycontrol.Policy, error) {
	v := inventorycontrol.Policy{ID: id, Scope: scope}
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		perm := "inventory.planning.manage"
		if to == inventorycontrol.PolicyActive || to == inventorycontrol.PolicyRejected {
			perm = "inventory.planning.approve"
		}
		if e := inventoryControlAuth(ctx, tx, scope, actor, perm); e != nil {
			return e
		}
		op := "inventory.policy.transition." + string(to) + ".v1"
		acq, _, e := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.Status = to
			return nil
		}
		if e = tx.tx.QueryRow(ctx, `SELECT product_id::text,status,cost_method,lot_controlled,reorder_point,reorder_quantity,maximum_stock,safety_stock,lead_time_days,COALESCE(preferred_supplier_id::text,''),reason,created_by::text,created_at,COALESCE(approved_by::text,'') FROM inventory_policies WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5 FOR UPDATE`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, id).Scan(&v.ProductID, &v.Status, &v.CostMethod, &v.LotControlled, &v.ReorderPoint, &v.ReorderQuantity, &v.MaximumStock, &v.SafetyStock, &v.LeadTimeDays, &v.PreferredSupplierID, &v.Reason, &v.CreatedBy, &v.CreatedAt, &v.ApprovedBy); e != nil {
			return normalizeError(e)
		}
		from := v.Status
		valid := from == inventorycontrol.PolicyDraft && to == inventorycontrol.PolicySubmitted || from == inventorycontrol.PolicySubmitted && (to == inventorycontrol.PolicyActive || to == inventorycontrol.PolicyRejected)
		if !valid {
			return inventorycontrol.ErrInvalidTransition
		}
		if (to == inventorycontrol.PolicyActive || to == inventorycontrol.PolicyRejected) && v.CreatedBy == actor {
			return inventorycontrol.ErrSeparationOfDuties
		}
		if to == inventorycontrol.PolicyActive {
			if v.LotControlled {
				var stock, lots int64
				if e = tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM inventory_stock_ledger WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, v.ProductID).Scan(&stock); e != nil {
					return normalizeError(e)
				}
				if e = tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(x.quantity),0)::bigint FROM inventory_lot_ledger x WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, v.ProductID).Scan(&lots); e != nil {
					return normalizeError(e)
				}
				if stock != lots {
					return inventorycontrol.ErrLotReconciliation
				}
			}
			_, e = tx.tx.Exec(ctx, `UPDATE inventory_policies SET status='REJECTED' WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5 AND status='ACTIVE'`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, v.ProductID)
			if e != nil {
				return normalizeError(e)
			}
		}
		_, e = tx.tx.Exec(ctx, `UPDATE inventory_policies SET status=$1,approved_by=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $2::uuid ELSE approved_by END,approved_at=CASE WHEN $1 IN('ACTIVE','REJECTED') THEN $3 ELSE approved_at END WHERE id=$4`, to, actor, at, id)
		if e != nil {
			return normalizeError(e)
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO inventory_policy_transitions(tenant_id,company_id,policy_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if e != nil {
			return normalizeError(e)
		}
		if e = tx.financialEvidence(ctx, scope, actor, "inventory.policy_"+string(to), "inventory_policy", id, id, at); e != nil {
			return e
		}
		if e = tx.CompleteIdempotency(ctx, scope, op, idem, id); e != nil {
			return e
		}
		v.Status = to
		if to == inventorycontrol.PolicyActive {
			v.ApprovedBy = actor
		}
		return nil
	})
	return v, err
}
func (s *Store) RegisterReceiptLots(ctx context.Context, v inventorycontrol.LotRegistration, idem, hash string) (inventorycontrol.LotRegistration, error) {
	err := s.WithTransaction(ctx, func(c sales.Transaction) error {
		tx := c.(*transaction)
		if e := inventoryControlAuth(ctx, tx, v.Scope, v.CreatedBy, "inventory.lots.manage"); e != nil {
			return e
		}
		acq, id, e := tx.ClaimIdempotency(ctx, v.Scope, "inventory.lots.register.v1", idem, hash)
		if e != nil {
			return e
		}
		if !acq {
			v.ID = id
			return nil
		}
		var kind, status string
		if e = tx.tx.QueryRow(ctx, `SELECT document_type,status FROM operation_documents WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, v.GoodsReceiptID).Scan(&kind, &status); e != nil {
			return normalizeError(e)
		}
		if kind != "GOODS_RECEIPT" || status != "APPROVED" {
			return inventorycontrol.ErrLotAllocation
		}
		receipt := map[string]int64{}
		rows, e := tx.tx.Query(ctx, `SELECT product_id::text,quantity FROM operation_document_lines WHERE tenant_id=$1 AND company_id=$2 AND document_id=$3`, v.Scope.TenantID, v.Scope.CompanyID, v.GoodsReceiptID)
		if e != nil {
			return normalizeError(e)
		}
		for rows.Next() {
			var p string
			var q int64
			if e = rows.Scan(&p, &q); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			receipt[p] = q
		}
		rows.Close()
		allocated := map[string]int64{}
		for _, l := range v.Lines {
			if _, ok := receipt[l.ProductID]; !ok {
				return inventorycontrol.ErrLotAllocation
			}
			allocated[l.ProductID] += l.Quantity
		}
		for product, qty := range receipt {
			var controlled bool
			if e = tx.tx.QueryRow(ctx, `SELECT COALESCE((SELECT lot_controlled FROM inventory_policies WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5 AND status='ACTIVE'),false)`, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, product).Scan(&controlled); e != nil {
				return normalizeError(e)
			}
			if controlled && allocated[product] != qty {
				return inventorycontrol.ErrLotAllocation
			}
		}
		_, e = tx.tx.Exec(ctx, `INSERT INTO inventory_lot_registrations(id,tenant_id,company_id,branch_id,warehouse_id,goods_receipt_id,reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.Scope.TenantID, v.Scope.CompanyID, v.Scope.BranchID, v.Scope.WarehouseID, v.GoodsReceiptID, v.Reason, v.CreatedBy, v.CreatedAt)
		if e != nil {
			return normalizeError(e)
		}
		for _, l := range v.Lines {
			if _, e = tx.tx.Exec(ctx, `INSERT INTO inventory_lot_registration_lines(id,tenant_id,company_id,registration_id,product_id,lot_number,quantity,manufactured_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, l.ID, v.Scope.TenantID, v.Scope.CompanyID, v.ID, l.ProductID, l.LotNumber, l.Quantity, l.ManufacturedAt, l.ExpiresAt); e != nil {
				return normalizeError(e)
			}
		}
		if e = tx.financialEvidence(ctx, v.Scope, v.CreatedBy, "inventory.receipt_lots_registered", "goods_receipt", v.GoodsReceiptID, v.ID, v.CreatedAt); e != nil {
			return e
		}
		return tx.CompleteIdempotency(ctx, v.Scope, "inventory.lots.register.v1", idem, v.ID)
	})
	return v, err
}
func inventoryControlAuth(ctx context.Context, tx *transaction, scope tenancy.Scope, actor, permission string) error {
	ok, e := tx.Authorize(ctx, scope, actor, permission)
	if e != nil {
		return e
	}
	if !ok {
		return sales.ErrForbidden
	}
	return nil
}
