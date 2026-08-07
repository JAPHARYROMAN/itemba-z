SET search_path TO itembaz, public;

-- The device row exposes only the latest acknowledged lease to clients. Both
-- values are initialized to the same instant so upgraded devices remain
-- fail-closed until a successful cache acknowledgement issues a real lease.
ALTER TABLE mobile_devices
    ADD COLUMN offline_sales_valid_from timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN offline_sales_valid_until timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD CONSTRAINT mobile_devices_offline_sales_lease_order
        CHECK (offline_sales_valid_until >= offline_sales_valid_from);

-- Historical leases are authorization facts, not mutable device state. Exact
-- app and cache-version binding lets a queued transaction remain verifiable
-- after the device acknowledges a newer cache or application build.
CREATE TABLE mobile_device_offline_leases (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    device_id uuid NOT NULL,
    app_version text NOT NULL CHECK (length(btrim(app_version)) > 0),
    master_data_version bigint NOT NULL CHECK (master_data_version > 0),
    price_version bigint NOT NULL CHECK (price_version > 0),
    valid_from timestamptz NOT NULL,
    valid_until timestamptz NOT NULL,
    PRIMARY KEY (
        tenant_id, device_id, app_version, master_data_version,
        price_version, valid_from, valid_until
    ),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, device_id)
        REFERENCES mobile_devices(tenant_id, company_id, branch_id, warehouse_id, id),
    CHECK (valid_until > valid_from),
    CHECK (valid_until <= valid_from + interval '4 hours')
);

ALTER TABLE mobile_device_offline_leases ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mobile_device_offline_leases
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE TRIGGER mobile_device_offline_leases_append_only
BEFORE UPDATE OR DELETE ON mobile_device_offline_leases
FOR EACH ROW EXECUTE FUNCTION reject_mutation();

-- Serialize tax configuration with enrollment and reject a transition that
-- overlaps an already-issued lease. Operators must schedule the transition at
-- or after every outstanding lease deadline (or wait for leases to drain).
-- This closes both the ordinary lead-time gap and the concurrent insert/ack
-- race; the existing AFTER trigger then bumps the master-data version while
-- holding the same advisory lock.
CREATE OR REPLACE FUNCTION guard_tax_rule_against_offline_leases()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, itembaz
AS $$
DECLARE
    old_lock_key text;
    new_lock_key text;
    overlaps_lease boolean;
BEGIN
    IF TG_OP = 'UPDATE' AND
       ROW(OLD.tenant_id, OLD.company_id, OLD.code, OLD.basis_points, OLD.effective_from, OLD.effective_to)
       IS NOT DISTINCT FROM
       ROW(NEW.tenant_id, NEW.company_id, NEW.code, NEW.basis_points, NEW.effective_from, NEW.effective_to) THEN
        RETURN NEW;
    END IF;

    IF TG_OP <> 'INSERT' THEN
        old_lock_key := 'master-data-version:' || OLD.tenant_id::text || ':' || OLD.company_id::text;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        new_lock_key := 'master-data-version:' || NEW.tenant_id::text || ':' || NEW.company_id::text;
    END IF;

    IF old_lock_key IS NULL THEN
        PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(new_lock_key, 0));
    ELSIF new_lock_key IS NULL THEN
        PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(old_lock_key, 0));
    ELSE
        PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(LEAST(old_lock_key, new_lock_key), 0));
        IF old_lock_key <> new_lock_key THEN
            PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(GREATEST(old_lock_key, new_lock_key), 0));
        END IF;
    END IF;

    overlaps_lease := false;
    IF TG_OP <> 'INSERT' THEN
        SELECT EXISTS (
            SELECT 1 FROM itembaz.mobile_device_offline_leases AS lease
            WHERE lease.tenant_id = OLD.tenant_id
              AND lease.company_id = OLD.company_id
              AND lease.valid_until > pg_catalog.clock_timestamp()
              AND pg_catalog.tstzrange(lease.valid_from, lease.valid_until, '[)') &&
                  pg_catalog.tstzrange(OLD.effective_from, COALESCE(OLD.effective_to, 'infinity'::timestamptz), '[)')
        ) INTO overlaps_lease;
    END IF;
    IF NOT overlaps_lease AND TG_OP <> 'DELETE' THEN
        SELECT EXISTS (
            SELECT 1 FROM itembaz.mobile_device_offline_leases AS lease
            WHERE lease.tenant_id = NEW.tenant_id
              AND lease.company_id = NEW.company_id
              AND lease.valid_until > pg_catalog.clock_timestamp()
              AND pg_catalog.tstzrange(lease.valid_from, lease.valid_until, '[)') &&
                  pg_catalog.tstzrange(NEW.effective_from, COALESCE(NEW.effective_to, 'infinity'::timestamptz), '[)')
        ) INTO overlaps_lease;
    END IF;

    IF overlaps_lease THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            MESSAGE = 'tax rule transition overlaps an outstanding offline-sales lease',
            HINT = 'Schedule activation at or after all offline_sales_valid_until deadlines, or wait for leases to drain.';
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

REVOKE ALL ON FUNCTION guard_tax_rule_against_offline_leases() FROM PUBLIC;
REVOKE ALL ON FUNCTION guard_tax_rule_against_offline_leases() FROM itembaz_runtime;
REVOKE ALL ON FUNCTION guard_tax_rule_against_offline_leases() FROM itembaz_worker_runtime;

CREATE TRIGGER tax_rule_offline_lease_guard
BEFORE INSERT OR UPDATE OR DELETE ON tax_rules
FOR EACH ROW EXECUTE FUNCTION guard_tax_rule_against_offline_leases();

REVOKE ALL ON mobile_device_offline_leases FROM PUBLIC;
REVOKE ALL ON mobile_device_offline_leases FROM itembaz_worker_runtime;
GRANT SELECT, INSERT ON mobile_device_offline_leases TO itembaz_runtime;
GRANT UPDATE (offline_sales_valid_from, offline_sales_valid_until)
    ON mobile_devices TO itembaz_runtime;
