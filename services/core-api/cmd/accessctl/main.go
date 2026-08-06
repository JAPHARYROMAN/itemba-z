// Command accessctl applies approved joiner, mover, leaver, delegation,
// emergency-access, and review decisions through an administrative database
// identity. It is intentionally not linked into the API runtime.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
)

type command struct {
	action, tenantID, targetUserID, actorID, approverID    string
	assignmentID, roleID, companyID, branchID, warehouseID string
	assignmentType, sourceUserID, reason, ticket           string
	duration                                               time.Duration
}

func required(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func uuidEnv(name string, optional bool) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" && optional {
		return "", nil
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	canonical, err := identity.CanonicalUUID(value)
	if err != nil {
		return "", fmt.Errorf("%s must be a UUID", name)
	}
	return canonical, nil
}

func loadCommand() (command, error) {
	action, err := required("ITEMBA_ACCESS_ACTION")
	if err != nil {
		return command{}, err
	}
	c := command{action: action}
	for name, target := range map[string]*string{
		"ITEMBA_TENANT_ID": &c.tenantID, "ITEMBA_TARGET_USER_ID": &c.targetUserID,
		"ITEMBA_ACTOR_ID": &c.actorID, "ITEMBA_APPROVER_ID": &c.approverID,
	} {
		*target, err = uuidEnv(name, false)
		if err != nil {
			return command{}, err
		}
	}
	if c.actorID == c.approverID {
		return command{}, errors.New("request actor and approver must be different users")
	}
	if c.targetUserID == c.actorID || c.targetUserID == c.approverID {
		return command{}, errors.New("target user cannot execute or approve their own access change")
	}
	c.reason, err = required("ITEMBA_ACCESS_REASON")
	if err != nil || len(c.reason) < 8 || len(c.reason) > 500 {
		return command{}, errors.New("ITEMBA_ACCESS_REASON must contain 8 to 500 characters")
	}
	c.ticket, err = required("ITEMBA_ACCESS_TICKET")
	if err != nil || len(c.ticket) < 3 || len(c.ticket) > 200 {
		return command{}, errors.New("ITEMBA_ACCESS_TICKET must contain 3 to 200 characters")
	}
	if action == "grant-assignment" {
		for name, target := range map[string]*string{
			"ITEMBA_ROLE_ID": &c.roleID, "ITEMBA_COMPANY_ID": &c.companyID,
			"ITEMBA_BRANCH_ID": &c.branchID, "ITEMBA_WAREHOUSE_ID": &c.warehouseID,
		} {
			*target, err = uuidEnv(name, false)
			if err != nil {
				return command{}, err
			}
		}
		c.assignmentType = strings.TrimSpace(os.Getenv("ITEMBA_ASSIGNMENT_TYPE"))
		if c.assignmentType == "" {
			c.assignmentType = "STANDARD"
		}
		if c.assignmentType != "STANDARD" && c.assignmentType != "DELEGATED" && c.assignmentType != "BREAK_GLASS" {
			return command{}, errors.New("ITEMBA_ASSIGNMENT_TYPE must be STANDARD, DELEGATED, or BREAK_GLASS")
		}
		c.sourceUserID, err = uuidEnv("ITEMBA_SOURCE_USER_ID", c.assignmentType != "DELEGATED")
		if err != nil {
			return command{}, err
		}
		if raw := strings.TrimSpace(os.Getenv("ITEMBA_ACCESS_DURATION_SECONDS")); raw != "" {
			seconds, parseErr := strconv.Atoi(raw)
			if parseErr != nil || seconds <= 0 {
				return command{}, errors.New("ITEMBA_ACCESS_DURATION_SECONDS must be a positive integer")
			}
			c.duration = time.Duration(seconds) * time.Second
		}
		if c.assignmentType == "DELEGATED" && (c.duration <= 0 || c.duration > 30*24*time.Hour) {
			return command{}, errors.New("delegated access must expire within 30 days")
		}
		if c.assignmentType == "BREAK_GLASS" && (c.duration <= 0 || c.duration > 2*time.Hour) {
			return command{}, errors.New("break-glass access must expire within two hours")
		}
	}
	if action == "revoke-assignment" || action == "attest-assignment" {
		c.assignmentID, err = uuidEnv("ITEMBA_ASSIGNMENT_ID", false)
		if err != nil {
			return command{}, err
		}
	}
	if action != "enable-user" && action != "disable-user" && action != "grant-assignment" && action != "revoke-assignment" && action != "attest-assignment" {
		return command{}, errors.New("ITEMBA_ACCESS_ACTION is not supported")
	}
	return c, nil
}

func newID() (string, error) { return (identity.UUIDGenerator{}).New() }

