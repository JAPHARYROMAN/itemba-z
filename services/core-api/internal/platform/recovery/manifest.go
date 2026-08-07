// Package recovery produces deterministic database reconciliation manifests
// for isolated backup/restore exercises. It is not a backup implementation.
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

var requiredTables = []string{
	"journals",
	"journal_lines",
	"inventory_stock_ledger",
	"customer_ledger",
	"supplier_ledger",
	"audit_events",
	"outbox_events",
	"integration_deliveries",
	"employee_documents",
	"payroll_export_artifacts",
	"report_exports",
}

type TableDigest struct {
	Rows   uint64 `json:"rows"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion      int                    `json:"schema_version"`
	MigrationRows      uint64                 `json:"migration_rows"`
	MigrationSHA256    string                 `json:"migration_sha256"`
	SequenceRows       uint64                 `json:"sequence_rows"`
	SequenceSHA256     string                 `json:"sequence_sha256"`
	AuthoritativeTable map[string]TableDigest `json:"authoritative_tables"`
}

func Capture(ctx context.Context, pool *pgxpool.Pool, schema string) (Manifest, error) {
	if pool == nil {
		return Manifest{}, errors.New("recovery manifest pool is required")
	}
	if !identifierPattern.MatchString(schema) {
		return Manifest{}, fmt.Errorf("invalid recovery schema %q", schema)
	}
	manifest := Manifest{SchemaVersion: 1, AuthoritativeTable: make(map[string]TableDigest, len(requiredTables))}
	migrations, err := pool.Query(ctx, `SELECT version || ':' || checksum FROM public.itembaz_schema_migrations ORDER BY version`)
	if err != nil {
		return Manifest{}, fmt.Errorf("query migration ledger: %w", err)
	}
	manifest.MigrationRows, manifest.MigrationSHA256, err = digestRows(migrations)
	if err != nil {
		return Manifest{}, fmt.Errorf("digest migration ledger: %w", err)
	}
	sequences, err := pool.Query(ctx, `SELECT sequencename || ':' || COALESCE(last_value::text,'NULL') FROM pg_sequences WHERE schemaname=$1 ORDER BY sequencename`, schema)
	if err != nil {
		return Manifest{}, fmt.Errorf("query sequence state: %w", err)
	}
	manifest.SequenceRows, manifest.SequenceSHA256, err = digestRows(sequences)
	if err != nil {
		return Manifest{}, fmt.Errorf("digest sequence state: %w", err)
	}
	for _, table := range requiredTables {
		qualified := pgx.Identifier{schema, table}.Sanitize()
		rows, queryErr := pool.Query(ctx, `SELECT row_to_json(recovery_row)::text FROM (SELECT * FROM `+qualified+` ORDER BY id) AS recovery_row`)
		if queryErr != nil {
			return Manifest{}, fmt.Errorf("query %s: %w", table, queryErr)
		}
		count, digest, digestErr := digestRows(rows)
		if digestErr != nil {
			return Manifest{}, fmt.Errorf("digest %s: %w", table, digestErr)
		}
		manifest.AuthoritativeTable[table] = TableDigest{Rows: count, SHA256: digest}
	}
	return manifest, nil
}

type textRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}

func digestRows(rows textRows) (uint64, string, error) {
	defer rows.Close()
	hash := sha256.New()
	var count uint64
	var length [8]byte
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return 0, "", err
		}
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}
	return count, hex.EncodeToString(hash.Sum(nil)), nil
}
