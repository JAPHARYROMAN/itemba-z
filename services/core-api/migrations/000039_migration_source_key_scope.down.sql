SET search_path TO itembaz, public;

ALTER TABLE migration_stage_rows
DROP CONSTRAINT migration_stage_rows_batch_source_key_unique;

ALTER TABLE migration_stage_rows
ADD CONSTRAINT migration_stage_rows_batch_id_source_file_id_source_key_hash_key
UNIQUE (batch_id, source_file_id, source_key_hash);