func insertEvent(ctx context.Context, tx pgx.Tx, c command, assignmentID, eventType string, at time.Time) (string, error) {
	eventID, err := newID()
	if err != nil {
		return "", err
	}
	correlationID, err := newID()
	if err != nil {
		return "", err
	}
	evidence, _ := json.Marshal(map[string]string{"action": c.action, "assignment_type": c.assignmentType})
	_, err = tx.Exec(ctx, `INSERT INTO access_assignment_events(id,tenant_id,assignment_id,target_user_id,event_type,actor_id,approver_id,reason,ticket_reference,evidence,correlation_id,occurred_at) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, eventID, c.tenantID, assignmentID, c.targetUserID, eventType, c.actorID, c.approverID, c.reason, c.ticket, evidence, correlationID, at)
	return correlationID, err
}

func execute(ctx context.Context, pool *pgxpool.Pool, c command, at time.Time) (string, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO itembaz, public`); err != nil {
		return "", err
	}
	var activeControllers int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id=$1 AND active AND id = ANY($2::uuid[])`, c.tenantID, []string{c.actorID, c.approverID}).Scan(&activeControllers); err != nil {
		return "", err
	}
	if activeControllers != 2 {
		return "", errors.New("access actor and approver must both be active tenant users")
	}
	var assignmentID, eventType string
	switch c.action {
	case "enable-user":
		result, execErr := tx.Exec(ctx, `UPDATE users SET active=true WHERE tenant_id=$1 AND id=$2 AND NOT active`, c.tenantID, c.targetUserID)
		if execErr != nil || result.RowsAffected() != 1 {
			return "", errors.New("inactive target user was not found")
		}
		eventType = "USER_ENABLED"
	case "disable-user":
		result, execErr := tx.Exec(ctx, `UPDATE users SET active=false WHERE tenant_id=$1 AND id=$2 AND active`, c.tenantID, c.targetUserID)
		if execErr != nil || result.RowsAffected() != 1 {
			return "", errors.New("active target user was not found")
		}
		if _, execErr = tx.Exec(ctx, `UPDATE user_role_scopes SET revoked_at=$3,revoked_by=$4,revocation_reason=$5 WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL`, c.tenantID, c.targetUserID, at, c.approverID, c.reason); execErr != nil {
			return "", execErr
		}
		eventType = "USER_DISABLED"
	case "grant-assignment":
		if c.assignmentType == "DELEGATED" {
			var sourceAuthorized bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_role_scopes WHERE tenant_id=$1 AND user_id=$2 AND role_id=$3 AND company_id=$4 AND branch_id=$5 AND warehouse_id=$6 AND revoked_at IS NULL AND valid_from <= $7 AND (valid_until IS NULL OR valid_until > $7))`, c.tenantID, c.sourceUserID, c.roleID, c.companyID, c.branchID, c.warehouseID, at).Scan(&sourceAuthorized); err != nil {
				return "", err
			}
			if !sourceAuthorized {
				return "", errors.New("delegation source does not hold the active scoped role")
			}
		}
		assignmentID, err = newID()
		if err != nil {
			return "", err
		}
		var validUntil any
		if c.duration > 0 {
			validUntil = at.Add(c.duration)
		}
		_, err = tx.Exec(ctx, `INSERT INTO user_role_scopes(id,tenant_id,user_id,role_id,company_id,branch_id,warehouse_id,assignment_type,valid_from,valid_until,source_user_id,granted_by,approved_by,reason,ticket_reference) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid,$12,$13,$14,$15)`, assignmentID, c.tenantID, c.targetUserID, c.roleID, c.companyID, c.branchID, c.warehouseID, c.assignmentType, at, validUntil, c.sourceUserID, c.actorID, c.approverID, c.reason, c.ticket)
		if err != nil {
			return "", err
		}
		eventType = map[string]string{"STANDARD": "ASSIGNMENT_GRANTED", "DELEGATED": "DELEGATION_ACTIVATED", "BREAK_GLASS": "BREAK_GLASS_ACTIVATED"}[c.assignmentType]
	case "revoke-assignment":
		assignmentID = c.assignmentID
		result, execErr := tx.Exec(ctx, `UPDATE user_role_scopes SET revoked_at=$4,revoked_by=$5,revocation_reason=$6 WHERE tenant_id=$1 AND user_id=$2 AND id=$3 AND revoked_at IS NULL`, c.tenantID, c.targetUserID, assignmentID, at, c.approverID, c.reason)
		if execErr != nil || result.RowsAffected() != 1 {
			return "", errors.New("active target assignment was not found")
		}
		eventType = "ASSIGNMENT_REVOKED"
	case "attest-assignment":
		assignmentID = c.assignmentID
		result, execErr := tx.Exec(ctx, `UPDATE user_role_scopes SET last_reviewed_at=$4,last_reviewed_by=$5 WHERE tenant_id=$1 AND user_id=$2 AND id=$3 AND revoked_at IS NULL`, c.tenantID, c.targetUserID, assignmentID, at, c.approverID)
		if execErr != nil || result.RowsAffected() != 1 {
			return "", errors.New("active target assignment was not found")
		}
		eventType = "ACCESS_REVIEW_ATTESTED"
	}
	correlationID, err := insertEvent(ctx, tx, c, assignmentID, eventType, at)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correlationID, nil
}

func run() error {
	databaseURL, err := required("ITEMBA_ADMIN_DATABASE_URL")
	if err != nil {
		return err
	}
	c, err := loadCommand()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	correlationID, err := execute(ctx, pool, c, time.Now().UTC())
	if err != nil {
		return err
	}
	fmt.Printf("access governance command committed; correlation_id=%s\n", correlationID)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "access governance command rejected:", err)
		os.Exit(1)
	}
}
