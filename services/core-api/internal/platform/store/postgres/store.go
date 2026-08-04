// Package postgres is the authoritative PostgreSQL adapter for the modular
// monolith. Every application transaction receives a tenant-local RLS context.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/dbrole"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

var schemaPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

type Store struct {
	pool       *pgxpool.Pool
	schema     string
	searchPath string
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	return open(ctx, databaseURL, dbrole.APIRuntimeGroup, dbrole.WorkerRuntimeGroup)
}

func OpenWorker(ctx context.Context, databaseURL string) (*Store, error) {
	return open(ctx, databaseURL, dbrole.WorkerRuntimeGroup, dbrole.APIRuntimeGroup)
}

func open(ctx context.Context, databaseURL, requiredGroup, forbiddenGroup string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if err := validateRuntimePool(ctx, pool, "itembaz", requiredGroup, forbiddenGroup); err != nil {
		pool.Close()
		return nil, err
	}
	return New(pool, "itembaz")
}

func validateRuntimePool(ctx context.Context, pool *pgxpool.Pool, schema, requiredGroup, forbiddenGroup string) error {
	var groupExists bool
	if err := pool.QueryRow(ctx, `SELECT count(*)=2 FROM pg_roles WHERE rolname=ANY($1::text[])`,
		[]string{requiredGroup, forbiddenGroup}).Scan(&groupExists); err != nil {
		return fmt.Errorf("inspect PostgreSQL runtime role: %w", err)
	}
	if !groupExists {
		return errors.New("PostgreSQL runtime role is absent; apply migrations and provision a runtime login")
	}
	var currentUser string
	var superuser, bypassRLS, inherit, member, ownsTables, schemaExists, schemaUsage bool
	err := pool.QueryRow(ctx, `
		SELECT current_user, r.rolsuper, r.rolbypassrls, r.rolinherit,
		       pg_has_role(current_user, $2, 'MEMBER'),
		       pg_has_role(current_user, $3, 'MEMBER'),
		       EXISTS (
		           SELECT 1 FROM pg_class c
		           JOIN pg_namespace n ON n.oid=c.relnamespace
		           WHERE n.nspname=$1 AND c.relowner=r.oid AND c.relkind IN ('r','p')
		       ),
		       to_regnamespace($1) IS NOT NULL,
		       has_schema_privilege(current_user, $1, 'USAGE')
		FROM pg_roles r WHERE r.rolname=current_user`, schema, requiredGroup, forbiddenGroup).Scan(
		&currentUser, &superuser, &bypassRLS, &inherit, &member, &groupExists, &ownsTables, &schemaExists, &schemaUsage)
	if err != nil {
		return fmt.Errorf("verify PostgreSQL runtime identity: %w", err)
	}
	if superuser || bypassRLS || ownsTables {
		return fmt.Errorf("PostgreSQL identity %q can bypass row-level security", currentUser)
	}
	if !inherit || !member || groupExists || !schemaExists || !schemaUsage {
		return fmt.Errorf("PostgreSQL identity %q is not a provisioned ITEMBA-Z runtime login", currentUser)
	}
	return nil
}

func New(pool *pgxpool.Pool, schema string) (*Store, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	if !schemaPattern.MatchString(schema) {
		return nil, fmt.Errorf("invalid PostgreSQL schema %q", schema)
	}
	return &Store{pool: pool, schema: schema, searchPath: pgx.Identifier{schema}.Sanitize() + ", public"}, nil
}

func (s *Store) Close()              { s.pool.Close() }
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
func (s *Store) Schema() string      { return s.schema }

func (s *Store) WithTransaction(ctx context.Context, fn func(sales.Transaction) error) error {
	if fn == nil {
		return errors.New("transaction callback is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite})
	if err != nil {
		return fmt.Errorf("begin PostgreSQL transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+s.searchPath); err != nil {
		return fmt.Errorf("set transaction schema: %w", err)
	}
	adapter := &transaction{tx: tx}
	if err := fn(adapter); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return normalizeError(err)
	}
	return nil
}

type transaction struct {
	tx       pgx.Tx
	tenantID string
}

func (t *transaction) ensureScope(ctx context.Context, scope tenancy.Scope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if t.tenantID != "" {
		if t.tenantID != scope.TenantID {
			return sales.ErrForbidden
		}
		return nil
	}
	if _, err := t.tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, scope.TenantID); err != nil {
		return fmt.Errorf("set tenant security context: %w", err)
	}
	t.tenantID = scope.TenantID
	return nil
}

func normalizeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return sales.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			if postgresError.ConstraintName == "one_reversal_per_sale" {
				return sales.ErrAlreadyReversed
			}
		case "40001", "40P01":
			return fmt.Errorf("retryable database transaction conflict: %w", err)
		}
	}
	return err
}
