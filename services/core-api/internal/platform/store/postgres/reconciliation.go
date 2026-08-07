package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) RecordReconciliationCase(ctx context.Context, value mobile.ReconciliationCase, caseAudit audit.Event, caseEvent outbox.Event) (mobile.ReconciliationCase, error) {
	var result mobile.ReconciliationCase
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, value.Scope, value.CreatedBy, "mobile.sales.sync")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		device, err := tx.MobileDevice(ctx, value.Scope, value.CreatedBy, value.DeviceID)
		if err != nil {
			return err
		}
		if device.Status != devices.StatusActive {
			return devices.ErrNotActive
		}
		command, err := tx.tx.Exec(ctx, `
			INSERT INTO mobile_reconciliation_cases (
				id, tenant_id, company_id, branch_id, warehouse_id, device_id,
				client_transaction_id, client_timestamp, app_version,
				master_data_version, price_version, catalog_snapshot_token,
				failure_code, command, command_hash, created_by, correlation_id, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
			ON CONFLICT (tenant_id, company_id, branch_id, warehouse_id, device_id, client_transaction_id)
			DO NOTHING`, value.ID, value.Scope.TenantID, value.Scope.CompanyID,
			value.Scope.BranchID, value.Scope.WarehouseID, value.DeviceID,
			value.ClientTransactionID, value.ClientTimestamp, value.AppVersion,
			value.MasterDataVersion, value.PriceVersion, value.CatalogSnapshotToken,
			value.FailureCode, []byte(value.Command), value.CommandHash, value.CreatedBy,
			value.CorrelationID, value.CreatedAt)
		if err != nil {
			return normalizeError(err)
		}
		if command.RowsAffected() == 0 {
			var existingHash string
			if err := tx.tx.QueryRow(ctx, `
				SELECT command_hash FROM mobile_reconciliation_cases
				WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4
				  AND device_id=$5 AND client_transaction_id=$6`, value.Scope.TenantID,
				value.Scope.CompanyID, value.Scope.BranchID, value.Scope.WarehouseID,
				value.DeviceID, value.ClientTransactionID).Scan(&existingHash); err != nil {
				return normalizeError(err)
			}
			if existingHash != value.CommandHash {
				return sales.ErrIdempotencyConflict
			}
		} else {
			if err := tx.AppendAuditEvent(ctx, caseAudit); err != nil {
				return err
			}
			if err := tx.AppendOutboxEvent(ctx, caseEvent); err != nil {
				return err
			}
		}
		result, err = tx.reconciliationCaseByDeviceCommand(ctx, value.Scope, value.DeviceID, value.ClientTransactionID)
		return err
	})
	return result, err
}

func (s *Store) ListReconciliationCases(ctx context.Context, scope tenancy.Scope, actorID string, status mobile.ReconciliationStatus, afterID string, limit int) ([]mobile.ReconciliationCase, error) {
	result := make([]mobile.ReconciliationCase, 0)
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "mobile.reconciliation.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, reconciliationSelect+`
			WHERE c.tenant_id=$1 AND c.company_id=$2 AND c.branch_id=$3 AND c.warehouse_id=$4
			  AND ($5='' OR (CASE WHEN r.id IS NULL THEN 'OPEN' ELSE 'RESOLVED' END)=$5)
			  AND ($6='' OR c.id > NULLIF($6, '')::uuid)
			ORDER BY c.id LIMIT $7`, scope.TenantID, scope.CompanyID, scope.BranchID,
			scope.WarehouseID, status, afterID, limit)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			value, err := scanReconciliationCase(rows)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return normalizeError(rows.Err())
	})
	return result, err
}

func (s *Store) ReconciliationCase(ctx context.Context, scope tenancy.Scope, actorID, caseID string) (mobile.ReconciliationCase, error) {
	var result mobile.ReconciliationCase
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "mobile.reconciliation.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		result, err = tx.reconciliationCaseByID(ctx, scope, caseID)
		return err
	})
	return result, err
}

func (s *Store) ResolveReconciliationCase(ctx context.Context, scope tenancy.Scope, actorID string, resolution mobile.ReconciliationResolution, resolutionAudit audit.Event, resolutionEvent outbox.Event) (mobile.ReconciliationCase, error) {
	var result mobile.ReconciliationCase
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "mobile.reconciliation.resolve")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		if _, err := tx.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "mobile-reconciliation:"+scope.TenantID+":"+resolution.CaseID); err != nil {
			return normalizeError(err)
		}
		if _, err := tx.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "mobile-reconciliation-idempotency:"+scope.TenantID+":"+scope.CompanyID+":"+resolution.IdempotencyKey); err != nil {
			return normalizeError(err)
		}
		var idempotentCaseID, idempotentHash string
		err = tx.tx.QueryRow(ctx, `
			SELECT case_id, request_hash FROM mobile_reconciliation_resolutions
			WHERE tenant_id=$1 AND company_id=$2 AND idempotency_key=$3`,
			scope.TenantID, scope.CompanyID, resolution.IdempotencyKey).Scan(&idempotentCaseID, &idempotentHash)
		if err == nil {
			if idempotentCaseID != resolution.CaseID || idempotentHash != resolution.RequestHash {
				return sales.ErrIdempotencyConflict
			}
			result, err = tx.reconciliationCaseByID(ctx, scope, resolution.CaseID)
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return normalizeError(err)
		}
		var existingKey, existingHash string
		err = tx.tx.QueryRow(ctx, `
			SELECT idempotency_key, request_hash FROM mobile_reconciliation_resolutions
			WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4 AND case_id=$5`,
			scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, resolution.CaseID).Scan(&existingKey, &existingHash)
		if err == nil {
			if existingKey == resolution.IdempotencyKey && existingHash != resolution.RequestHash {
				return sales.ErrIdempotencyConflict
			}
			return mobile.ErrReconciliationResolved
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return normalizeError(err)
		}
		if _, err := tx.reconciliationCaseByID(ctx, scope, resolution.CaseID); err != nil {
			return err
		}
		_, err = tx.tx.Exec(ctx, `
			INSERT INTO mobile_reconciliation_resolutions (
				id, tenant_id, company_id, branch_id, warehouse_id, case_id, action,
				reason, external_reference, resolved_by, correlation_id, resolved_at,
				idempotency_key, request_hash
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, resolution.ID,
			scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID,
			resolution.CaseID, resolution.Action, resolution.Reason,
			nullableText(resolution.ExternalReference), resolution.ResolvedBy,
			resolution.CorrelationID, resolution.ResolvedAt, resolution.IdempotencyKey,
			resolution.RequestHash)
		if err != nil {
			return normalizeError(err)
		}
		if err := tx.AppendAuditEvent(ctx, resolutionAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, resolutionEvent); err != nil {
			return err
		}
		result, err = tx.reconciliationCaseByID(ctx, scope, resolution.CaseID)
		return err
	})
	return result, err
}

