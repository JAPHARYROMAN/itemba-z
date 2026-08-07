package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	deviceStatusOperation     = "mobile.device.status.change.v1"
	deviceAllocationOperation = "mobile.device.allocation.change.v1"
)

func (s *Store) ListManagedDevices(ctx context.Context, scope tenancy.Scope, actorID, afterID string, limit int) ([]devices.Device, error) {
	var result []devices.Device
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "mobile.devices.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `
			SELECT id, actor_id FROM mobile_devices
			WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4
			  AND id > COALESCE(NULLIF($5, '')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
			ORDER BY id LIMIT $6`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, afterID, limit)
		if err != nil {
			return normalizeError(err)
		}
		var ids, actors []string
		for rows.Next() {
			var id, boundActor string
			if err := rows.Scan(&id, &boundActor); err != nil {
				rows.Close()
				return normalizeError(err)
			}
			ids, actors = append(ids, id), append(actors, boundActor)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return normalizeError(err)
		}
		for index, id := range ids {
			value, err := tx.MobileDevice(ctx, scope, actors[index], id)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return nil
	})
	return result, err
}

func (s *Store) ChangeManagedDeviceStatus(ctx context.Context, change mobile.DeviceStatusChange, changeAudit audit.Event, changeEvent outbox.Event) (devices.Device, error) {
	var result devices.Device
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, change.Scope, change.ChangedBy, "mobile.devices.manage")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		acquired, _, err := tx.ClaimIdempotency(ctx, change.Scope, deviceStatusOperation, change.IdempotencyKey, change.RequestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.managedDevice(ctx, change.Scope, change.DeviceID)
			return err
		}
		current, err := tx.managedDevice(ctx, change.Scope, change.DeviceID)
		if err != nil {
			return err
		}
		if current.Status == devices.StatusRevoked || (current.Status != devices.StatusActive && current.Status != devices.StatusSuspended) {
			return devices.ErrInvalidStatusTransition
		}
		if current.Status == change.Status {
			return devices.ErrStatusUnchanged
		}
		change.PreviousStatus = current.Status
		_, err = tx.tx.Exec(ctx, `
			UPDATE mobile_devices
			SET status=$6, authorization_epoch=CASE WHEN $6='SUSPENDED' THEN gen_random_uuid() ELSE authorization_epoch END,
			    offline_sales_valid_until=CASE WHEN $6='SUSPENDED'
			      THEN LEAST(offline_sales_valid_until, GREATEST(offline_sales_valid_from, $7))
			      ELSE offline_sales_valid_until END
			WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`,
			change.Scope.TenantID, change.Scope.CompanyID, change.Scope.BranchID, change.Scope.WarehouseID,
			change.DeviceID, change.Status, change.ChangedAt)
		if err != nil {
			return normalizeError(err)
		}
		payload, err := json.Marshal(change)
		if err != nil {
			return err
		}
		changeAudit.Data, changeEvent.Payload = payload, payload
		_, err = tx.tx.Exec(ctx, `
			INSERT INTO mobile_device_status_changes (
				id,tenant_id,company_id,branch_id,warehouse_id,device_id,previous_status,new_status,
				reason,changed_by,correlation_id,changed_at,idempotency_key,request_hash
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			change.ID, change.Scope.TenantID, change.Scope.CompanyID, change.Scope.BranchID, change.Scope.WarehouseID,
			change.DeviceID, change.PreviousStatus, change.Status, change.Reason, change.ChangedBy,
			change.CorrelationID, change.ChangedAt, change.IdempotencyKey, change.RequestHash)
		if err != nil {
			return normalizeError(err)
		}
		if err := tx.AppendAuditEvent(ctx, changeAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, changeEvent); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, change.Scope, deviceStatusOperation, change.IdempotencyKey, change.DeviceID); err != nil {
			return err
		}
		result, err = tx.managedDevice(ctx, change.Scope, change.DeviceID)
		return err
	})
	return result, err
}

