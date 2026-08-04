SET search_path TO itembaz, public;

CREATE TABLE offline_posting_policies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    accounting_time_basis text NOT NULL CHECK (accounting_time_basis = 'SERVER_RECEIPT'),
    maximum_future_skew_seconds bigint NOT NULL CHECK (maximum_future_skew_seconds BETWEEN 0 AND 86400),
    require_same_fiscal_period boolean NOT NULL CHECK (require_same_fiscal_period),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id, company_id, effective_from),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

ALTER TABLE sales
    ADD COLUMN document_at timestamptz,
    ADD COLUMN received_at timestamptz,
    ADD COLUMN accounting_at timestamptz,
    ADD COLUMN accounting_time_basis text;

-- The migration backfills newly explicit timing evidence on immutable posted
-- rows. Normal runtime mutations remain protected before and after this block.
ALTER TABLE sales DISABLE TRIGGER sales_immutable;
UPDATE sales
SET document_at = COALESCE(client_timestamp, created_at),
    received_at = created_at,
    accounting_at = created_at,
    accounting_time_basis = 'SERVER_RECEIPT';
ALTER TABLE sales ENABLE TRIGGER sales_immutable;

ALTER TABLE sales
    ALTER COLUMN document_at SET NOT NULL,
    ALTER COLUMN received_at SET NOT NULL,
    ALTER COLUMN accounting_at SET NOT NULL,
    ALTER COLUMN accounting_time_basis SET NOT NULL,
    ADD CONSTRAINT sales_accounting_time_basis_check CHECK (accounting_time_basis = 'SERVER_RECEIPT'),
    ADD CONSTRAINT sales_accounting_receipt_time_check CHECK (accounting_at = received_at);

ALTER TABLE offline_posting_policies ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON offline_posting_policies
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE TRIGGER offline_posting_policies_append_only
BEFORE UPDATE OR DELETE ON offline_posting_policies
FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON offline_posting_policies FROM PUBLIC, itembaz_worker_runtime;
GRANT SELECT ON offline_posting_policies TO itembaz_runtime;

ALTER TABLE mobile_reconciliation_cases
    DROP CONSTRAINT mobile_reconciliation_cases_failure_code_check,
    ADD CONSTRAINT mobile_reconciliation_cases_failure_code_check CHECK (failure_code IN (
        'offline_reconciliation_required',
        'offline_fiscal_period_reconciliation_required',
        'offline_clock_reconciliation_required'
    ));
