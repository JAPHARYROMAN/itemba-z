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
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
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

func (s *Store) IntegrationWorkspace(ctx context.Context, scope tenancy.Scope, actor string) (integrations.Workspace, error) {
	result := integrations.Workspace{Scope: scope, Routes: []integrations.Route{}, Deliveries: []integrations.Delivery{}, Attempts: []integrations.Attempt{}}
	routesByID := map[string]integrations.Route{}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actor, "integrations.read")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		rows, err := tx.tx.Query(ctx, `SELECT r.id::text,r.capability,r.provider_code,r.contract_version,r.endpoint_url,r.secret_reference,
			r.timeout_milliseconds,r.max_attempts,r.base_backoff_seconds,r.max_backoff_seconds,r.circuit_failure_threshold,r.circuit_open_seconds,
			r.status,r.valid_from,r.valid_until,r.reason,r.created_by::text,r.created_at,COALESCE(r.approved_by::text,''),r.approved_at,
			COALESCE(h.consecutive_failures,0),h.opened_until,h.last_success_at,h.last_failure_at,h.updated_at
			FROM integration_routes r LEFT JOIN integration_route_health h ON h.tenant_id=r.tenant_id AND h.route_id=r.id
			WHERE r.tenant_id=$1 AND r.company_id=$2 ORDER BY r.created_at DESC`, scope.TenantID, scope.CompanyID)
		if err != nil {
			return normalizeError(err)
		}
		for rows.Next() {
			var route integrations.Route
			if err = rows.Scan(&route.ID, &route.Capability, &route.ProviderCode, &route.ContractVersion, &route.EndpointURL, &route.SecretReference, &route.TimeoutMilliseconds, &route.MaxAttempts, &route.BaseBackoffSeconds, &route.MaxBackoffSeconds, &route.CircuitFailureThreshold, &route.CircuitOpenSeconds, &route.Status, &route.ValidFrom, &route.ValidUntil, &route.Reason, &route.CreatedBy, &route.CreatedAt, &route.ApprovedBy, &route.ApprovedAt, &route.Health.ConsecutiveFailures, &route.Health.OpenedUntil, &route.Health.LastSuccessAt, &route.Health.LastFailureAt, &route.Health.UpdatedAt); err != nil {
				rows.Close()
				return normalizeError(err)
			}
			route.TenantID, route.CompanyID = scope.TenantID, scope.CompanyID
			route.Timeout = time.Duration(route.TimeoutMilliseconds) * time.Millisecond
			route.BaseBackoff = time.Duration(route.BaseBackoffSeconds) * time.Second
			route.MaxBackoff = time.Duration(route.MaxBackoffSeconds) * time.Second
			route.CircuitOpen = time.Duration(route.CircuitOpenSeconds) * time.Second
			result.Routes = append(result.Routes, route)
			routesByID[route.ID] = route
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return normalizeError(err)
		}
		rows.Close()
		rows, err = tx.tx.Query(ctx, `SELECT d.id::text,d.route_id::text,d.capability,d.operation,d.source_event_id::text,d.aggregate_type,d.aggregate_id::text,d.correlation_id::text,d.idempotency_key,d.request_hash,d.status,d.attempt_count,d.max_attempts,d.available_at,COALESCE(d.locked_by,''),COALESCE(d.provider_reference,''),COALESCE(d.last_error_code,''),COALESCE(d.last_error_message,''),d.created_at,d.completed_at,r.provider_code,r.contract_version
			FROM integration_deliveries d JOIN integration_routes r ON r.tenant_id=d.tenant_id AND r.id=d.route_id
			WHERE d.tenant_id=$1 AND d.company_id=$2 ORDER BY d.created_at DESC LIMIT 200`, scope.TenantID, scope.CompanyID)
		if err != nil {
			return normalizeError(err)
		}
		for rows.Next() {
			var delivery integrations.Delivery
			if err = rows.Scan(&delivery.ID, &delivery.Route.ID, &delivery.Capability, &delivery.Operation, &delivery.SourceEventID, &delivery.AggregateType, &delivery.AggregateID, &delivery.CorrelationID, &delivery.IdempotencyKey, &delivery.RequestHash, &delivery.Status, &delivery.AttemptCount, &delivery.MaxAttempts, &delivery.AvailableAt, &delivery.LockedBy, &delivery.ProviderReference, &delivery.LastErrorCode, &delivery.LastErrorMessage, &delivery.CreatedAt, &delivery.CompletedAt, &delivery.Route.ProviderCode, &delivery.Route.ContractVersion); err != nil {
				rows.Close()
				return normalizeError(err)
			}
			delivery.TenantID, delivery.CompanyID = scope.TenantID, scope.CompanyID
			if route, found := routesByID[delivery.Route.ID]; found {
				delivery.Route = route
			} else {
				delivery.Route.TenantID, delivery.Route.CompanyID = scope.TenantID, scope.CompanyID
			}
			result.Deliveries = append(result.Deliveries, delivery)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return normalizeError(err)
		}
		rows.Close()
		rows, err = tx.tx.Query(ctx, `SELECT a.id::text,a.delivery_id::text,a.attempt_number,a.worker_id,a.outcome,COALESCE(a.error_code,''),COALESCE(a.error_message,''),COALESCE(a.provider_reference,''),a.started_at,a.completed_at FROM integration_delivery_attempts a WHERE a.tenant_id=$1 AND a.company_id=$2 ORDER BY a.completed_at DESC LIMIT 500`, scope.TenantID, scope.CompanyID)
		if err != nil {
			return normalizeError(err)
		}
		for rows.Next() {
			var attempt integrations.Attempt
			if err = rows.Scan(&attempt.ID, &attempt.DeliveryID, &attempt.AttemptNumber, &attempt.WorkerID, &attempt.Outcome, &attempt.ErrorCode, &attempt.ErrorMessage, &attempt.ProviderReference, &attempt.StartedAt, &attempt.CompletedAt); err != nil {
				rows.Close()
				return normalizeError(err)
			}
			result.Attempts = append(result.Attempts, attempt)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return normalizeError(err)
		}
		rows.Close()
		for _, delivery := range result.Deliveries {
			result.Reconciliation.Total++
			switch delivery.Status {
			case integrations.Pending:
				result.Reconciliation.Pending++
			case integrations.InFlight:
				result.Reconciliation.InFlight++
			case integrations.RetryScheduled:
				result.Reconciliation.Retrying++
			case integrations.Succeeded:
				result.Reconciliation.Succeeded++
			case integrations.DeadLetter:
				result.Reconciliation.DeadLetter++
			}
		}
		result.Reconciliation.Unreconciled = result.Reconciliation.Pending + result.Reconciliation.InFlight + result.Reconciliation.Retrying + result.Reconciliation.DeadLetter
		return nil
	})
	return result, err
}

