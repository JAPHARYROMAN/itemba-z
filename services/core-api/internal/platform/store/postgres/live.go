package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) EnrollDevice(ctx context.Context, value devices.Device, acknowledgement *devices.InstallAcknowledgement, enrollmentAudit audit.Event, enrollmentEvent outbox.Event) (devices.Device, error) {
	var result devices.Device
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, value.Scope, value.ActorID, "mobile.devices.enroll")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		versionLockKey := "master-data-version:" + value.Scope.TenantID + ":" + value.Scope.CompanyID
		if _, err := tx.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, versionLockKey); err != nil {
			return normalizeError(err)
		}
		var nextTaxTransition *time.Time
		if err := tx.tx.QueryRow(ctx, `
			SELECT company.master_data_version, company.price_version, company.catalog_snapshot_token, company.business_timezone,
			       (SELECT min(transition_at) FROM (
			            SELECT effective_from AS transition_at FROM tax_rules
			            WHERE tenant_id=$1 AND company_id=$2 AND effective_from > $3
			            UNION ALL
			            SELECT effective_to AS transition_at FROM tax_rules
			            WHERE tenant_id=$1 AND company_id=$2 AND effective_to > $3
			        ) transitions)
			FROM legal_companies company WHERE company.tenant_id = $1 AND company.id = $2`,
			value.Scope.TenantID, value.Scope.CompanyID, value.LastSeenAt).Scan(
			&value.AvailableMasterDataVersion, &value.AvailablePriceVersion, &value.AvailableCatalogSnapshotToken, &value.TimeZone, &nextTaxTransition); err != nil {
			return normalizeError(err)
		}
		leaseUntil := value.LastSeenAt.Add(devices.OfflineSalesLeaseDuration)
		if nextTaxTransition != nil && nextTaxTransition.Before(leaseUntil) {
			leaseUntil = nextTaxTransition.UTC()
		}
		if err := devices.ValidateWireSafe(value); err != nil {
			return err
		}
		if acknowledgement != nil && (acknowledgement.MasterDataVersion != value.AvailableMasterDataVersion || acknowledgement.PriceVersion != value.AvailablePriceVersion || acknowledgement.CatalogSnapshotToken != value.AvailableCatalogSnapshotToken) {
			return devices.ErrStaleMasterData
		}
		existing, err := tx.MobileDevice(ctx, value.Scope, value.ActorID, value.ID)
		if err == nil {
			existing.AvailableMasterDataVersion, existing.AvailablePriceVersion = value.AvailableMasterDataVersion, value.AvailablePriceVersion
			existing.AvailableCatalogSnapshotToken = value.AvailableCatalogSnapshotToken
			existing.TimeZone = value.TimeZone
			if acknowledgement == nil {
				err = tx.tx.QueryRow(ctx, `
					UPDATE mobile_devices
					SET device_name = $3, last_seen_at = GREATEST(last_seen_at, $4)
					WHERE tenant_id = $1 AND id = $2
					RETURNING last_seen_at`, value.Scope.TenantID, value.ID, value.Name, value.LastSeenAt).Scan(&existing.LastSeenAt)
				if err != nil {
					return normalizeError(err)
				}
				existing.Name = value.Name
				result = existing
				return nil
			}

			sameInstallation := existing.AppVersion == value.AppVersion &&
				existing.MasterDataVersion == acknowledgement.MasterDataVersion &&
				existing.PriceVersion == acknowledgement.PriceVersion
			sameInstallation = sameInstallation && existing.CatalogSnapshotToken == acknowledgement.CatalogSnapshotToken
			liveAuthorization := !existing.OfflineEnabled ||
				(!existing.OfflineSalesValidFrom.After(value.LastSeenAt) && existing.OfflineSalesValidUntil.After(value.LastSeenAt))
			if sameInstallation && liveAuthorization {
				result = existing
				return nil
			}

			existing.Name = value.Name
			existing.AppVersion = value.AppVersion
			existing.MasterDataVersion = acknowledgement.MasterDataVersion
			existing.PriceVersion = acknowledgement.PriceVersion
			existing.CatalogSnapshotToken = acknowledgement.CatalogSnapshotToken
			existing.OfflineSalesValidFrom = value.LastSeenAt
			existing.OfflineSalesValidUntil = value.LastSeenAt
			if existing.OfflineEnabled {
				existing.OfflineSalesValidUntil = leaseUntil
			}
			err = tx.tx.QueryRow(ctx, `
				UPDATE mobile_devices
				SET device_name = $3, app_version = $4, last_seen_at = GREATEST(last_seen_at, $5),
				    master_data_version = $6, price_version = $7, catalog_snapshot_token = $8,
				    offline_sales_valid_from = $9, offline_sales_valid_until = $10
				WHERE tenant_id = $1 AND id = $2
				RETURNING last_seen_at`, value.Scope.TenantID, value.ID, existing.Name, existing.AppVersion,
				value.LastSeenAt, existing.MasterDataVersion, existing.PriceVersion, existing.CatalogSnapshotToken,
				existing.OfflineSalesValidFrom, existing.OfflineSalesValidUntil).Scan(&existing.LastSeenAt)
			if err != nil {
				return normalizeError(err)
			}
			payload, err := json.Marshal(existing)
			if err != nil {
				return err
			}
			enrollmentAudit.Action = "mobile.device.installation_acknowledged"
			enrollmentAudit.Data = payload
			enrollmentEvent.EventType = "mobile.device.installation_acknowledged"
			enrollmentEvent.Payload = payload
			if err := tx.AppendAuditEvent(ctx, enrollmentAudit); err != nil {
				return err
			}
			if err := tx.AppendOutboxEvent(ctx, enrollmentEvent); err != nil {
				return err
			}
			if existing.OfflineEnabled {
				if err := tx.appendOfflineLease(ctx, devices.OfflineLease{
					Scope: existing.Scope, DeviceID: existing.ID, AppVersion: existing.AppVersion,
					MasterDataVersion: existing.MasterDataVersion, PriceVersion: existing.PriceVersion,
					CatalogSnapshotToken: existing.CatalogSnapshotToken,
					ValidFrom:            existing.OfflineSalesValidFrom, ValidUntil: existing.OfflineSalesValidUntil,
				}); err != nil {
					return err
				}
			}
			result = existing
			return nil
		}
		if !errors.Is(err, devices.ErrNotEnrolled) {
			return err
		}
		if acknowledgement != nil {
			value.MasterDataVersion = acknowledgement.MasterDataVersion
			value.PriceVersion = acknowledgement.PriceVersion
			value.CatalogSnapshotToken = acknowledgement.CatalogSnapshotToken
			value.OfflineSalesValidFrom = value.LastSeenAt
			value.OfflineSalesValidUntil = value.LastSeenAt
			if value.OfflineEnabled {
				value.OfflineSalesValidUntil = leaseUntil
			}
		} else {
			value.OfflineSalesValidFrom = value.LastSeenAt
			value.OfflineSalesValidUntil = value.LastSeenAt
		}
		value.StockAllocations = make([]devices.StockAllocation, 0)
		value.OfflineRemainingDailyMinor = value.OfflineDailyLimitMinor
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		enrollmentAudit.Data = payload
		enrollmentEvent.Payload = payload
		_, err = tx.tx.Exec(ctx, `
			INSERT INTO mobile_devices (
				id, tenant_id, company_id, branch_id, warehouse_id, actor_id, status,
				device_name, app_version, master_data_version, price_version, catalog_snapshot_token, offline_enabled,
				offline_transaction_limit_minor, offline_daily_limit_minor,
				offline_sales_valid_from, offline_sales_valid_until, enrolled_at, last_seen_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			value.ID, value.Scope.TenantID, value.Scope.CompanyID, value.Scope.BranchID, value.Scope.WarehouseID,
			value.ActorID, value.Status, value.Name, value.AppVersion, value.MasterDataVersion, value.PriceVersion, value.CatalogSnapshotToken,
			value.OfflineEnabled, value.OfflineTransactionLimitMinor, value.OfflineDailyLimitMinor,
			value.OfflineSalesValidFrom, value.OfflineSalesValidUntil, value.EnrolledAt, value.LastSeenAt)
		if err != nil {
			return normalizeError(err)
		}
		if acknowledgement != nil && value.OfflineEnabled {
			if err := tx.appendOfflineLease(ctx, devices.OfflineLease{
				Scope: value.Scope, DeviceID: value.ID, AppVersion: value.AppVersion,
				MasterDataVersion: value.MasterDataVersion, PriceVersion: value.PriceVersion,
				CatalogSnapshotToken: value.CatalogSnapshotToken,
				ValidFrom:            value.OfflineSalesValidFrom, ValidUntil: value.OfflineSalesValidUntil,
			}); err != nil {
				return err
			}
		}
		if err := tx.AppendAuditEvent(ctx, enrollmentAudit); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, enrollmentEvent); err != nil {
			return err
		}
		result = value
		return nil
	})
	return result, err
}

func (t *transaction) appendOfflineLease(ctx context.Context, lease devices.OfflineLease) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO mobile_device_offline_leases (
			tenant_id, company_id, branch_id, warehouse_id, device_id, app_version,
			master_data_version, price_version, catalog_snapshot_token, valid_from, valid_until
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT DO NOTHING`, lease.Scope.TenantID, lease.Scope.CompanyID, lease.Scope.BranchID,
		lease.Scope.WarehouseID, lease.DeviceID, lease.AppVersion, lease.MasterDataVersion,
		lease.PriceVersion, lease.CatalogSnapshotToken, lease.ValidFrom, lease.ValidUntil)
	return normalizeError(err)
}

