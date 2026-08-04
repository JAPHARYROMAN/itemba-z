SET search_path TO itembaz, public;

ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE SELECT ON TABLES FROM itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE USAGE, SELECT ON SEQUENCES FROM itembaz_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA itembaz REVOKE EXECUTE ON FUNCTIONS FROM itembaz_runtime;

REVOKE ALL ON ALL TABLES IN SCHEMA itembaz FROM itembaz_runtime;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA itembaz FROM itembaz_runtime;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA itembaz FROM itembaz_runtime;
REVOKE USAGE ON SCHEMA itembaz FROM itembaz_runtime;

ALTER TABLE sales
    DROP CONSTRAINT IF EXISTS sales_mobile_provenance_required,
    DROP COLUMN IF EXISTS client_price_version,
    DROP COLUMN IF EXISTS client_master_data_version,
    DROP COLUMN IF EXISTS client_app_version,
    DROP COLUMN IF EXISTS client_timestamp;

-- The database-wide NOLOGIN group role is deliberately retained because
-- other schemas or provisioned login roles may still depend on it.
