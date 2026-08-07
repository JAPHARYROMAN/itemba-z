SET search_path TO itembaz, public;

REVOKE EXECUTE ON FUNCTION pending_outbox_tenant_ids() FROM itembaz_worker_runtime;
REVOKE ALL ON outbox_events FROM itembaz_worker_runtime;
REVOKE USAGE ON SCHEMA itembaz FROM itembaz_worker_runtime;
DROP FUNCTION pending_outbox_tenant_ids();

-- Restore the exact privilege state established by migration 000003.
GRANT SELECT ON tenants TO itembaz_runtime;
GRANT SELECT ON outbox_events TO itembaz_runtime;
GRANT UPDATE (
    available_at,
    processed_at,
    attempts,
    last_error,
    locked_by,
    locked_until
) ON outbox_events TO itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz GRANT SELECT ON TABLES TO itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz GRANT EXECUTE ON FUNCTIONS TO itembaz_runtime;
ALTER DEFAULT PRIVILEGES GRANT EXECUTE ON FUNCTIONS TO PUBLIC;

-- The database-wide NOLOGIN worker group is retained because provisioned
-- login roles may still reference it.
