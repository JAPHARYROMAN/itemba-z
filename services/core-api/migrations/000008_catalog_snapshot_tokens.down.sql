SET search_path TO itembaz, public;

REVOKE UPDATE (catalog_snapshot_token) ON mobile_devices FROM itembaz_runtime;

DROP TRIGGER customers_publish_catalog_change ON customer_accounts;
DROP FUNCTION publish_customer_catalog_change();
DROP TRIGGER products_publish_catalog_change ON products;
DROP FUNCTION publish_product_catalog_change();

-- Restore the serialized tax-version publisher from migration 000006.
CREATE OR REPLACE FUNCTION bump_master_data_version_for_tax_rule()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, itembaz
AS $$
DECLARE
    old_lock_key text;
    new_lock_key text;
BEGIN
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
    IF TG_OP = 'INSERT' THEN
        UPDATE itembaz.legal_companies SET master_data_version = master_data_version + 1
        WHERE tenant_id = NEW.tenant_id AND id = NEW.company_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE itembaz.legal_companies SET master_data_version = master_data_version + 1
        WHERE tenant_id = OLD.tenant_id AND id = OLD.company_id;
        RETURN OLD;
    ELSIF ROW(OLD.tenant_id, OLD.company_id, OLD.code, OLD.basis_points, OLD.effective_from, OLD.effective_to)
          IS DISTINCT FROM
          ROW(NEW.tenant_id, NEW.company_id, NEW.code, NEW.basis_points, NEW.effective_from, NEW.effective_to) THEN
        UPDATE itembaz.legal_companies SET master_data_version = master_data_version + 1
        WHERE tenant_id = OLD.tenant_id AND id = OLD.company_id;
        IF ROW(OLD.tenant_id, OLD.company_id) IS DISTINCT FROM ROW(NEW.tenant_id, NEW.company_id) THEN
            UPDATE itembaz.legal_companies SET master_data_version = master_data_version + 1
            WHERE tenant_id = NEW.tenant_id AND id = NEW.company_id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE sales
    DROP CONSTRAINT sales_mobile_snapshot_required,
    DROP COLUMN client_catalog_snapshot_token;

ALTER TABLE mobile_device_offline_leases
    DROP CONSTRAINT mobile_offline_lease_snapshot_required,
    DROP COLUMN catalog_snapshot_token;

ALTER TABLE mobile_devices DROP COLUMN catalog_snapshot_token;
ALTER TABLE legal_companies DROP COLUMN catalog_snapshot_token;