func (s *Store) ChangeManagedDeviceAllocation(ctx context.Context, change mobile.DeviceAllocationChange, changeAudit audit.Event, changeEvent outbox.Event) (devices.Device, error) {
	var result devices.Device
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, change.Scope, change.ChangedBy, "mobile.devices.manage")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		acquired, _, err := tx.ClaimIdempotency(ctx, change.Scope, deviceAllocationOperation, change.IdempotencyKey, change.RequestHash)
		if err != nil {
			return err
		}
		if !acquired {
			result, err = tx.managedDevice(ctx, change.Scope, change.DeviceID)
			return err
		}
		if _, err := tx.managedDevice(ctx, change.Scope, change.DeviceID); err != nil {
			return err
		}
		if _, err := tx.Product(ctx, change.Scope, change.ProductID); err != nil {
			return err
		}
		allocationLock := strings.Join([]string{change.Scope.TenantID, change.Scope.CompanyID, change.Scope.BranchID, change.Scope.WarehouseID, "offline-allocation", change.ProductID}, ":")
		if _, err := tx.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, allocationLock); err != nil {
			return normalizeError(err)
		}
		available, err := tx.AvailableStock(ctx, change.Scope, change.ProductID)
		if err != nil {
			return err
		}
		if err := tx.tx.QueryRow(ctx, `
			SELECT COALESCE((SELECT allocated_quantity FROM mobile_device_stock_allocations
			  WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND device_id=$5 AND product_id=$6),0)::bigint,
			COALESCE((SELECT sum(sl.quantity) FROM sales s JOIN sale_lines sl
			  ON sl.tenant_id=s.tenant_id AND sl.company_id=s.company_id AND sl.sale_id=s.id
			  WHERE s.tenant_id=$1 AND s.company_id=$2 AND s.branch_id=$3 AND s.warehouse_id=$4
			    AND s.device_id=$5 AND s.offline AND s.record_type='SALE' AND s.status='POSTED' AND sl.product_id=$6),0)::bigint`,
			change.Scope.TenantID, change.Scope.CompanyID, change.Scope.BranchID, change.Scope.WarehouseID,
			change.DeviceID, change.ProductID).Scan(&change.PreviousQuantity, &change.ConsumedQuantity); err != nil {
			return normalizeError(err)
		}
		if change.AllocatedQuantity < change.ConsumedQuantity {
			return devices.ErrAllocationBelowConsumed
		}
		var otherReserved int64
		if err := tx.tx.QueryRow(ctx, `
			SELECT COALESCE(sum(GREATEST(a.allocated_quantity-COALESCE(consumed.quantity,0),0)),0)::bigint
			FROM mobile_device_stock_allocations a
			LEFT JOIN LATERAL (
			  SELECT sum(sl.quantity)::bigint AS quantity FROM sales s JOIN sale_lines sl
			    ON sl.tenant_id=s.tenant_id AND sl.company_id=s.company_id AND sl.sale_id=s.id
			  WHERE s.tenant_id=a.tenant_id AND s.company_id=a.company_id AND s.branch_id=a.branch_id
			    AND s.warehouse_id=a.warehouse_id AND s.device_id=a.device_id AND s.offline
			    AND s.record_type='SALE' AND s.status='POSTED' AND sl.product_id=a.product_id
			) consumed ON true
			WHERE a.tenant_id=$1 AND a.company_id=$2 AND a.branch_id=$3 AND a.warehouse_id=$4
			  AND a.product_id=$5 AND a.device_id<>$6`, change.Scope.TenantID, change.Scope.CompanyID,
			change.Scope.BranchID, change.Scope.WarehouseID, change.ProductID, change.DeviceID).Scan(&otherReserved); err != nil {
			return normalizeError(err)
		}
		if otherReserved+(change.AllocatedQuantity-change.ConsumedQuantity) > available {
			return devices.ErrAllocationOvercommitted
		}
		_, err = tx.tx.Exec(ctx, `
			INSERT INTO mobile_device_stock_allocations
			  (tenant_id,company_id,branch_id,warehouse_id,device_id,product_id,allocated_quantity,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (tenant_id,device_id,product_id) DO UPDATE
			SET allocated_quantity=EXCLUDED.allocated_quantity,updated_at=EXCLUDED.updated_at`,
			change.Scope.TenantID, change.Scope.CompanyID, change.Scope.BranchID, change.Scope.WarehouseID,
			change.DeviceID, change.ProductID, change.AllocatedQuantity, change.ChangedAt)
		if err != nil {
			return normalizeError(err)
		}
		payload, err := json.Marshal(change)
		if err != nil {
			return err
		}
		changeAudit.Data, changeEvent.Payload = payload, payload
		_, err = tx.tx.Exec(ctx, `
			INSERT INTO mobile_device_allocation_changes (
				id,tenant_id,company_id,branch_id,warehouse_id,device_id,product_id,
				previous_quantity,new_quantity,consumed_quantity,reason,changed_by,correlation_id,
				changed_at,idempotency_key,request_hash
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			change.ID, change.Scope.TenantID, change.Scope.CompanyID, change.Scope.BranchID,
			change.Scope.WarehouseID, change.DeviceID, change.ProductID, change.PreviousQuantity,
			change.AllocatedQuantity, change.ConsumedQuantity, change.Reason, change.ChangedBy,
			change.CorrelationID, change.ChangedAt, change.IdempotencyKey, change.RequestHash)
		if err != nil {
			return normalizeError(err)
		}
		if err := tx.AppendAuditEvent(ctx, changeAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, changeEvent); err != nil {
			return err
		}
		if err := tx.CompleteIdempotency(ctx, change.Scope, deviceAllocationOperation, change.IdempotencyKey, change.DeviceID); err != nil {
			return err
		}
		result, err = tx.managedDevice(ctx, change.Scope, change.DeviceID)
		return err
	})
	return result, err
}

func (t *transaction) managedDevice(ctx context.Context, scope tenancy.Scope, deviceID string) (devices.Device, error) {
	var boundActor string
	err := t.tx.QueryRow(ctx, `
		SELECT actor_id FROM mobile_devices
		WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND id=$5`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, deviceID).Scan(&boundActor)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.Device{}, devices.ErrNotEnrolled
	}
	if err != nil {
		return devices.Device{}, normalizeError(err)
	}
	return t.MobileDevice(ctx, scope, boundActor, deviceID)
}
