package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
)

func (s *Store) TenantIDs(ctx context.Context) ([]string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+s.searchPath); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
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

func (s *Store) Claim(ctx context.Context, tenantID, workerID string, limit int, now, lockedUntil time.Time) ([]outbox.Event, error) {
	if tenantID == "" || workerID == "" || limit < 1 || !lockedUntil.After(now) {
		return nil, errors.New("invalid outbox claim")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
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
		SELECT id FROM outbox_events
		WHERE tenant_id=$1 AND processed_at IS NULL AND available_at <= $4
		  AND (locked_until IS NULL OR locked_until < $4)
		ORDER BY occurred_at, id FOR UPDATE SKIP LOCKED LIMIT $3
	) UPDATE outbox_events event
	SET locked_by=$2, locked_until=$5, attempts=event.attempts+1, last_error=NULL
	FROM candidates WHERE event.id=candidates.id
	RETURNING event.id,event.tenant_id,event.company_id,event.aggregate_type,event.aggregate_id,
	          event.event_type,event.version,event.correlation_id,event.causation_id,event.payload,event.occurred_at`, tenantID, workerID, limit, now, lockedUntil)
	if err != nil {
		return nil, err
	}
	var result []outbox.Event
	for rows.Next() {
		var event outbox.Event
		if err := rows.Scan(&event.ID, &event.TenantID, &event.CompanyID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Version, &event.CorrelationID, &event.CausationID, &event.Payload, &event.OccurredAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) MarkPublished(ctx context.Context, tenantID, eventID, workerID string, publishedAt time.Time) error {
	return s.updateDelivery(ctx, tenantID, eventID, workerID, `processed_at=$4, locked_by=NULL, locked_until=NULL, last_error=NULLIF($5,'')`, publishedAt, "")
}

func (s *Store) MarkFailed(ctx context.Context, tenantID, eventID, workerID string, retryAt time.Time, reason string) error {
	if len(reason) > 2000 {
		reason = reason[:2000]
	}
	return s.updateDelivery(ctx, tenantID, eventID, workerID, `available_at=$4, locked_by=NULL, locked_until=NULL, last_error=$5`, retryAt, reason)
}

func (s *Store) updateDelivery(ctx context.Context, tenantID, eventID, workerID, assignment string, at time.Time, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+s.searchPath); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true)`, tenantID); err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `UPDATE outbox_events SET `+assignment+`
		WHERE tenant_id=$1 AND id=$2 AND locked_by=$3 AND processed_at IS NULL`, tenantID, eventID, workerID, at, reason)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("outbox lease is absent or no longer owned")
	}
	return tx.Commit(ctx)
}
