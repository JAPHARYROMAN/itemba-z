package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/itemba-z/itemba-z/services/core-api/internal/integrations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
)

func (s *Store) ProjectFiscalDelivery(ctx context.Context, event outbox.Event, deliveryID, operation, requestHash string) error {
	if event.TenantID == "" || event.CompanyID == "" || event.ID == "" || event.AggregateType != "sale" || event.AggregateID == "" || event.CorrelationID == "" || deliveryID == "" || operation == "" || len(requestHash) != 64 || !json.Valid(event.Payload) {
		return errors.New("invalid fiscal delivery projection")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+s.searchPath); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true)`, event.TenantID); err != nil {
		return err
	}
	var route integrations.Route
	err = tx.QueryRow(ctx, `
		SELECT id::text,tenant_id::text,company_id::text,capability,provider_code,contract_version,
		       endpoint_url,secret_reference,timeout_milliseconds,max_attempts,base_backoff_seconds,
		       max_backoff_seconds,circuit_failure_threshold,circuit_open_seconds
		FROM integration_routes
		WHERE tenant_id=$1 AND company_id=$2 AND capability='TRA_FISCALIZATION'
		  AND status='ACTIVE' AND valid_from <= $3 AND (valid_until IS NULL OR valid_until > $3)`,
		event.TenantID, event.CompanyID, event.OccurredAt).Scan(
		&route.ID, &route.TenantID, &route.CompanyID, &route.Capability, &route.ProviderCode,
		&route.ContractVersion, &route.EndpointURL, &route.SecretReference, durationMilliseconds(&route.Timeout),
		&route.MaxAttempts, durationSeconds(&route.BaseBackoff), durationSeconds(&route.MaxBackoff),
		&route.CircuitFailureThreshold, durationSeconds(&route.CircuitOpen))
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return normalizeError(err)
	}
	command, err := tx.Exec(ctx, `
		INSERT INTO integration_deliveries(
			id,tenant_id,company_id,route_id,capability,operation,source_event_id,
			aggregate_type,aggregate_id,correlation_id,idempotency_key,request_hash,
			request_payload,status,max_attempts,available_at,created_at
		) VALUES($1,$2,$3,$4,'TRA_FISCALIZATION',$5,$6,$7,$8,$9,$14,$10,$11,'PENDING',$12,$13,$13)
		ON CONFLICT(tenant_id,source_event_id,capability) DO NOTHING`,
		deliveryID, event.TenantID, event.CompanyID, route.ID, operation, event.ID,
		event.AggregateType, event.AggregateID, event.CorrelationID, requestHash, event.Payload,
		route.MaxAttempts, event.OccurredAt, event.ID)
	if err != nil {
		return normalizeError(err)
	}
	if command.RowsAffected() == 1 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO integration_route_health(tenant_id,company_id,route_id,updated_at)
			VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,route_id) DO NOTHING`,
			event.TenantID, event.CompanyID, route.ID, event.OccurredAt); err != nil {
			return normalizeError(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE sales SET fiscal_status='PENDING'
			WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND fiscal_status IN ('NOT_CONFIGURED','FAILED')`,
			event.TenantID, event.CompanyID, event.AggregateID); err != nil {
			return normalizeError(err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) IntegrationTenantIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT tenant_id::text FROM `+pgx.Identifier{s.schema}.Sanitize()+`.pending_integration_tenant_ids()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, err
		}
		result = append(result, tenantID)
	}
	return result, rows.Err()
}

func (s *Store) ClaimIntegrationDeliveries(ctx context.Context, tenantID, workerID string, limit int, now, lockedUntil time.Time) ([]integrations.Delivery, error) {
	if tenantID == "" || workerID == "" || limit < 1 || !lockedUntil.After(now) {
		return nil, errors.New("invalid integration delivery claim")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+s.searchPath); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true)`, tenantID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `WITH candidates AS (
		SELECT delivery.id
		FROM integration_deliveries delivery
		JOIN integration_routes route ON route.tenant_id=delivery.tenant_id AND route.id=delivery.route_id
		LEFT JOIN integration_route_health health ON health.tenant_id=route.tenant_id AND health.route_id=route.id
		WHERE delivery.tenant_id=$1 AND delivery.status IN ('PENDING','RETRY_SCHEDULED','IN_FLIGHT')
		  AND delivery.available_at <= $4 AND (delivery.locked_until IS NULL OR delivery.locked_until < $4)
		  AND route.status='ACTIVE' AND route.valid_from <= $4 AND (route.valid_until IS NULL OR route.valid_until > $4)
		  AND (health.opened_until IS NULL OR health.opened_until <= $4)
		ORDER BY delivery.created_at,delivery.id FOR UPDATE OF delivery SKIP LOCKED LIMIT $3
	), claimed AS (
		UPDATE integration_deliveries delivery SET status='IN_FLIGHT',locked_by=$2,locked_until=$5,
		       attempt_count=delivery.attempt_count+1,last_error_code=NULL,last_error_message=NULL
		FROM candidates WHERE delivery.id=candidates.id RETURNING delivery.*
	)
	SELECT claimed.id::text,claimed.tenant_id::text,claimed.company_id::text,claimed.capability,
	       claimed.operation,claimed.source_event_id::text,claimed.aggregate_type,claimed.aggregate_id::text,
	       claimed.correlation_id::text,claimed.idempotency_key,claimed.request_hash,claimed.request_payload,
	       claimed.status,claimed.attempt_count,claimed.max_attempts,claimed.available_at,$4,claimed.locked_by,
	       route.id::text,route.provider_code,route.contract_version,route.endpoint_url,route.secret_reference,
	       route.timeout_milliseconds,route.base_backoff_seconds,route.max_backoff_seconds,
	       route.circuit_failure_threshold,route.circuit_open_seconds
	FROM claimed JOIN integration_routes route ON route.tenant_id=claimed.tenant_id AND route.id=claimed.route_id`,
		tenantID, workerID, limit, now, lockedUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []integrations.Delivery
	for rows.Next() {
		var delivery integrations.Delivery
		var timeoutMS, baseSeconds, maxSeconds, openSeconds int64
		if err := rows.Scan(&delivery.ID, &delivery.TenantID, &delivery.CompanyID, &delivery.Capability,
			&delivery.Operation, &delivery.SourceEventID, &delivery.AggregateType, &delivery.AggregateID,
			&delivery.CorrelationID, &delivery.IdempotencyKey, &delivery.RequestHash, &delivery.RequestPayload,
			&delivery.Status, &delivery.AttemptCount, &delivery.MaxAttempts, &delivery.AvailableAt,
			&delivery.AttemptStartedAt, &delivery.LockedBy, &delivery.Route.ID, &delivery.Route.ProviderCode,
			&delivery.Route.ContractVersion, &delivery.Route.EndpointURL, &delivery.Route.SecretReference,
			&timeoutMS, &baseSeconds, &maxSeconds, &delivery.Route.CircuitFailureThreshold, &openSeconds); err != nil {
			return nil, err
		}
		delivery.Route.TenantID, delivery.Route.CompanyID, delivery.Route.Capability = delivery.TenantID, delivery.CompanyID, delivery.Capability
		delivery.Route.MaxAttempts = delivery.MaxAttempts
		delivery.Route.Timeout = time.Duration(timeoutMS) * time.Millisecond
		delivery.Route.BaseBackoff = time.Duration(baseSeconds) * time.Second
		delivery.Route.MaxBackoff = time.Duration(maxSeconds) * time.Second
		delivery.Route.CircuitOpen = time.Duration(openSeconds) * time.Second
		result = append(result, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) CompleteIntegrationDelivery(ctx context.Context, delivery integrations.Delivery, result integrations.Result, completedAt time.Time) error {
	response := integrationNullableJSON(result.Response)
	return s.finishIntegrationDelivery(ctx, delivery, "SUCCEEDED", "", "", result.ProviderReference, response, completedAt, completedAt, false)
}

func (s *Store) FailIntegrationDelivery(ctx context.Context, delivery integrations.Delivery, failure integrations.Failure, retryAt time.Time, deadLetter bool, completedAt time.Time) error {
	status := "RETRY_SCHEDULED"
	if deadLetter {
		status = "DEAD_LETTER"
	}
	return s.finishIntegrationDelivery(ctx, delivery, status, safeText(failure.Code, 100), safeText(failure.Message, 1000), "", nil, retryAt, completedAt, deadLetter)
}

func (s *Store) finishIntegrationDelivery(ctx context.Context, delivery integrations.Delivery, status, errorCode, errorMessage, providerReference string, response any, availableAt, completedAt time.Time, deadLetter bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+s.searchPath); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true)`, delivery.TenantID); err != nil {
		return err
	}
	var finished any
	if status == "SUCCEEDED" {
		finished = completedAt
	}
	command, err := tx.Exec(ctx, `UPDATE integration_deliveries SET status=$4,available_at=$5,
		locked_by=NULL,locked_until=NULL,provider_reference=NULLIF($6,''),response_payload=$7,
		last_error_code=NULLIF($8,''),last_error_message=NULLIF($9,''),completed_at=$10
		WHERE tenant_id=$1 AND id=$2 AND locked_by=$3 AND status='IN_FLIGHT'`,
		delivery.TenantID, delivery.ID, delivery.LockedBy, status, availableAt, providerReference,
		response, errorCode, errorMessage, finished)
	if err != nil {
		return normalizeError(err)
	}
	if command.RowsAffected() != 1 {
		return errors.New("integration delivery lease is absent or no longer owned")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO integration_delivery_attempts(
		id,tenant_id,company_id,delivery_id,attempt_number,worker_id,outcome,error_code,error_message,
		provider_reference,response_payload,started_at,completed_at
	) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11,$12)`,
		delivery.TenantID, delivery.CompanyID, delivery.ID, delivery.AttemptCount, delivery.LockedBy,
		status, errorCode, errorMessage, providerReference, response, delivery.AttemptStartedAt, completedAt); err != nil {
		return normalizeError(err)
	}
	if status == "SUCCEEDED" {
		if _, err := tx.Exec(ctx, `UPDATE integration_route_health SET consecutive_failures=0,opened_until=NULL,
			last_success_at=$4,updated_at=$4 WHERE tenant_id=$1 AND company_id=$2 AND route_id=$3`,
			delivery.TenantID, delivery.CompanyID, delivery.Route.ID, completedAt); err != nil {
			return normalizeError(err)
		}
		if delivery.Capability == integrations.TRAFiscalization && delivery.AggregateType == "sale" {
			if _, err := tx.Exec(ctx, `UPDATE sales SET fiscal_status='FISCALIZED' WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND fiscal_status='PENDING'`, delivery.TenantID, delivery.CompanyID, delivery.AggregateID); err != nil {
				return normalizeError(err)
			}
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE integration_route_health SET
			consecutive_failures=consecutive_failures+1,last_failure_at=$4,updated_at=$4,
			opened_until=CASE WHEN consecutive_failures+1 >= $5 THEN $4 + ($6 * interval '1 second') ELSE opened_until END
			WHERE tenant_id=$1 AND company_id=$2 AND route_id=$3`, delivery.TenantID, delivery.CompanyID,
			delivery.Route.ID, completedAt, delivery.Route.CircuitFailureThreshold, int64(delivery.Route.CircuitOpen/time.Second)); err != nil {
			return normalizeError(err)
		}
		if deadLetter && delivery.Capability == integrations.TRAFiscalization && delivery.AggregateType == "sale" {
			if _, err := tx.Exec(ctx, `UPDATE sales SET fiscal_status='FAILED' WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND fiscal_status='PENDING'`, delivery.TenantID, delivery.CompanyID, delivery.AggregateID); err != nil {
				return normalizeError(err)
			}
		}
	}
	return tx.Commit(ctx)
}

type durationScan struct {
	target *time.Duration
	unit   time.Duration
}

func durationMilliseconds(target *time.Duration) *durationScan {
	return &durationScan{target: target, unit: time.Millisecond}
}
func durationSeconds(target *time.Duration) *durationScan {
	return &durationScan{target: target, unit: time.Second}
}

func (d *durationScan) Scan(value any) error {
	var number int64
	switch typed := value.(type) {
	case int64:
		number = typed
	case int32:
		number = int64(typed)
	default:
		return fmt.Errorf("scan duration from %T", value)
	}
	*d.target = time.Duration(number) * d.unit
	return nil
}

func integrationNullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func safeText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
