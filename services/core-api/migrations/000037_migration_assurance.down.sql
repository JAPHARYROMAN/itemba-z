SET search_path TO itembaz, public;

DROP FUNCTION IF EXISTS transition_migration_batch(uuid, text, text, text, char);
DROP TRIGGER IF EXISTS migration_batches_guard ON migration_batches;
DROP FUNCTION IF EXISTS guard_migration_batch_update();
DROP TRIGGER IF EXISTS migration_batch_transitions_append_only ON migration_batch_transitions;
DROP TRIGGER IF EXISTS migration_reconciliations_append_only ON migration_reconciliations;
DROP TRIGGER IF EXISTS migration_control_totals_append_only ON migration_control_totals;
DROP TRIGGER IF EXISTS migration_stage_rows_append_only ON migration_stage_rows;
DROP TRIGGER IF EXISTS migration_source_files_append_only ON migration_source_files;
DROP FUNCTION IF EXISTS reject_migration_fact_mutation();
DROP TABLE IF EXISTS migration_batch_transitions;
DROP TABLE IF EXISTS migration_reconciliations;
DROP TABLE IF EXISTS migration_control_totals;
DROP TABLE IF EXISTS migration_stage_rows;
DROP TABLE IF EXISTS migration_source_files;
DROP TABLE IF EXISTS migration_batches;
