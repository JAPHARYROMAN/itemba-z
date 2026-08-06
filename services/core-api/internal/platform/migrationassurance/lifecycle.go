package migrationassurance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var requiredReconciliationDomains = []string{
	"ACCOUNTS_PAYABLE", "ACCOUNTS_RECEIVABLE", "BANK", "CASH", "EMPLOYEE_LOANS",
	"LEAVE", "MOBILE_MONEY", "PAYROLL", "STOCK", "TRIAL_BALANCE",
}

type ReconciliationFile struct {
	SchemaVersion     int                   `json:"schema_version"`
	BatchID           string                `json:"batch_id"`
	ApproverReference string                `json:"approver_reference"`
	Entries           []ReconciliationEntry `json:"entries"`
}

type ReconciliationEntry struct {
	Domain         string `json:"domain"`
	ExpectedValue  string `json:"expected_value"`
	ActualValue    string `json:"actual_value"`
	Unit           string `json:"unit"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}

type ReconciliationReport struct {
	SchemaVersion  int    `json:"schema_version"`
	BatchID        string `json:"batch_id"`
	Status         string `json:"status"`
	EvidenceSHA256 string `json:"evidence_sha256"`
	Domains        int    `json:"domains"`
	AllPassed      bool   `json:"all_passed"`
}

type BatchEvidence struct {
	SchemaVersion          int    `json:"schema_version"`
	BatchID                string `json:"batch_id"`
	Mode                   string `json:"mode"`
	TrialNumber            *int   `json:"trial_number"`
	Status                 string `json:"status"`
	ManifestSHA256         string `json:"manifest_sha256"`
	MappingVersion         string `json:"mapping_version"`
	SourceFiles            int64  `json:"source_files"`
	StagedRows             int64  `json:"staged_rows"`
	ValidRows              int64  `json:"valid_rows"`
	ControlTotals          int64  `json:"control_totals"`
	ReconciliationRuns     int64  `json:"reconciliation_runs"`
	Reconciliations        int64  `json:"reconciliations"`
	PassingReconciliations int64  `json:"passing_reconciliations"`
}

func Transition(ctx context.Context, pool *pgxpool.Pool, batchID, expected, next, actor, evidenceSHA256 string) error {
	if pool == nil {
		return errors.New("migration lifecycle pool is required")
	}
	if !isUUID(batchID) || !validReference(actor) || !checksumPattern.MatchString(evidenceSHA256) {
		return errors.New("valid batch ID, actor reference, and evidence SHA-256 are required")
	}
	if _, err := pool.Exec(ctx, `SELECT itembaz.transition_migration_batch($1,$2,$3,$4,$5)`,
		batchID, expected, next, actor, evidenceSHA256); err != nil {
		var alreadyRecorded bool
		lookupErr := pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM itembaz.migration_batch_transitions
			 WHERE batch_id=$1 AND from_status=$2 AND to_status=$3
			   AND actor_reference=$4 AND evidence_sha256=$5
		)`, batchID, expected, next, actor, evidenceSHA256).Scan(&alreadyRecorded)
		if lookupErr == nil && alreadyRecorded {
			return nil
		}
		return fmt.Errorf("transition migration batch: %w", err)
	}
	return nil
}

func CaptureBatchEvidence(ctx context.Context, pool *pgxpool.Pool, batchID string) (BatchEvidence, error) {
	if pool == nil || !isUUID(batchID) {
		return BatchEvidence{}, errors.New("migration evidence requires a pool and batch UUID")
	}
	var evidence BatchEvidence
	evidence.SchemaVersion = 1
	err := pool.QueryRow(ctx, `SELECT b.id, b.mode, b.trial_number, b.status, b.manifest_sha256,
		b.mapping_version,
		(SELECT count(*) FROM itembaz.migration_source_files f WHERE f.batch_id=b.id),
		(SELECT count(*) FROM itembaz.migration_stage_rows r WHERE r.batch_id=b.id),
		(SELECT count(*) FROM itembaz.migration_stage_rows r WHERE r.batch_id=b.id AND r.validation_status='VALID'),
		(SELECT count(*) FROM itembaz.migration_control_totals c WHERE c.batch_id=b.id),
		(SELECT count(*) FROM itembaz.migration_reconciliation_runs q WHERE q.batch_id=b.id),
		(SELECT count(*) FROM itembaz.migration_reconciliations x WHERE x.batch_id=b.id),
		(SELECT count(*) FROM itembaz.migration_reconciliations x WHERE x.batch_id=b.id AND x.passed)
	 FROM itembaz.migration_batches b WHERE b.id=$1`, batchID).Scan(
		&evidence.BatchID, &evidence.Mode, &evidence.TrialNumber, &evidence.Status,
		&evidence.ManifestSHA256, &evidence.MappingVersion, &evidence.SourceFiles,
		&evidence.StagedRows, &evidence.ValidRows, &evidence.ControlTotals,
		&evidence.ReconciliationRuns, &evidence.Reconciliations, &evidence.PassingReconciliations,
	)
	if err != nil {
		return BatchEvidence{}, fmt.Errorf("capture migration batch evidence: %w", err)
	}
	return evidence, nil
}

