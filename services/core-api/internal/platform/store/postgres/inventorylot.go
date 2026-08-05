package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventorycontrol"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/jackc/pgx/v5"
)

func (t *transaction) activeInventoryControl(ctx context.Context, scopeTenant, company, branch, warehouse, product string) (inventorycontrol.CostMethod, bool, error) {
	var method inventorycontrol.CostMethod
	var lot bool
	err := t.tx.QueryRow(ctx, `SELECT cost_method,lot_controlled FROM inventory_policies WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5 AND status='ACTIVE'`, scopeTenant, company, branch, warehouse, product).Scan(&method, &lot)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return method, lot, normalizeError(err)
}
func (t *transaction) applyGoodsReceiptInventoryControls(ctx context.Context, document operations.Document, line operations.DocumentLine, movementID string, product catalog.Product, at time.Time) error {
	method, lotControlled, e := t.activeInventoryControl(ctx, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID, line.ProductID)
	if e != nil {
		return e
	}
	if method == "" {
		return nil
	}
	if lotControlled {
		rows, e := t.tx.Query(ctx, `SELECT l.product_id::text,l.lot_number,l.quantity,l.manufactured_at,l.expires_at FROM inventory_lot_registrations r JOIN inventory_lot_registration_lines l ON l.tenant_id=r.tenant_id AND l.company_id=r.company_id AND l.registration_id=r.id WHERE r.tenant_id=$1 AND r.company_id=$2 AND r.branch_id=$3 AND r.warehouse_id=$4 AND r.goods_receipt_id=$5 AND l.product_id=$6 ORDER BY l.lot_number`, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID, document.ID, line.ProductID)
		if e != nil {
			return normalizeError(e)
		}
		type receiptLot struct {
			productID, number     string
			quantity              int64
			manufactured, expires *time.Time
		}
		lots := []receiptLot{}
		var total int64
		for rows.Next() {
			var lot receiptLot
			if e = rows.Scan(&lot.productID, &lot.number, &lot.quantity, &lot.manufactured, &lot.expires); e != nil {
				rows.Close()
				return normalizeError(e)
			}
			if lot.expires == nil || !lot.expires.After(at) {
				rows.Close()
				return inventorycontrol.ErrLotAllocation
			}
			lots = append(lots, lot)
			total += lot.quantity
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return normalizeError(e)
		}
		rows.Close()
		if total != line.Quantity {
			return inventorycontrol.ErrLotAllocation
		}
		for _, lot := range lots {
			var lotID string
			e = t.tx.QueryRow(ctx, `INSERT INTO inventory_lots(tenant_id,company_id,branch_id,warehouse_id,product_id,lot_number,manufactured_at,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(tenant_id,company_id,branch_id,warehouse_id,product_id,lot_number) DO UPDATE SET lot_number=EXCLUDED.lot_number WHERE inventory_lots.manufactured_at IS NOT DISTINCT FROM EXCLUDED.manufactured_at AND inventory_lots.expires_at IS NOT DISTINCT FROM EXCLUDED.expires_at RETURNING id::text`, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID, lot.productID, lot.number, lot.manufactured, lot.expires, at).Scan(&lotID)
			if e != nil {
				return inventorycontrol.ErrLotAllocation
			}
			if _, e = t.tx.Exec(ctx, `INSERT INTO inventory_lot_ledger(tenant_id,company_id,branch_id,warehouse_id,lot_id,product_id,stock_movement_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,'GOODS_RECEIPT',$8,$9,$10)`, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID, lotID, lot.productID, movementID, document.ID, lot.quantity, at); e != nil {
				return normalizeError(e)
			}
		}
	}
	var quantityAfter int64
	if e = t.tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM inventory_stock_ledger WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5`, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID, line.ProductID).Scan(&quantityAfter); e != nil {
		return normalizeError(e)
	}
	before := quantityAfter - line.Quantity
	afterCost := product.StandardCostMinor
	if method == inventorycontrol.MovingAverage {
		if e = t.tx.QueryRow(ctx, `SELECT round(($1::numeric*$2::numeric+$3::numeric*$4::numeric)/($1::numeric+$3::numeric))::bigint`, before, product.StandardCostMinor, line.Quantity, line.UnitPriceMinor).Scan(&afterCost); e != nil {
			return normalizeError(e)
		}
		if _, e = t.tx.Exec(ctx, `UPDATE products SET standard_cost_minor=$1 WHERE tenant_id=$2 AND company_id=$3 AND id=$4`, afterCost, document.Scope.TenantID, document.Scope.CompanyID, line.ProductID); e != nil {
			return normalizeError(e)
		}
	}
	_, e = t.tx.Exec(ctx, `INSERT INTO inventory_cost_history(tenant_id,company_id,branch_id,warehouse_id,product_id,source_id,cost_method,quantity_before,quantity_received,cost_before_minor,receipt_cost_minor,cost_after_minor,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, document.Scope.TenantID, document.Scope.CompanyID, document.Scope.BranchID, document.Scope.WarehouseID, line.ProductID, document.ID, method, before, line.Quantity, product.StandardCostMinor, line.UnitPriceMinor, afterCost, at)
	return normalizeError(e)
}