func (s *Store) CreateIntegrationRoute(ctx context.Context, scope tenancy.Scope, route integrations.Route, idem, hash string) (integrations.Route, error) {
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, route.CreatedBy, "integrations.manage")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, resultID, err := tx.ClaimIdempotency(ctx, scope, "integrations.route.create.v1", idem, hash)
		if err != nil {
			return err
		}
		if !acquired {
			route.ID = resultID
			return nil
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO integration_routes(id,tenant_id,company_id,capability,provider_code,contract_version,endpoint_url,secret_reference,timeout_milliseconds,max_attempts,base_backoff_seconds,max_backoff_seconds,circuit_failure_threshold,circuit_open_seconds,status,valid_from,valid_until,created_by,reason,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'DRAFT',$15,$16,$17,$18,$19)`, route.ID, route.TenantID, route.CompanyID, route.Capability, route.ProviderCode, route.ContractVersion, route.EndpointURL, route.SecretReference, route.TimeoutMilliseconds, route.MaxAttempts, route.BaseBackoffSeconds, route.MaxBackoffSeconds, route.CircuitFailureThreshold, route.CircuitOpenSeconds, route.ValidFrom, route.ValidUntil, route.CreatedBy, route.Reason, route.CreatedAt)
		if err != nil {
			return normalizeError(err)
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO integration_route_health(tenant_id,company_id,route_id,updated_at) VALUES($1,$2,$3,$4)`, route.TenantID, route.CompanyID, route.ID, route.CreatedAt)
		if err != nil {
			return normalizeError(err)
		}
		if err = tx.financialEvidence(ctx, scope, route.CreatedBy, "integrations.route_created", "integration_route", route.ID, route.ID, route.CreatedAt); err != nil {
			return err
		}
		return tx.CompleteIdempotency(ctx, scope, "integrations.route.create.v1", idem, route.ID)
	})
	return route, err
}