func (s *Store) WorkingContext(ctx context.Context, scope tenancy.Scope, actorID string) (readmodel.WorkingContext, error) {
	var result readmodel.WorkingContext
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		if err := tx.ensureScope(ctx, scope); err != nil {
			return err
		}
		var permissions []string
		err := tx.tx.QueryRow(ctx, `
			SELECT c.name, c.base_currency, c.master_data_version, c.price_version, c.catalog_snapshot_token, c.business_timezone,
			       b.name, w.name,
			       COALESCE(array_agg(DISTINCT rp.permission_code ORDER BY rp.permission_code)
			           FILTER (WHERE rp.permission_code IS NOT NULL), ARRAY[]::text[])
			FROM users u
			JOIN user_role_scopes urs ON urs.tenant_id = u.tenant_id AND urs.user_id = u.id
			JOIN legal_companies c ON c.tenant_id = urs.tenant_id AND c.id = urs.company_id
			JOIN branches b ON b.tenant_id = urs.tenant_id AND b.company_id = urs.company_id AND b.id = urs.branch_id
			JOIN warehouses w ON w.tenant_id = urs.tenant_id AND w.company_id = urs.company_id
			    AND w.branch_id = urs.branch_id AND w.id = urs.warehouse_id
			LEFT JOIN role_permissions rp ON rp.tenant_id = urs.tenant_id AND rp.role_id = urs.role_id
			WHERE u.tenant_id = $1 AND u.id = $2 AND u.active
			  AND urs.company_id = $3 AND urs.branch_id = $4 AND urs.warehouse_id = $5
			GROUP BY c.name, c.base_currency, c.master_data_version, c.price_version, c.catalog_snapshot_token, c.business_timezone, b.name, w.name`,
			scope.TenantID, actorID, scope.CompanyID, scope.BranchID, scope.WarehouseID).Scan(
			&result.CompanyName, &result.Currency, &result.MasterDataVersion, &result.PriceVersion, &result.CatalogSnapshotToken, &result.TimeZone,
			&result.BranchName, &result.WarehouseName, &permissions)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return sales.ErrForbidden
			}
			return normalizeError(err)
		}
		result.ActorID, result.TenantID, result.CompanyID = actorID, scope.TenantID, scope.CompanyID
		result.BranchID, result.WarehouseID = scope.BranchID, scope.WarehouseID
		result.Locale, result.Permissions = "en-TZ", permissions
		return nil
	})
	return result, err
}

