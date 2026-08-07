// Package dbrole provisions a non-owner PostgreSQL login for application
// processes. Schema migrations remain the responsibility of an administrative
// database identity.
package dbrole

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Capability string

const (
	CapabilityAPI      Capability = "api"
	CapabilityWorker   Capability = "worker"
	APIRuntimeGroup               = "itembaz_runtime"
	WorkerRuntimeGroup            = "itembaz_worker_runtime"
)

var loginPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func (capability Capability) group() (required, forbidden string, err error) {
	switch capability {
	case CapabilityAPI:
		return APIRuntimeGroup, WorkerRuntimeGroup, nil
	case CapabilityWorker:
		return WorkerRuntimeGroup, APIRuntimeGroup, nil
	default:
		return "", "", errors.New("ITEMBA_RUNTIME_CAPABILITY must be api or worker")
	}
}

func ValidateLogin(username, password string, capability Capability) error {
	if _, _, err := capability.group(); err != nil {
		return err
	}
	if !loginPattern.MatchString(username) || username == APIRuntimeGroup || username == WorkerRuntimeGroup {
		return errors.New("ITEMBA_RUNTIME_USER must be a safe dedicated PostgreSQL login name")
	}
	if len(password) < 16 {
		return errors.New("ITEMBA_RUNTIME_PASSWORD must contain at least 16 characters")
	}
	return nil
}

func Provision(ctx context.Context, pool *pgxpool.Pool, username, password string, capability Capability) error {
	username = strings.TrimSpace(username)
	if pool == nil {
		return errors.New("administrative PostgreSQL pool is required")
	}
	requiredGroup, forbiddenGroup, err := capability.group()
	if err != nil {
		return err
	}
	if err := ValidateLogin(username, password, capability); err != nil {
		return err
	}
	var administrator string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&administrator); err != nil {
		return fmt.Errorf("read administrative database identity: %w", err)
	}
	if username == administrator {
		return errors.New("runtime login must differ from the migration identity")
	}
	var readyGroups int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_roles
		WHERE rolname = ANY($1::text[]) AND NOT rolcanlogin AND NOT rolsuper AND NOT rolbypassrls`,
		[]string{APIRuntimeGroup, WorkerRuntimeGroup}).Scan(&readyGroups); err != nil {
		return fmt.Errorf("read runtime capability roles: %w", err)
	}
	if readyGroups != 2 {
		return errors.New("runtime capability roles are not safely configured; apply all migrations first")
	}
	var quotedPassword string
	if err := pool.QueryRow(ctx, `SELECT quote_literal($1::text)`, password).Scan(&quotedPassword); err != nil {
		return fmt.Errorf("prepare runtime login credential: %w", err)
	}
	identifier := pgx.Identifier{username}.Sanitize()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, username).Scan(&exists); err != nil {
		return fmt.Errorf("inspect runtime login: %w", err)
	}
	statement := `CREATE ROLE ` + identifier
	if exists {
		statement = `ALTER ROLE ` + identifier
	}
	statement += ` WITH LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD ` + quotedPassword
	if _, err := pool.Exec(ctx, statement); err != nil {
		return fmt.Errorf("configure runtime login: %w", err)
	}
	if _, err := pool.Exec(ctx, `REVOKE `+pgx.Identifier{APIRuntimeGroup}.Sanitize()+`, `+
		pgx.Identifier{WorkerRuntimeGroup}.Sanitize()+` FROM `+identifier); err != nil {
		return fmt.Errorf("clear runtime role memberships: %w", err)
	}
	if _, err := pool.Exec(ctx, `GRANT `+pgx.Identifier{requiredGroup}.Sanitize()+` TO `+identifier); err != nil {
		return fmt.Errorf("grant runtime role membership: %w", err)
	}
	var login, inherit, superuser, bypass, requiredMember, forbiddenMember bool
	if err := pool.QueryRow(ctx, `
		SELECT rolcanlogin, rolinherit, rolsuper, rolbypassrls,
		       pg_has_role(rolname, $2, 'MEMBER'), pg_has_role(rolname, $3, 'MEMBER')
		FROM pg_roles WHERE rolname=$1`, username, requiredGroup, forbiddenGroup).Scan(
		&login, &inherit, &superuser, &bypass, &requiredMember, &forbiddenMember); err != nil {
		return fmt.Errorf("verify runtime login: %w", err)
	}
	if !login || !inherit || superuser || bypass || !requiredMember || forbiddenMember {
		return errors.New("runtime login verification failed")
	}
	return nil
}
