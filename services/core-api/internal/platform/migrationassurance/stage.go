package migrationassurance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StageReport struct {
	SchemaVersion  int    `json:"schema_version"`
	BatchID        string `json:"batch_id"`
	Mode           string `json:"mode"`
	Status         string `json:"status"`
	ManifestSHA256 string `json:"manifest_sha256"`
	EvidenceSHA256 string `json:"evidence_sha256"`
	SourceFiles    int    `json:"source_files"`
	StagedRows     int64  `json:"staged_rows"`
}

func Stage(ctx context.Context, pool *pgxpool.Pool, batch ValidatedBatch) (StageReport, error) {
	if pool == nil {
		return StageReport{}, errors.New("migration staging pool is required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return StageReport{}, fmt.Errorf("begin migration staging: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	m := batch.Manifest
	tag, err := tx.Exec(ctx, `INSERT INTO itembaz.migration_batches (
		id, schema_version, mode, trial_number, manifest_sha256, mapping_version,
		cutoff_at, actor_reference
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	ON CONFLICT (id) DO NOTHING`,
		m.BatchID, m.SchemaVersion, m.Mode, m.TrialNumber, batch.Report.ManifestSHA256,
		m.MappingVersion, m.CutoffAt, m.ActorReference)
	if err != nil {
		return StageReport{}, fmt.Errorf("register migration batch: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var mode, status, manifestSHA string
		var sourceFiles int
		var stagedRows int64
		err = tx.QueryRow(ctx, `SELECT b.mode, b.status, b.manifest_sha256,
			(SELECT count(*) FROM itembaz.migration_source_files f WHERE f.batch_id=b.id),
			(SELECT count(*) FROM itembaz.migration_stage_rows r WHERE r.batch_id=b.id)
		 FROM itembaz.migration_batches b WHERE b.id=$1`, m.BatchID).Scan(
			&mode, &status, &manifestSHA, &sourceFiles, &stagedRows,
		)
		if err != nil {
			return StageReport{}, fmt.Errorf("read existing migration batch: %w", err)
		}
		if mode != m.Mode || manifestSHA != batch.Report.ManifestSHA256 || sourceFiles != len(m.Sources) || stagedRows != batch.Report.TotalRows || status == "REGISTERED" || status == "REJECTED" {
			return StageReport{}, errors.New("migration batch ID already exists with mismatched or incomplete facts")
		}
		if err := tx.Commit(ctx); err != nil {
			return StageReport{}, fmt.Errorf("commit idempotent migration staging replay: %w", err)
		}
		return StageReport{
			SchemaVersion: 1, BatchID: m.BatchID, Mode: m.Mode, Status: status,
			ManifestSHA256: batch.Report.ManifestSHA256, EvidenceSHA256: batch.Report.EvidenceSHA256,
			SourceFiles: sourceFiles, StagedRows: stagedRows,
		}, nil
	}

	var stagedRows int64
	for _, source := range m.Sources {
		_, err = tx.Exec(ctx, `INSERT INTO itembaz.migration_source_files (
			id, batch_id, category, file_name, sha256, byte_count, row_count,
			classification, owner_reference
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, source.ID, m.BatchID, source.Category,
			source.File, source.SHA256, source.ByteCount, source.ExpectedRows,
			source.Classification, source.OwnerReference)
		if err != nil {
			return StageReport{}, fmt.Errorf("register source %s: %w", source.ID, err)
		}
		_, err = validateSource(batch.SourceRoot, m.BatchID, source, func(rowNumber int64, rowHash, keyHash string, row CanonicalRow) error {
			record, marshalErr := json.Marshal(row.Record)
			if marshalErr != nil {
				return fmt.Errorf("normalize source %s row %d: %w", source.ID, rowNumber, marshalErr)
			}
			_, insertErr := tx.Exec(ctx, `INSERT INTO itembaz.migration_stage_rows (
				batch_id, source_file_id, row_number, row_sha256, source_key_hash,
				tenant_key, company_key, normalized_record, validation_status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'VALID')`, m.BatchID, source.ID, rowNumber,
				rowHash, keyHash, row.TenantKey, row.CompanyKey, record)
			if insertErr == nil {
				stagedRows++
			}
			return insertErr
		})
		if err != nil {
			return StageReport{}, fmt.Errorf("stage source %s: %w", source.ID, err)
		}
	}
	for _, total := range m.ControlTotals {
		_, err = tx.Exec(ctx, `INSERT INTO itembaz.migration_control_totals (
			batch_id, domain, metric, unit, expected_value
		) VALUES ($1,$2,$3,$4,$5)`, m.BatchID, total.Domain, total.Metric, total.Unit, total.ExpectedValue)
		if err != nil {
			return StageReport{}, fmt.Errorf("stage control total %s/%s: %w", total.Domain, total.Metric, err)
		}
	}
	_, err = tx.Exec(ctx, `SELECT itembaz.transition_migration_batch($1,'REGISTERED','STAGED',$2,$3)`,
		m.BatchID, m.ActorReference, batch.Report.EvidenceSHA256)
	if err != nil {
		return StageReport{}, fmt.Errorf("transition migration batch to staged: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return StageReport{}, fmt.Errorf("commit migration staging: %w", err)
	}
	return StageReport{
		SchemaVersion: 1, BatchID: m.BatchID, Mode: m.Mode, Status: "STAGED",
		ManifestSHA256: batch.Report.ManifestSHA256, EvidenceSHA256: batch.Report.EvidenceSHA256,
		SourceFiles: len(m.Sources), StagedRows: stagedRows,
	}, nil
}