func (s *Store) ListCustomers(ctx context.Context, scope tenancy.Scope, actorID string, options readmodel.ListOptions) (readmodel.CatalogSnapshot, []readmodel.CustomerSummary, error) {
	var snapshot readmodel.CatalogSnapshot
	result := make([]readmodel.CustomerSummary, 0)
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "customers.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		snapshot, err = tx.catalogSnapshot(ctx, scope, options.CatalogSnapshotToken)
		if err != nil {
			return err
		}
		rows, err := tx.tx.Query(ctx, `
			SELECT c.id, c.code, c.name, CASE WHEN c.active THEN 'active' ELSE 'inactive' END,
			       c.is_general, c.credit_enabled, c.credit_limit_minor, exposure.amount_minor,
			       c.credit_limit_minor - exposure.amount_minor
			FROM customer_accounts c
			LEFT JOIN LATERAL (
				SELECT COALESCE(sum(cl.amount_minor), 0)::bigint AS amount_minor
				FROM customer_ledger cl
				WHERE cl.tenant_id = c.tenant_id AND cl.company_id = c.company_id AND cl.customer_id = c.id
			) exposure ON true
			WHERE c.tenant_id = $1 AND c.company_id = $2
			  AND ($3 = '' OR c.code ILIKE '%' || $3 || '%' OR c.name ILIKE '%' || $3 || '%')
			  AND ($4 = '' OR c.id > NULLIF($4, '')::uuid)
			  AND ($5::boolean IS NULL OR
			      (c.active AND NOT c.is_general AND c.credit_enabled
			       AND c.credit_limit_minor - exposure.amount_minor > 0) = $5)
			ORDER BY c.id LIMIT $6`, scope.TenantID, scope.CompanyID, options.Query,
			options.AfterID, options.CreditEligible, options.Limit+1)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var value readmodel.CustomerSummary
			if err := rows.Scan(&value.ID, &value.Code, &value.Name, &value.Status,
				&value.IsGeneralCustomer, &value.CreditEnabled, &value.CreditLimitMinor,
				&value.CurrentExposureMinor, &value.AvailableCreditMinor); err != nil {
				return normalizeError(err)
			}
			result = append(result, value)
		}
		return normalizeError(rows.Err())
	})
	return snapshot, result, err
}

