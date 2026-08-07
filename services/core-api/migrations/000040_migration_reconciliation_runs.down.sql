SET search_path TO itembaz, public;

DROP TRIGGER IF EXISTS migration_reconciliation_runs_append_only ON migration_reconciliation_runs;
DROP TABLE IF EXISTS migration_reconciliation_runs;
