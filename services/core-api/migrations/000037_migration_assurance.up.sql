SET search_path TO itembaz, public;

CREATE TABLE migration_batches (
    id uuid PRIMARY KEY,
    schema_version integer NOT NULL CHECK (schema_version = 1),
    mode text NOT NULL CHECK (mode IN ('SYNTHETIC_REHEARSAL', 'PRODUCTION_TRIAL', 'FINAL_CUTOVER')),
    trial_number integer CHECK (
        (mode IN ('SYNTHETIC_REHEARSAL', 'PRODUCTION_TRIAL') AND trial_number IN (1, 2))
        OR (mode = 'FINAL_CUTOVER' AND trial_number IS NULL)
    ),
    manifest_sha256 char(64) NOT NULL CHECK (manifest_sha256 ~ '^[0-9a-f]{64}$'),
    mapping_version text NOT NULL CHECK (mapping_version ~ '^[A-Za-z0-9._-]{1,64}$'),
    cutoff_at timestamptz NOT NULL,
    actor_reference text NOT NULL CHECK (length(btrim(actor_reference)) BETWEEN 3 AND 200),
    status text NOT NULL DEFAULT 'REGISTERED' CHECK (status IN (
        'REGISTERED', 'STAGED', 'VALIDATED', 'APPROVED', 'APPLIED', 'RECONCILED', 'REJECTED'
    )),
    created_at timestamptz NOT NULL DEFAULT now(),
    status_changed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (manifest_sha256)
);

CREATE TABLE migration_source_files (
    id uuid PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES migration_batches(id),
    category text NOT NULL CHECK (category IN (
        'ORGANIZATIONS', 'USERS', 'CUSTOMERS', 'SUPPLIERS', 'PRODUCTS', 'UNITS',
        'PRICES', 'TAX_MAPPINGS', 'ACCOUNTS', 'EMPLOYEES', 'HISTORICAL_SUMMARIES',
        'OPEN_DOCUMENTS', 'OPEN_BALANCES'
    )),
    file_name text NOT NULL CHECK (file_name !~ '[/\\]' AND length(file_name) BETWEEN 1 AND 240),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    byte_count bigint NOT NULL CHECK (byte_count >= 0),
    row_count bigint NOT NULL CHECK (row_count >= 0),
    classification text NOT NULL CHECK (classification IN ('INTERNAL', 'CONFIDENTIAL', 'RESTRICTED')),
    owner_reference text NOT NULL CHECK (length(btrim(owner_reference)) BETWEEN 3 AND 200),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (batch_id, file_name),
    UNIQUE (batch_id, sha256)
);

CREATE TABLE migration_stage_rows (
    id bigserial PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES migration_batches(id),
    source_file_id uuid NOT NULL REFERENCES migration_source_files(id),
    row_number bigint NOT NULL CHECK (row_number > 0),
    row_sha256 char(64) NOT NULL CHECK (row_sha256 ~ '^[0-9a-f]{64}$'),
    source_key_hash char(64) NOT NULL CHECK (source_key_hash ~ '^[0-9a-f]{64}$'),
    tenant_key text NOT NULL CHECK (length(btrim(tenant_key)) BETWEEN 1 AND 200),
    company_key text NOT NULL CHECK (length(btrim(company_key)) BETWEEN 1 AND 200),
    normalized_record jsonb NOT NULL CHECK (jsonb_typeof(normalized_record) = 'object'),
    validation_status text NOT NULL CHECK (validation_status IN ('VALID', 'INVALID', 'DUPLICATE', 'BLOCKED')),
    error_codes text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_file_id, row_number),
    UNIQUE (batch_id, source_file_id, source_key_hash)
);

CREATE TABLE migration_control_totals (
    id bigserial PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES migration_batches(id),
    domain text NOT NULL,
    metric text NOT NULL,
    unit text NOT NULL CHECK (unit IN ('COUNT', 'MINOR_UNITS', 'QUANTITY', 'DAYS')),
    expected_value numeric(30, 6) NOT NULL,
    actual_value numeric(30, 6),
    evidence_sha256 char(64) CHECK (evidence_sha256 IS NULL OR evidence_sha256 ~ '^[0-9a-f]{64}$'),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (batch_id, domain, metric, unit)
);