func (s *Store) TransitionIntegrationRoute(ctx context.Context, scope tenancy.Scope, actor, id string, to integrations.RouteStatus, reason, idem, hash string, at time.Time) (integrations.Route, error) {
	route := integrations.Route{ID: id, TenantID: scope.TenantID, CompanyID: scope.CompanyID}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actor, "integrations.manage")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		op := "integrations.route.transition." + string(to) + ".v1"
		acquired, _, err := tx.ClaimIdempotency(ctx, scope, op, idem, hash)
		if err != nil {
			return err
		}
		if !acquired {
			return nil
		}
		if err = tx.tx.QueryRow(ctx, `SELECT status,created_by::text,capability,provider_code,contract_version,endpoint_url,secret_reference,timeout_milliseconds,max_attempts,base_backoff_seconds,max_backoff_seconds,circuit_failure_threshold,circuit_open_seconds,valid_from,valid_until,reason,created_at FROM integration_routes WHERE tenant_id=$1 AND company_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.CompanyID, id).Scan(&route.Status, &route.CreatedBy, &route.Capability, &route.ProviderCode, &route.ContractVersion, &route.EndpointURL, &route.SecretReference, &route.TimeoutMilliseconds, &route.MaxAttempts, &route.BaseBackoffSeconds, &route.MaxBackoffSeconds, &route.CircuitFailureThreshold, &route.CircuitOpenSeconds, &route.ValidFrom, &route.ValidUntil, &route.Reason, &route.CreatedAt); err != nil {
			return normalizeError(err)
		}
		from := route.Status
		valid := from == integrations.RouteDraft && (to == integrations.RouteActive || to == integrations.RouteRetired) || from == integrations.RouteActive && (to == integrations.RouteSuspended || to == integrations.RouteRetired) || from == integrations.RouteSuspended && (to == integrations.RouteActive || to == integrations.RouteRetired)
		if !valid {
			return integrations.ErrInvalidTransition
		}
		if from == integrations.RouteDraft && actor == route.CreatedBy {
			return integrations.ErrSeparationOfDuties
		}
		_, err = tx.tx.Exec(ctx, `UPDATE integration_routes SET status=$1,approved_by=$2,approved_at=$3 WHERE tenant_id=$4 AND company_id=$5 AND id=$6`, to, actor, at, scope.TenantID, scope.CompanyID, id)
		if err != nil {
			return normalizeError(err)
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO integration_route_transitions(tenant_id,company_id,route_id,from_status,to_status,reason,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.TenantID, scope.CompanyID, id, from, to, reason, actor, at)
		if err != nil {
			return normalizeError(err)
		}
		if err = tx.financialEvidence(ctx, scope, actor, "integrations.route_"+strings.ToLower(string(to)), "integration_route", id, id, at); err != nil {
			return err
		}
		if err = tx.CompleteIdempotency(ctx, scope, op, idem, id); err != nil {
			return err
		}
		route.Status = to
		route.ApprovedBy = actor
		route.ApprovedAt = &at
		return nil
	})
	return route, err
}

func (s *Store) ReplayIntegrationDelivery(ctx context.Context, scope tenancy.Scope, actor, deliveryID, replayID, reason, idem, hash string, at time.Time) (integrations.Delivery, error) {
	delivery := integrations.Delivery{ID: deliveryID, TenantID: scope.TenantID, CompanyID: scope.CompanyID}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actor, "integrations.replay")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, _, err := tx.ClaimIdempotency(ctx, scope, "integrations.delivery.replay.v1", idem, hash)
		if err != nil {
			return err
		}
		if !acquired {
			return nil
		}
		var routeAttempts int
		if err = tx.tx.QueryRow(ctx, `SELECT d.status,d.attempt_count,d.max_attempts,d.capability,d.operation,d.aggregate_type,d.aggregate_id::text,d.correlation_id::text,d.created_at,r.id::text,r.max_attempts,r.provider_code,r.contract_version FROM integration_deliveries d JOIN integration_routes r ON r.tenant_id=d.tenant_id AND r.id=d.route_id WHERE d.tenant_id=$1 AND d.company_id=$2 AND d.id=$3 AND r.status='ACTIVE' FOR UPDATE OF d`, scope.TenantID, scope.CompanyID, deliveryID).Scan(&delivery.Status, &delivery.AttemptCount, &delivery.MaxAttempts, &delivery.Capability, &delivery.Operation, &delivery.AggregateType, &delivery.AggregateID, &delivery.CorrelationID, &delivery.CreatedAt, &delivery.Route.ID, &routeAttempts, &delivery.Route.ProviderCode, &delivery.Route.ContractVersion); err != nil {
			return normalizeError(err)
		}
		if delivery.Status != integrations.DeadLetter {
			return integrations.ErrReplayUnavailable
		}
		newBudget := delivery.AttemptCount + routeAttempts
		_, err = tx.tx.Exec(ctx, `UPDATE integration_deliveries SET status='PENDING',max_attempts=$1,available_at=$2,locked_by=NULL,locked_until=NULL,last_error_code=NULL,last_error_message=NULL WHERE tenant_id=$3 AND company_id=$4 AND id=$5`, newBudget, at, scope.TenantID, scope.CompanyID, deliveryID)
		if err != nil {
			return normalizeError(err)
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO integration_delivery_replays(id,tenant_id,company_id,delivery_id,previous_attempt_count,new_attempt_budget,reason,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, replayID, scope.TenantID, scope.CompanyID, deliveryID, delivery.AttemptCount, newBudget, reason, actor, at)
		if err != nil {
			return normalizeError(err)
		}
		if delivery.Capability == integrations.TRAFiscalization && delivery.AggregateType == "sale" {
			if _, err = tx.tx.Exec(ctx, `UPDATE sales SET fiscal_status='PENDING' WHERE tenant_id=$1 AND company_id=$2 AND id=$3 AND fiscal_status='FAILED'`, scope.TenantID, scope.CompanyID, delivery.AggregateID); err != nil {
				return normalizeError(err)
			}
		}
		if err = tx.financialEvidence(ctx, scope, actor, "integrations.delivery_replayed", "integration_delivery", deliveryID, replayID, at); err != nil {
			return err
		}
		if err = tx.CompleteIdempotency(ctx, scope, "integrations.delivery.replay.v1", idem, deliveryID); err != nil {
			return err
		}
		delivery.Status = integrations.Pending
		delivery.MaxAttempts = newBudget
		delivery.AvailableAt = at
		return nil
	})
	return delivery, err
}

func (s *Store) ResetIntegrationCircuit(ctx context.Context, scope tenancy.Scope, actor, routeID, resetID, reason, idem, hash string, at time.Time) (integrations.Route, error) {
	route := integrations.Route{ID: routeID, TenantID: scope.TenantID, CompanyID: scope.CompanyID}
	err := s.WithTransaction(ctx, func(contract sales.Transaction) error {
		tx := contract.(*transaction)
		allowed, err := tx.Authorize(ctx, scope, actor, "integrations.manage")
		if err != nil {
			return err
		}
		if !allowed {
			return sales.ErrForbidden
		}
		acquired, _, err := tx.ClaimIdempotency(ctx, scope, "integrations.circuit.reset.v1", idem, hash)
		if err != nil {
			return err
		}
		if !acquired {
			return nil
		}
		if err = tx.tx.QueryRow(ctx, `SELECT r.status,r.provider_code,r.capability,h.consecutive_failures,h.opened_until FROM integration_routes r JOIN integration_route_health h ON h.tenant_id=r.tenant_id AND h.route_id=r.id WHERE r.tenant_id=$1 AND r.company_id=$2 AND r.id=$3 FOR UPDATE OF h`, scope.TenantID, scope.CompanyID, routeID).Scan(&route.Status, &route.ProviderCode, &route.Capability, &route.Health.ConsecutiveFailures, &route.Health.OpenedUntil); err != nil {
			return normalizeError(err)
		}
		_, err = tx.tx.Exec(ctx, `INSERT INTO integration_circuit_resets(id,tenant_id,company_id,route_id,previous_failures,previous_opened_until,reason,actor_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, resetID, scope.TenantID, scope.CompanyID, routeID, route.Health.ConsecutiveFailures, route.Health.OpenedUntil, reason, actor, at)
		if err != nil {
			return normalizeError(err)
		}
		_, err = tx.tx.Exec(ctx, `UPDATE integration_route_health SET consecutive_failures=0,opened_until=NULL,updated_at=$1 WHERE tenant_id=$2 AND company_id=$3 AND route_id=$4`, at, scope.TenantID, scope.CompanyID, routeID)
		if err != nil {
			return normalizeError(err)
		}
		if err = tx.financialEvidence(ctx, scope, actor, "integrations.circuit_reset", "integration_route", routeID, resetID, at); err != nil {
			return err
		}
		if err = tx.CompleteIdempotency(ctx, scope, "integrations.circuit.reset.v1", idem, routeID); err != nil {
			return err
		}
		route.Health = integrations.RouteHealth{UpdatedAt: &at}
		return nil
	})
	return route, err
}