func (s *Store) ListProducts(ctx context.Context, scope tenancy.Scope, actorID string, options readmodel.ListOptions) (readmodel.CatalogSnapshot, []readmodel.ProductSummary, error) {
	var snapshot readmodel.CatalogSnapshot
	result := make([]readmodel.ProductSummary, 0)
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "products.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		snapshot, err = tx.catalogSnapshot(ctx, scope, options.CatalogSnapshotToken)
		if err != nil {
			return err
		}
		rows, err := tx.tx.Query(ctx, `
			SELECT p.id, p.sku, p.name, p.base_unit_code, p.currency, p.list_price_minor,
			       COALESCE(sum(sl.quantity), 0)::bigint, p.price_version, c.master_data_version,
			       COALESCE(tax.basis_points, -1)::bigint
			FROM products p
			JOIN legal_companies c ON c.tenant_id=p.tenant_id AND c.id=p.company_id
			LEFT JOIN LATERAL (
				SELECT tr.basis_points FROM tax_rules tr
				WHERE tr.tenant_id=p.tenant_id AND tr.company_id=p.company_id AND tr.code=p.tax_code
				  AND tr.effective_from <= CURRENT_TIMESTAMP
				  AND (tr.effective_to IS NULL OR tr.effective_to > CURRENT_TIMESTAMP)
				ORDER BY tr.effective_from DESC LIMIT 1
			) tax ON true
			LEFT JOIN inventory_stock_ledger sl ON sl.tenant_id = p.tenant_id AND sl.company_id = p.company_id
			    AND sl.branch_id = $3 AND sl.warehouse_id = $4 AND sl.product_id = p.id
			WHERE p.tenant_id = $1 AND p.company_id = $2 AND p.active
			  AND ($5 = '' OR p.sku ILIKE '%' || $5 || '%' OR p.name ILIKE '%' || $5 || '%')
			  AND ($6 = '' OR p.id > NULLIF($6, '')::uuid)
			GROUP BY p.id, c.master_data_version, tax.basis_points
			ORDER BY p.id LIMIT $7`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID,
			options.Query, options.AfterID, options.Limit+1)
		if err != nil {
			return normalizeError(err)
		}
		defer rows.Close()
		for rows.Next() {
			var value readmodel.ProductSummary
			if err := rows.Scan(&value.ID, &value.Code, &value.Name, &value.Unit, &value.Currency,
				&value.UnitPriceMinor, &value.AvailableQuantity, &value.PriceVersion,
				&value.MasterDataVersion, &value.TaxBasisPoints); err != nil {
				return normalizeError(err)
			}
			if value.TaxBasisPoints < 0 || value.TaxBasisPoints > 10_000 {
				return sales.ErrPostingConfig
			}
			result = append(result, value)
		}
		return normalizeError(rows.Err())
	})
	return snapshot, result, err
}

