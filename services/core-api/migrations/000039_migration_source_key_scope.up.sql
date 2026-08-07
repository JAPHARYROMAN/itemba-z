SET search_path TO itembaz, public;

ALTER TABLE migration_stage_rows
DROP CONSTRAINT migration_stage_rows_batch_id_source_file_id_source_key_has_key;

ALTER TABLE migration_stage_rows
ADD CONSTRAINT migration_stage_rows_batch_source_key_unique UNIQUE (batch_id, source_key_hash);