func RecordReconciliation(ctx context.Context, pool *pgxpool.Pool, path string) (ReconciliationReport, error) {
	if pool == nil {
		return ReconciliationReport{}, errors.New("migration reconciliation pool is required")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("read reconciliation file: %w", err)
	}
	var file ReconciliationFile
	if err := decodeStrict(contents, &file); err != nil {
		return ReconciliationReport{}, fmt.Errorf("decode reconciliation file: %w", err)
	}
	allPassed, err := validateReconciliation(file)
	if err != nil {
		return ReconciliationReport{}, err
	}
	sum := sha256.Sum256(contents)
	evidenceSHA := hex.EncodeToString(sum[:])
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("begin reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM itembaz.migration_batches WHERE id=$1 FOR UPDATE`, file.BatchID).Scan(&status); err != nil {
		return ReconciliationReport{}, fmt.Errorf("lock migration batch: %w", err)
	}
	var priorFileSHA string
	var priorPassed bool
	err = tx.QueryRow(ctx, `SELECT file_sha256, all_passed FROM itembaz.migration_reconciliation_runs WHERE batch_id=$1`, file.BatchID).Scan(&priorFileSHA, &priorPassed)
	if err == nil {
		if priorFileSHA != evidenceSHA || priorPassed != allPassed {
			return ReconciliationReport{}, errors.New("migration batch already has different immutable reconciliation evidence")
		}
		if err := tx.Commit(ctx); err != nil {
			return ReconciliationReport{}, fmt.Errorf("commit idempotent reconciliation replay: %w", err)
		}
		return ReconciliationReport{
			SchemaVersion: 1, BatchID: file.BatchID, Status: status, EvidenceSHA256: evidenceSHA,
			Domains: len(file.Entries), AllPassed: allPassed,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ReconciliationReport{}, fmt.Errorf("read prior reconciliation: %w", err)
	}
	if status != "APPLIED" {
		return ReconciliationReport{}, fmt.Errorf("migration batch must be APPLIED, got %s", status)
	}
	for _, entry := range file.Entries {
		_, err = tx.Exec(ctx, `INSERT INTO itembaz.migration_reconciliations (
			batch_id, domain, expected_value, actual_value, unit, evidence_sha256, approver_reference
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`, file.BatchID, entry.Domain, entry.ExpectedValue,
			entry.ActualValue, entry.Unit, entry.EvidenceSHA256, file.ApproverReference)
		if err != nil {
			return ReconciliationReport{}, fmt.Errorf("record %s reconciliation: %w", entry.Domain, err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO itembaz.migration_reconciliation_runs (
		batch_id, file_sha256, all_passed, approver_reference
	) VALUES ($1,$2,$3,$4)`, file.BatchID, evidenceSHA, allPassed, file.ApproverReference)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("record reconciliation run: %w", err)
	}
	if allPassed {
		if _, err = tx.Exec(ctx, `SELECT itembaz.transition_migration_batch($1,'APPLIED','RECONCILED',$2,$3)`,
			file.BatchID, file.ApproverReference, evidenceSHA); err != nil {
			return ReconciliationReport{}, fmt.Errorf("complete migration reconciliation: %w", err)
		}
		status = "RECONCILED"
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconciliationReport{}, fmt.Errorf("commit migration reconciliation: %w", err)
	}
	return ReconciliationReport{
		SchemaVersion: 1, BatchID: file.BatchID, Status: status, EvidenceSHA256: evidenceSHA,
		Domains: len(file.Entries), AllPassed: allPassed,
	}, nil
}

func validateReconciliation(file ReconciliationFile) (bool, error) {
	if file.SchemaVersion != 1 || !isUUID(file.BatchID) || !validReference(file.ApproverReference) {
		return false, errors.New("reconciliation requires schema version 1, a batch UUID, and approver reference")
	}
	if len(file.Entries) != len(requiredReconciliationDomains) {
		return false, fmt.Errorf("reconciliation requires exactly %d domains", len(requiredReconciliationDomains))
	}
	seen := make([]string, 0, len(file.Entries))
	allPassed := true
	for _, entry := range file.Entries {
		if !validUnits[entry.Unit] || !decimalPattern.MatchString(entry.ExpectedValue) || !decimalPattern.MatchString(entry.ActualValue) || !checksumPattern.MatchString(entry.EvidenceSHA256) {
			return false, fmt.Errorf("reconciliation domain %q has invalid values or evidence", entry.Domain)
		}
		seen = append(seen, entry.Domain)
		expected, expectedOK := new(big.Rat).SetString(entry.ExpectedValue)
		actual, actualOK := new(big.Rat).SetString(entry.ActualValue)
		if !expectedOK || !actualOK {
			return false, fmt.Errorf("reconciliation domain %q has invalid decimal values", entry.Domain)
		}
		allPassed = allPassed && expected.Cmp(actual) == 0
	}
	sort.Strings(seen)
	for index, domain := range requiredReconciliationDomains {
		if seen[index] != domain {
			return false, fmt.Errorf("reconciliation domains must include each required domain exactly once")
		}
	}
	return allPassed, nil
}
