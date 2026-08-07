SET search_path TO itembaz, public;

-- API and outbox worker processes have separate database capabilities. The
-- API must never enumerate the global tenants table merely because the worker
-- needs to discover pending outbox partitions.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'itembaz_worker_runtime') THEN
        CREATE ROLE itembaz_worker_runtime
            NOLOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    ELSE
        ALTER ROLE itembaz_worker_runtime
            NOLOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    END IF;
END;
$$;

REVOKE ALL ON tenants FROM itembaz_runtime;
REVOKE SELECT ON outbox_events FROM itembaz_runtime;
REVOKE UPDATE (
    available_at,
    processed_at,
    attempts,
    last_error,
    locked_by,
    locked_until
) ON outbox_events FROM itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE SELECT ON TABLES FROM itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE EXECUTE ON FUNCTIONS FROM itembaz_runtime;
-- PostgreSQL's built-in function default grants PUBLIC execute globally.
-- A per-schema REVOKE cannot remove that global default, so revoke it at the
-- creating-role level and grant every callable function explicitly.
ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

CREATE OR REPLACE FUNCTION pending_outbox_tenant_ids()
RETURNS TABLE (tenant_id uuid)
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog, itembaz
AS $$
    SELECT DISTINCT event.tenant_id
    FROM itembaz.outbox_events AS event
    WHERE event.processed_at IS NULL
      AND event.available_at <= clock_timestamp()
      AND (event.locked_until IS NULL OR event.locked_until < clock_timestamp())
    ORDER BY event.tenant_id
$$;

REVOKE ALL ON FUNCTION pending_outbox_tenant_ids() FROM PUBLIC;
REVOKE ALL ON FUNCTION pending_outbox_tenant_ids() FROM itembaz_runtime;

GRANT USAGE ON SCHEMA itembaz TO itembaz_worker_runtime;
GRANT SELECT ON outbox_events TO itembaz_worker_runtime;
GRANT UPDATE (
    available_at,
    processed_at,
    attempts,
    last_error,
    locked_by,
    locked_until
) ON outbox_events TO itembaz_worker_runtime;
GRANT EXECUTE ON FUNCTION pending_outbox_tenant_ids() TO itembaz_worker_runtime;