func (t *transaction) catalogSnapshot(ctx context.Context, scope tenancy.Scope, expectedToken string) (readmodel.CatalogSnapshot, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return readmodel.CatalogSnapshot{}, err
	}
	lockKey := "master-data-version:" + scope.TenantID + ":" + scope.CompanyID
	if _, err := t.tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended($1, 0))`, lockKey); err != nil {
		return readmodel.CatalogSnapshot{}, normalizeError(err)
	}
	var value readmodel.CatalogSnapshot
	if err := t.tx.QueryRow(ctx, `SELECT catalog_snapshot_token, master_data_version, price_version
		FROM legal_companies WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.CompanyID).Scan(
		&value.Token, &value.MasterDataVersion, &value.PriceVersion); err != nil {
		return readmodel.CatalogSnapshot{}, normalizeError(err)
	}
	if expectedToken != "" && expectedToken != value.Token {
		return readmodel.CatalogSnapshot{}, devices.ErrStaleMasterData
	}
	return value, nil
}

func (s *Store) ListSales(ctx context.Context, scope tenancy.Scope, actorID string, options readmodel.ListOptions) ([]sales.Sale, error) {
	result := make([]sales.Sale, 0)
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		authorized, err := tx.Authorize(ctx, scope, actorID, "sales.read")
		if err != nil {
			return err
		}
		if !authorized {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `
			SELECT id FROM sales
			WHERE tenant_id = $1 AND company_id = $2 AND branch_id = $3 AND warehouse_id = $4
			  AND ($5 = '' OR id > NULLIF($5, '')::uuid)
			ORDER BY id LIMIT $6`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID,
			options.AfterID, options.Limit+1)
		if err != nil {
			return normalizeError(err)
		}
		ids := make([]string, 0)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return normalizeError(err)
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return normalizeError(err)
		}
		for _, id := range ids {
			value, err := tx.Sale(ctx, scope, id)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return nil
	})
	return result, err
}

