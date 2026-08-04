SET search_path TO itembaz, public;

ALTER TABLE mobile_reconciliation_cases
    DROP CONSTRAINT mobile_reconciliation_cases_failure_code_check,
    ADD CONSTRAINT mobile_reconciliation_cases_failure_code_check
        CHECK (failure_code = 'offline_reconciliation_required');

ALTER TABLE sales
    DROP CONSTRAINT sales_accounting_receipt_time_check,
    DROP CONSTRAINT sales_accounting_time_basis_check,
    DROP COLUMN accounting_time_basis,
    DROP COLUMN accounting_at,
    DROP COLUMN received_at,
    DROP COLUMN document_at;

REVOKE ALL ON offline_posting_policies FROM itembaz_runtime;
DROP TABLE offline_posting_policies;
