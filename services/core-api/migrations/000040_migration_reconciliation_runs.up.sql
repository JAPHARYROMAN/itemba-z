SET search_path TO itembaz, public;

CREATE TABLE migration_reconciliation_runs (
    batch_id uuid PRIMARY KEY REFERENCES migration_batches(id),
    file_sha256 char(64) NOT NULL UNIQUE CHECK (file_sha256 ~ '^[0-9a-f]{64}$'),
    all_passed boolean NOT NULL,
    approver_reference text NOT NULL CHECK (length(btrim(approver_reference)) BETWEEN 3 AND 200),
    recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER migration_reconciliation_runs_append_only
BEFORE UPDATE OR DELETE ON migration_reconciliation_runs
FOR EACH ROW EXECUTE FUNCTION reject_migration_fact_mutation();

REVOKE ALL ON TABLE migration_reconciliation_runs FROM PUBLIC;