const reconciliationSelect = `
	SELECT c.id, c.tenant_id, c.company_id, c.branch_id, c.warehouse_id,
	       CASE WHEN r.id IS NULL THEN 'OPEN' ELSE 'RESOLVED' END,
	       c.device_id, c.client_transaction_id, c.client_timestamp, c.app_version,
	       c.master_data_version, c.price_version, c.catalog_snapshot_token,
	       c.failure_code, c.command, c.command_hash, c.created_by,
	       c.correlation_id, c.created_at,
	       r.id, r.action, r.reason, r.external_reference, r.resolved_by,
	       r.correlation_id, r.resolved_at, r.idempotency_key, r.request_hash
	FROM mobile_reconciliation_cases c
	LEFT JOIN mobile_reconciliation_resolutions r
	  ON r.tenant_id=c.tenant_id AND r.company_id=c.company_id
	 AND r.branch_id=c.branch_id AND r.warehouse_id=c.warehouse_id AND r.case_id=c.id`

func (t *transaction) reconciliationCaseByID(ctx context.Context, scope tenancy.Scope, caseID string) (mobile.ReconciliationCase, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return mobile.ReconciliationCase{}, err
	}
	return scanReconciliationCase(t.tx.QueryRow(ctx, reconciliationSelect+`
		WHERE c.tenant_id=$1 AND c.company_id=$2 AND c.branch_id=$3 AND c.warehouse_id=$4 AND c.id=$5`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, caseID))
}

func (t *transaction) reconciliationCaseByDeviceCommand(ctx context.Context, scope tenancy.Scope, deviceID, clientTransactionID string) (mobile.ReconciliationCase, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return mobile.ReconciliationCase{}, err
	}
	return scanReconciliationCase(t.tx.QueryRow(ctx, reconciliationSelect+`
		WHERE c.tenant_id=$1 AND c.company_id=$2 AND c.branch_id=$3 AND c.warehouse_id=$4
		  AND c.device_id=$5 AND c.client_transaction_id=$6`, scope.TenantID, scope.CompanyID,
		scope.BranchID, scope.WarehouseID, deviceID, clientTransactionID))
}

type reconciliationRow interface {
	Scan(dest ...any) error
}

func scanReconciliationCase(row reconciliationRow) (mobile.ReconciliationCase, error) {
	var value mobile.ReconciliationCase
	var resolutionID pgtype.UUID
	var action, reason, externalReference, resolvedBy, resolutionCorrelation, idempotencyKey, requestHash pgtype.Text
	var resolvedAt pgtype.Timestamptz
	err := row.Scan(&value.ID, &value.Scope.TenantID, &value.Scope.CompanyID,
		&value.Scope.BranchID, &value.Scope.WarehouseID, &value.Status,
		&value.DeviceID, &value.ClientTransactionID, &value.ClientTimestamp,
		&value.AppVersion, &value.MasterDataVersion, &value.PriceVersion,
		&value.CatalogSnapshotToken, &value.FailureCode, &value.Command,
		&value.CommandHash, &value.CreatedBy, &value.CorrelationID, &value.CreatedAt,
		&resolutionID, &action, &reason, &externalReference, &resolvedBy,
		&resolutionCorrelation, &resolvedAt, &idempotencyKey, &requestHash)
	if err != nil {
		return mobile.ReconciliationCase{}, normalizeError(err)
	}
	if resolutionID.Valid {
		value.Resolution = &mobile.ReconciliationResolution{
			ID: resolutionID.String(), CaseID: value.ID, Action: mobile.ResolutionAction(action.String),
			Reason: reason.String, ExternalReference: externalReference.String,
			ResolvedBy: resolvedBy.String, CorrelationID: resolutionCorrelation.String,
			ResolvedAt: resolvedAt.Time, IdempotencyKey: idempotencyKey.String,
			RequestHash: requestHash.String,
		}
	}
	return value, nil
}
