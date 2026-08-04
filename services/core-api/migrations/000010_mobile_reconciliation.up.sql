SET search_path TO itembaz, public;

INSERT INTO permissions (code, description) VALUES
    ('mobile.reconciliation.read', 'View offline-sale reconciliation cases in an assigned scope'),
    ('mobile.reconciliation.resolve', 'Record an approved resolution for an offline-sale reconciliation case');

CREATE TABLE mobile_reconciliation_cases (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    device_id uuid NOT NULL,
    client_transaction_id uuid NOT NULL,
    client_timestamp timestamptz NOT NULL,
    app_version text NOT NULL CHECK (length(btrim(app_version)) BETWEEN 1 AND 64),
    master_data_version bigint NOT NULL CHECK (master_data_version > 0),
    price_version bigint NOT NULL CHECK (price_version > 0),
    catalog_snapshot_token uuid NOT NULL,
    failure_code text NOT NULL CHECK (failure_code = 'offline_reconciliation_required'),
    command jsonb NOT NULL CHECK (jsonb_typeof(command) = 'object'),
    command_hash char(64) NOT NULL CHECK (command_hash ~ '^[0-9a-f]{64}$'),
    created_by uuid NOT NULL,
    correlation_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id, company_id, branch_id, warehouse_id, id),
    UNIQUE (tenant_id, company_id, branch_id, warehouse_id, device_id, client_transaction_id),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, device_id)
        REFERENCES mobile_devices(tenant_id, company_id, branch_id, warehouse_id, id),
    CHECK (catalog_snapshot_token <> '00000000-0000-0000-0000-000000000000')
);

CREATE TABLE mobile_reconciliation_resolutions (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    case_id uuid NOT NULL,
    action text NOT NULL CHECK (action IN ('CASH_REFUNDED', 'POSTED_EXTERNALLY', 'DUPLICATE_CONFIRMED')),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    external_reference text CHECK (external_reference IS NULL OR length(btrim(external_reference)) BETWEEN 1 AND 200),
    resolved_by uuid NOT NULL,
    correlation_id uuid NOT NULL,
    resolved_at timestamptz NOT NULL,
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    request_hash char(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    UNIQUE (tenant_id, company_id, branch_id, warehouse_id, case_id),
    UNIQUE (tenant_id, company_id, idempotency_key),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, case_id)
        REFERENCES mobile_reconciliation_cases(tenant_id, company_id, branch_id, warehouse_id, id),
    CHECK (action <> 'POSTED_EXTERNALLY' OR external_reference IS NOT NULL)
);

CREATE INDEX mobile_reconciliation_cases_scope_created
    ON mobile_reconciliation_cases (tenant_id, company_id, branch_id, warehouse_id, id);

ALTER TABLE mobile_reconciliation_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE mobile_reconciliation_resolutions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mobile_reconciliation_cases
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON mobile_reconciliation_resolutions
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE TRIGGER mobile_reconciliation_cases_append_only
BEFORE UPDATE OR DELETE ON mobile_reconciliation_cases
FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER mobile_reconciliation_resolutions_append_only
BEFORE UPDATE OR DELETE ON mobile_reconciliation_resolutions
FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON mobile_reconciliation_cases FROM PUBLIC, itembaz_worker_runtime;
REVOKE ALL ON mobile_reconciliation_resolutions FROM PUBLIC, itembaz_worker_runtime;
GRANT SELECT, INSERT ON mobile_reconciliation_cases TO itembaz_runtime;
GRANT SELECT, INSERT ON mobile_reconciliation_resolutions TO itembaz_runtime;