CREATE TABLE migration_reconciliations (
    id bigserial PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES migration_batches(id),
    domain text NOT NULL CHECK (domain IN (
        'STOCK', 'ACCOUNTS_RECEIVABLE', 'ACCOUNTS_PAYABLE', 'CASH', 'BANK',
        'MOBILE_MONEY', 'EMPLOYEE_LOANS', 'LEAVE', 'PAYROLL', 'TRIAL_BALANCE'
    )),
    expected_value numeric(30, 6) NOT NULL,
    actual_value numeric(30, 6) NOT NULL,
    unit text NOT NULL CHECK (unit IN ('COUNT', 'MINOR_UNITS', 'QUANTITY', 'DAYS')),
    passed boolean GENERATED ALWAYS AS (expected_value = actual_value) STORED,
    evidence_sha256 char(64) NOT NULL CHECK (evidence_sha256 ~ '^[0-9a-f]{64}$'),
    approver_reference text NOT NULL CHECK (length(btrim(approver_reference)) BETWEEN 3 AND 200),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (batch_id, domain)
);

CREATE TABLE migration_batch_transitions (
    id bigserial PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES migration_batches(id),
    from_status text NOT NULL,
    to_status text NOT NULL,
    actor_reference text NOT NULL CHECK (length(btrim(actor_reference)) BETWEEN 3 AND 200),
    evidence_sha256 char(64) NOT NULL CHECK (evidence_sha256 ~ '^[0-9a-f]{64}$'),
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION reject_migration_fact_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'migration assurance facts are append-only';
END;
$$;

CREATE TRIGGER migration_source_files_append_only BEFORE UPDATE OR DELETE ON migration_source_files
FOR EACH ROW EXECUTE FUNCTION reject_migration_fact_mutation();
CREATE TRIGGER migration_stage_rows_append_only BEFORE UPDATE OR DELETE ON migration_stage_rows
FOR EACH ROW EXECUTE FUNCTION reject_migration_fact_mutation();
CREATE TRIGGER migration_control_totals_append_only BEFORE UPDATE OR DELETE ON migration_control_totals
FOR EACH ROW EXECUTE FUNCTION reject_migration_fact_mutation();
CREATE TRIGGER migration_reconciliations_append_only BEFORE UPDATE OR DELETE ON migration_reconciliations
FOR EACH ROW EXECUTE FUNCTION reject_migration_fact_mutation();
CREATE TRIGGER migration_batch_transitions_append_only BEFORE UPDATE OR DELETE ON migration_batch_transitions
FOR EACH ROW EXECUTE FUNCTION reject_migration_fact_mutation();

CREATE FUNCTION guard_migration_batch_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
	IF TG_OP = 'DELETE' THEN
		RAISE EXCEPTION 'migration batches cannot be deleted';
	END IF;
    IF (NEW.id, NEW.schema_version, NEW.mode, NEW.trial_number, NEW.manifest_sha256,
        NEW.mapping_version, NEW.cutoff_at, NEW.actor_reference, NEW.created_at)
       IS DISTINCT FROM
       (OLD.id, OLD.schema_version, OLD.mode, OLD.trial_number, OLD.manifest_sha256,
        OLD.mapping_version, OLD.cutoff_at, OLD.actor_reference, OLD.created_at) THEN
        RAISE EXCEPTION 'migration batch identity and source facts are immutable';
    END IF;
    IF NEW.status_changed_at <= OLD.status_changed_at THEN
        RAISE EXCEPTION 'migration batch status timestamp must advance';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER migration_batches_guard BEFORE UPDATE OR DELETE ON migration_batches
FOR EACH ROW EXECUTE FUNCTION guard_migration_batch_update();

CREATE FUNCTION transition_migration_batch(
    p_batch_id uuid,
    p_expected_status text,
    p_next_status text,
    p_actor_reference text,
    p_evidence_sha256 char(64)
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    allowed boolean;
    batch_mode text;
    batch_mapping_version text;
BEGIN
    allowed := CASE
        WHEN p_expected_status = 'REGISTERED' AND p_next_status IN ('STAGED', 'REJECTED') THEN true
        WHEN p_expected_status = 'STAGED' AND p_next_status IN ('VALIDATED', 'REJECTED') THEN true
        WHEN p_expected_status = 'VALIDATED' AND p_next_status IN ('APPROVED', 'REJECTED') THEN true
        WHEN p_expected_status = 'APPROVED' AND p_next_status IN ('APPLIED', 'REJECTED') THEN true
        WHEN p_expected_status = 'APPLIED' AND p_next_status IN ('RECONCILED', 'REJECTED') THEN true
        ELSE false
    END;
    IF NOT allowed THEN
        RAISE EXCEPTION 'invalid migration transition % -> %', p_expected_status, p_next_status;
    END IF;
    IF p_evidence_sha256 !~ '^[0-9a-f]{64}$' THEN
        RAISE EXCEPTION 'invalid migration evidence checksum';
    END IF;

    SELECT mode, mapping_version INTO batch_mode, batch_mapping_version
      FROM migration_batches
     WHERE id = p_batch_id AND status = p_expected_status
     FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'migration batch status mismatch';
    END IF;
    IF batch_mode = 'SYNTHETIC_REHEARSAL' AND p_next_status NOT IN ('STAGED', 'REJECTED') THEN
        RAISE EXCEPTION 'synthetic rehearsal cannot satisfy production migration gates';
    END IF;
    IF p_next_status = 'VALIDATED' AND EXISTS (
        SELECT 1 FROM migration_stage_rows
         WHERE batch_id = p_batch_id AND validation_status <> 'VALID'
    ) THEN
        RAISE EXCEPTION 'migration batch contains non-valid staged rows';
    END IF;
    IF batch_mode = 'FINAL_CUTOVER' AND p_next_status = 'VALIDATED' AND NOT (
        EXISTS (SELECT 1 FROM migration_batches WHERE mode='PRODUCTION_TRIAL' AND trial_number=1 AND status='RECONCILED' AND mapping_version=batch_mapping_version)
        AND EXISTS (SELECT 1 FROM migration_batches WHERE mode='PRODUCTION_TRIAL' AND trial_number=2 AND status='RECONCILED' AND mapping_version=batch_mapping_version)
    ) THEN
        RAISE EXCEPTION 'final cutover requires reconciled production trials 1 and 2 using the same mapping version';
    END IF;
    IF p_next_status = 'RECONCILED' AND (
        (SELECT count(*) FROM migration_reconciliations WHERE batch_id=p_batch_id) <> 10
        OR EXISTS (SELECT 1 FROM migration_reconciliations WHERE batch_id=p_batch_id AND NOT passed)
    ) THEN
        RAISE EXCEPTION 'migration batch requires ten passing reconciliations';
    END IF;

    UPDATE migration_batches
       SET status = p_next_status, status_changed_at = clock_timestamp()
     WHERE id = p_batch_id AND status = p_expected_status;
    INSERT INTO migration_batch_transitions (
        batch_id, from_status, to_status, actor_reference, evidence_sha256
    ) VALUES (
        p_batch_id, p_expected_status, p_next_status, p_actor_reference, p_evidence_sha256
    );
END;
$$;

CREATE INDEX migration_stage_rows_batch_idx ON migration_stage_rows(batch_id, validation_status);
CREATE INDEX migration_reconciliations_batch_idx ON migration_reconciliations(batch_id, passed);

REVOKE ALL ON TABLE migration_batches, migration_source_files, migration_stage_rows,
    migration_control_totals, migration_reconciliations, migration_batch_transitions FROM PUBLIC;
REVOKE ALL ON SEQUENCE migration_stage_rows_id_seq, migration_control_totals_id_seq,
    migration_reconciliations_id_seq, migration_batch_transitions_id_seq FROM PUBLIC;
REVOKE ALL ON FUNCTION transition_migration_batch(uuid, text, text, text, char) FROM PUBLIC;

COMMENT ON TABLE migration_stage_rows IS 'Privileged, restricted staging data. Never expose through runtime APIs or logs.';
COMMENT ON FUNCTION transition_migration_batch(uuid, text, text, text, char) IS 'Fail-closed migration lifecycle transition with immutable checksum evidence.';