func (t *transaction) MobileDevice(ctx context.Context, scope tenancy.Scope, actorID, deviceID string) (devices.Device, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return devices.Device{}, err
	}
	var value devices.Device
	err := t.tx.QueryRow(ctx, `
		SELECT d.id, d.status, d.actor_id, d.tenant_id, d.company_id, d.branch_id, d.warehouse_id,
		       d.device_name, d.app_version, d.master_data_version, d.price_version, d.catalog_snapshot_token, d.offline_enabled,
		       d.offline_transaction_limit_minor, d.offline_daily_limit_minor,
		       d.offline_sales_valid_from, d.offline_sales_valid_until, d.enrolled_at, d.last_seen_at,
		       c.business_timezone
		FROM mobile_devices d
		JOIN legal_companies c ON c.tenant_id = d.tenant_id AND c.id = d.company_id
		WHERE d.tenant_id = $1 AND d.id = $2 FOR UPDATE OF d`, scope.TenantID, deviceID).Scan(
		&value.ID, &value.Status, &value.ActorID, &value.Scope.TenantID, &value.Scope.CompanyID,
		&value.Scope.BranchID, &value.Scope.WarehouseID, &value.Name, &value.AppVersion,
		&value.MasterDataVersion, &value.PriceVersion, &value.CatalogSnapshotToken, &value.OfflineEnabled,
		&value.OfflineTransactionLimitMinor, &value.OfflineDailyLimitMinor,
		&value.OfflineSalesValidFrom, &value.OfflineSalesValidUntil,
		&value.EnrolledAt, &value.LastSeenAt, &value.TimeZone)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.Device{}, devices.ErrNotEnrolled
	}
	if err != nil {
		return devices.Device{}, normalizeError(err)
	}
	if value.ActorID != actorID || value.Scope != scope {
		return devices.Device{}, devices.ErrScopeMismatch
	}
	if err := t.tx.QueryRow(ctx, `
		SELECT master_data_version, price_version, catalog_snapshot_token FROM legal_companies
		WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.CompanyID).Scan(
		&value.AvailableMasterDataVersion, &value.AvailablePriceVersion, &value.AvailableCatalogSnapshotToken); err != nil {
		return devices.Device{}, normalizeError(err)
	}
	value.StockAllocations = make([]devices.StockAllocation, 0)
	rows, err := t.tx.Query(ctx, `
		SELECT a.product_id, a.allocated_quantity,
		       a.allocated_quantity - COALESCE(sum(sl.quantity), 0)::bigint AS remaining_quantity
		FROM mobile_device_stock_allocations a
		LEFT JOIN sales s ON s.tenant_id = a.tenant_id AND s.company_id = a.company_id
		    AND s.branch_id = a.branch_id AND s.warehouse_id = a.warehouse_id
		    AND s.device_id = a.device_id AND s.offline AND s.record_type = 'SALE' AND s.status = 'POSTED'
		LEFT JOIN sale_lines sl ON sl.tenant_id = s.tenant_id AND sl.company_id = s.company_id
		    AND sl.sale_id = s.id AND sl.product_id = a.product_id
		WHERE a.tenant_id = $1 AND a.company_id = $2 AND a.branch_id = $3 AND a.warehouse_id = $4
		  AND a.device_id = $5
		GROUP BY a.product_id, a.allocated_quantity ORDER BY a.product_id`,
		scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID, deviceID)
	if err != nil {
		return devices.Device{}, normalizeError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var allocation devices.StockAllocation
		if err := rows.Scan(&allocation.ProductID, &allocation.AllocatedQuantity, &allocation.RemainingQuantity); err != nil {
			return devices.Device{}, normalizeError(err)
		}
		value.StockAllocations = append(value.StockAllocations, allocation)
	}
	if err := rows.Err(); err != nil {
		return devices.Device{}, normalizeError(err)
	}
	location, err := time.LoadLocation(value.TimeZone)
	if err != nil {
		return devices.Device{}, devices.ErrInvalidTimeZone
	}
	localNow := time.Now().UTC().In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	var spent int64
	if err := t.tx.QueryRow(ctx, `
		SELECT COALESCE(sum(total_minor), 0)::bigint FROM sales
		WHERE tenant_id = $1 AND company_id = $2 AND branch_id = $3 AND warehouse_id = $4
		  AND device_id = $5 AND offline AND record_type = 'SALE' AND status = 'POSTED'
		  AND client_timestamp >= $6 AND client_timestamp < $7`, scope.TenantID, scope.CompanyID, scope.BranchID,
		scope.WarehouseID, deviceID, start.UTC(), end.UTC()).Scan(&spent); err != nil {
		return devices.Device{}, normalizeError(err)
	}
	value.OfflineRemainingDailyMinor = value.OfflineDailyLimitMinor - spent
	if value.OfflineRemainingDailyMinor < 0 {
		value.OfflineRemainingDailyMinor = 0
	}
	if err := devices.ValidateWireSafe(value); err != nil {
		return devices.Device{}, err
	}
	return value, nil
}

func (t *transaction) OfflineLeaseValid(ctx context.Context, lease devices.OfflineLease, clientTimestamp time.Time) (bool, error) {
	if err := t.ensureScope(ctx, lease.Scope); err != nil {
		return false, err
	}
	var valid bool
	err := t.tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM mobile_device_offline_leases
			WHERE tenant_id=$1 AND company_id=$2 AND branch_id=$3 AND warehouse_id=$4
			  AND device_id=$5 AND app_version=$6 AND master_data_version=$7 AND price_version=$8
			  AND catalog_snapshot_token=$9 AND valid_from <= $10 AND $10 < valid_until
		)`, lease.Scope.TenantID, lease.Scope.CompanyID, lease.Scope.BranchID, lease.Scope.WarehouseID,
		lease.DeviceID, lease.AppVersion, lease.MasterDataVersion, lease.PriceVersion, lease.CatalogSnapshotToken, clientTimestamp).Scan(&valid)
	return valid, normalizeError(err)
}

