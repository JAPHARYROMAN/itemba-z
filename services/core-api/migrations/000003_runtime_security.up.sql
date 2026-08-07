SET search_path TO itembaz, public;

-- Preserve the original POS facts independently from the authoritative server
-- posting timestamp. Existing pre-migration mobile rows are intentionally not
-- validated because their original client facts cannot be reconstructed.
ALTER TABLE sales
    ADD COLUMN client_timestamp timestamptz,
    ADD COLUMN client_app_version text,
    ADD COLUMN client_master_data_version bigint,
    ADD COLUMN client_price_version bigint,
    ADD CONSTRAINT sales_mobile_provenance_required CHECK (
        device_id IS NULL OR (
            client_timestamp IS NOT NULL AND
            length(btrim(client_app_version)) > 0 AND
            client_master_data_version > 0 AND
            client_price_version > 0
        )
    ) NOT VALID;

-- Application processes inherit this NOLOGIN role. Migration and fixture
-- commands intentionally continue to use a separate administrative identity.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'itembaz_runtime') THEN
        CREATE ROLE itembaz_runtime
            NOLOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    ELSE
        ALTER ROLE itembaz_runtime
            NOLOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    END IF;
END;
$$;

REVOKE ALL ON SCHEMA itembaz FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA itembaz FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA itembaz FROM PUBLIC;
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA itembaz FROM PUBLIC;

GRANT USAGE ON SCHEMA itembaz TO itembaz_runtime;
GRANT SELECT ON ALL TABLES IN SCHEMA itembaz TO itembaz_runtime;
GRANT SELECT ON tenants TO itembaz_runtime;
GRANT INSERT ON
    idempotency_keys,
    sales,
    sale_lines,
    inventory_stock_ledger,
    customer_ledger,
    payments,
    journals,
    journal_lines,
    audit_events,
    outbox_events,
    mobile_devices
TO itembaz_runtime;
GRANT UPDATE (result_id, completed_at) ON idempotency_keys TO itembaz_runtime;
GRANT UPDATE (status, reversed_at) ON sales TO itembaz_runtime;
GRANT UPDATE (
    device_name,
    app_version,
    master_data_version,
    price_version,
    last_seen_at
) ON mobile_devices TO itembaz_runtime;
GRANT UPDATE (
    available_at,
    processed_at,
    attempts,
    last_error,
    locked_by,
    locked_until
) ON outbox_events TO itembaz_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA itembaz TO itembaz_runtime;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA itembaz TO itembaz_runtime;

ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE ALL ON SEQUENCES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz GRANT SELECT ON TABLES TO itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz GRANT USAGE, SELECT ON SEQUENCES TO itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz GRANT EXECUTE ON FUNCTIONS TO itembaz_runtime;