func (t *transaction) applyControlledLotMovement(ctx context.Context, v inventory.Movement) error {
	_, controlled, e := t.activeInventoryControl(ctx, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, v.ProductID)
	if e != nil || !controlled {
		return e
	}
	if v.SourceType == string(operations.GoodsReceipt) {
		return nil
	}
	if v.Quantity < 0 {
		if v.SourceType == "STOCK_TRANSFER_DISPATCH" {
			if e = t.requireControlledTransferDestination(ctx, v); e != nil {
				return e
			}
		}
		return t.consumeLotsFEFO(ctx, v)
	}
	switch v.SourceType {
	case string(sales.RecordReversal):
		return t.restoreReversedLots(ctx, v)
	case "STOCK_TRANSFER_RECEIPT":
		return t.receiveTransferredLots(ctx, v)
	default:
		return inventorycontrol.ErrLotAllocation
	}
}

func (t *transaction) requireControlledTransferDestination(ctx context.Context, v inventory.Movement) error {
	var branch, warehouse string
	if e := t.tx.QueryRow(ctx, `SELECT w.branch_id::text,d.destination_warehouse_id::text FROM operation_documents d JOIN warehouses w ON w.tenant_id=d.tenant_id AND w.company_id=d.company_id AND w.id=d.destination_warehouse_id WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.id=$3`, v.TenantID, v.CompanyID, v.SourceID).Scan(&branch, &warehouse); e != nil {
		return normalizeError(e)
	}
	_, controlled, e := t.activeInventoryControl(ctx, v.TenantID, v.CompanyID, branch, warehouse, v.ProductID)
	if e != nil {
		return e
	}
	if !controlled {
		return inventorycontrol.ErrLotReconciliation
	}
	return nil
}
func (t *transaction) consumeLotsFEFO(ctx context.Context, v inventory.Movement) error {
	rows, e := t.tx.Query(ctx, `SELECT l.id::text,COALESCE(sum(x.quantity),0)::bigint FROM inventory_lots l LEFT JOIN inventory_lot_ledger x ON x.tenant_id=l.tenant_id AND x.company_id=l.company_id AND x.lot_id=l.id WHERE l.tenant_id=$1 AND l.company_id=$2 AND l.branch_id=$3 AND l.warehouse_id=$4 AND l.product_id=$5 GROUP BY l.id,l.expires_at,l.lot_number HAVING COALESCE(sum(x.quantity),0)>0 ORDER BY l.expires_at NULLS LAST,l.lot_number`, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, v.ProductID)
	if e != nil {
		return normalizeError(e)
	}
	type lotAvailability struct {
		id       string
		quantity int64
	}
	lots := []lotAvailability{}
	for rows.Next() {
		var lot lotAvailability
		if e = rows.Scan(&lot.id, &lot.quantity); e != nil {
			rows.Close()
			return normalizeError(e)
		}
		lots = append(lots, lot)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return normalizeError(e)
	}
	rows.Close()
	remaining := -v.Quantity
	for _, lot := range lots {
		take := lot.quantity
		if take > remaining {
			take = remaining
		}
		if _, e = t.tx.Exec(ctx, `INSERT INTO inventory_lot_ledger(tenant_id,company_id,branch_id,warehouse_id,lot_id,product_id,stock_movement_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, lot.id, v.ProductID, v.ID, v.SourceType, v.SourceID, -take, v.OccurredAt); e != nil {
			return normalizeError(e)
		}
		remaining -= take
		if remaining == 0 {
			break
		}
	}
	if remaining != 0 {
		return inventorycontrol.ErrLotReconciliation
	}
	return nil
}
func (t *transaction) restoreReversedLots(ctx context.Context, v inventory.Movement) error {
	var original string
	if e := t.tx.QueryRow(ctx, `SELECT original_sale_id::text FROM sales WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND record_type='REVERSAL'`, v.TenantID, v.CompanyID, v.SourceID).Scan(&original); e != nil {
		return normalizeError(e)
	}
	rows, e := t.tx.Query(ctx, `SELECT lot_id::text,-quantity FROM inventory_lot_ledger WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND product_id=$5 AND source_type='SALE' AND source_id=$6`, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, v.ProductID, original)
	if e != nil {
		return normalizeError(e)
	}
	type reversalLot struct {
		id       string
		quantity int64
	}
	lots := []reversalLot{}
	var total int64
	for rows.Next() {
		var lot reversalLot
		if e = rows.Scan(&lot.id, &lot.quantity); e != nil {
			rows.Close()
			return normalizeError(e)
		}
		lots = append(lots, lot)
		total += lot.quantity
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return normalizeError(e)
	}
	rows.Close()
	if total != v.Quantity {
		return inventorycontrol.ErrLotReconciliation
	}
	for _, lot := range lots {
		if _, e = t.tx.Exec(ctx, `INSERT INTO inventory_lot_ledger(tenant_id,company_id,branch_id,warehouse_id,lot_id,product_id,stock_movement_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, lot.id, v.ProductID, v.ID, v.SourceType, v.SourceID, lot.quantity, v.OccurredAt); e != nil {
			return normalizeError(e)
		}
	}
	return nil
}
func (t *transaction) receiveTransferredLots(ctx context.Context, v inventory.Movement) error {
	rows, e := t.tx.Query(ctx, `SELECT l.lot_number,l.manufactured_at,l.expires_at,-sum(x.quantity)::bigint FROM inventory_lot_ledger x JOIN inventory_lots l ON l.tenant_id=x.tenant_id AND l.company_id=x.company_id AND l.id=x.lot_id WHERE x.tenant_id=$1 AND x.company_id=$2 AND x.product_id=$3 AND x.source_type='STOCK_TRANSFER_DISPATCH' AND x.source_id=$4 GROUP BY l.lot_number,l.manufactured_at,l.expires_at`, v.TenantID, v.CompanyID, v.ProductID, v.SourceID)
	if e != nil {
		return normalizeError(e)
	}
	type transferLot struct {
		number                string
		manufactured, expires *time.Time
		quantity              int64
	}
	lots := []transferLot{}
	var total int64
	for rows.Next() {
		var lot transferLot
		if e = rows.Scan(&lot.number, &lot.manufactured, &lot.expires, &lot.quantity); e != nil {
			rows.Close()
			return normalizeError(e)
		}
		lots = append(lots, lot)
		total += lot.quantity
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return normalizeError(e)
	}
	rows.Close()
	if total != v.Quantity {
		return inventorycontrol.ErrLotReconciliation
	}
	for _, transfer := range lots {
		var lot string
		if e = t.tx.QueryRow(ctx, `INSERT INTO inventory_lots(tenant_id,company_id,branch_id,warehouse_id,product_id,lot_number,manufactured_at,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(tenant_id,company_id,branch_id,warehouse_id,product_id,lot_number) DO UPDATE SET lot_number=EXCLUDED.lot_number RETURNING id::text`, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, v.ProductID, transfer.number, transfer.manufactured, transfer.expires, v.OccurredAt).Scan(&lot); e != nil {
			return normalizeError(e)
		}
		if _, e = t.tx.Exec(ctx, `INSERT INTO inventory_lot_ledger(tenant_id,company_id,branch_id,warehouse_id,lot_id,product_id,stock_movement_id,source_type,source_id,quantity,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.TenantID, v.CompanyID, v.BranchID, v.WarehouseID, lot, v.ProductID, v.ID, v.SourceType, v.SourceID, transfer.quantity, v.OccurredAt); e != nil {
			return normalizeError(e)
		}
	}
	return nil
}