func (t *transaction) OfflineAllocation(ctx context.Context, scope tenancy.Scope, deviceID, productID string) (int64, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return 0, err
	}
	lockKey := strings.Join([]string{scope.TenantID, deviceID, productID}, ":")
	if _, err := t.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return 0, normalizeError(err)
	}
	var remaining int64
	err := t.tx.QueryRow(ctx, `
		SELECT a.allocated_quantity - COALESCE(sum(sl.quantity), 0)::bigint
		FROM mobile_device_stock_allocations a
		LEFT JOIN sales s ON s.tenant_id = a.tenant_id AND s.company_id = a.company_id
		    AND s.branch_id = a.branch_id AND s.warehouse_id = a.warehouse_id
		    AND s.device_id = a.device_id AND s.offline AND s.record_type = 'SALE' AND s.status = 'POSTED'
		LEFT JOIN sale_lines sl ON sl.tenant_id = s.tenant_id AND sl.company_id = s.company_id
		    AND sl.sale_id = s.id AND sl.product_id = a.product_id
		WHERE a.tenant_id = $1 AND a.company_id = $2 AND a.branch_id = $3 AND a.warehouse_id = $4
		  AND a.device_id = $5 AND a.product_id = $6
		GROUP BY a.allocated_quantity`, scope.TenantID, scope.CompanyID, scope.BranchID, scope.WarehouseID,
		deviceID, productID).Scan(&remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, devices.ErrAllocationExceeded
	}
	return remaining, normalizeError(err)
}

func (t *transaction) OfflineSalesTotal(ctx context.Context, scope tenancy.Scope, deviceID string, startsAt, endsAt time.Time) (int64, error) {
	if err := t.ensureScope(ctx, scope); err != nil {
		return 0, err
	}
	var total int64
	err := t.tx.QueryRow(ctx, `
		SELECT COALESCE(sum(total_minor), 0)::bigint FROM sales
		WHERE tenant_id = $1 AND company_id = $2 AND branch_id = $3 AND warehouse_id = $4
		  AND device_id = $5 AND offline AND record_type = 'SALE' AND status = 'POSTED'
		  AND client_timestamp >= $6 AND client_timestamp < $7`, scope.TenantID, scope.CompanyID, scope.BranchID,
		scope.WarehouseID, deviceID, startsAt, endsAt).Scan(&total)
	return total, normalizeError(err)
}
