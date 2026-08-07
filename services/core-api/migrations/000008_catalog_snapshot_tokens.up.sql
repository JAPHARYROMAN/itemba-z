SET search_path TO itembaz, public;

-- A catalog snapshot token identifies one published combination of governed
-- customer, product, price, posting, and tax facts. Dynamic stock balances and
-- customer exposure are intentionally outside this token: offline stock is
-- bounded by device allocations and mobile credit remains disabled.
ALTER TABLE legal_companies
    ADD COLUMN catalog_snapshot_token uuid NOT NULL DEFAULT gen_random_uuid();

-- The all-zero token is the durable "not acknowledged" marker for devices and
-- pre-migration evidence. New leases and mobile sales reject that marker.
ALTER TABLE mobile_devices
    ADD COLUMN catalog_snapshot_token uuid NOT NULL
        DEFAULT '00000000-0000-0000-0000-000000000000';

ALTER TABLE mobile_device_offline_leases
    ADD COLUMN catalog_snapshot_token uuid NOT NULL
        DEFAULT '00000000-0000-0000-0000-000000000000',
    ADD CONSTRAINT mobile_offline_lease_snapshot_required CHECK (
        catalog_snapshot_token <> '00000000-0000-0000-0000-000000000000'
    ) NOT VALID;

ALTER TABLE sales
    ADD COLUMN client_catalog_snapshot_token uuid,
    ADD CONSTRAINT sales_mobile_snapshot_required CHECK (
        device_id IS NULL OR (
            client_catalog_snapshot_token IS NOT NULL AND
            client_catalog_snapshot_token <> '00000000-0000-0000-0000-000000000000'
        )
    ) NOT VALID;

-- Tax publication already serializes with device acknowledgements. Rotate the
-- snapshot token in the same transaction as the master-data version bump.
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
        UPDATE itembaz.legal_companies
        SET master_data_version = master_data_version + 1,
            catalog_snapshot_token = gen_random_uuid()
        WHERE tenant_id = NEW.tenant_id AND id = NEW.company_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE itembaz.legal_companies
        SET master_data_version = master_data_version + 1,
            catalog_snapshot_token = gen_random_uuid()
        WHERE tenant_id = OLD.tenant_id AND id = OLD.company_id;
        RETURN OLD;
    ELSIF ROW(OLD.tenant_id, OLD.company_id, OLD.code, OLD.basis_points, OLD.effective_from, OLD.effective_to)
          IS DISTINCT FROM
          ROW(NEW.tenant_id, NEW.company_id, NEW.code, NEW.basis_points, NEW.effective_from, NEW.effective_to) THEN
        UPDATE itembaz.legal_companies
        SET master_data_version = master_data_version + 1,
            catalog_snapshot_token = gen_random_uuid()
        WHERE tenant_id = OLD.tenant_id AND id = OLD.company_id;
        IF ROW(OLD.tenant_id, OLD.company_id) IS DISTINCT FROM ROW(NEW.tenant_id, NEW.company_id) THEN
            UPDATE itembaz.legal_companies
            SET master_data_version = master_data_version + 1,
                catalog_snapshot_token = gen_random_uuid()
            WHERE tenant_id = NEW.tenant_id AND id = NEW.company_id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

-- Product facts are published as one company version. Direct version writes
-- are ignored; governed field changes own the version and token transition.
CREATE FUNCTION publish_product_catalog_change()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, itembaz
AS $$
DECLARE
    master_changed boolean := false;
    price_changed boolean := false;
    next_master bigint;
    next_price bigint;
    lock_key text;
BEGIN
    IF TG_OP = 'UPDATE' AND ROW(OLD.tenant_id, OLD.company_id) IS DISTINCT FROM ROW(NEW.tenant_id, NEW.company_id) THEN
        RAISE EXCEPTION 'products cannot move between legal companies' USING ERRCODE = '23514';
    END IF;

    IF TG_OP = 'INSERT' OR TG_OP = 'DELETE' THEN
        master_changed := true;
        price_changed := true;
    ELSE
        master_changed := ROW(OLD.sku, OLD.name, OLD.base_unit_code, OLD.active,
            OLD.standard_cost_minor, OLD.tax_code, OLD.revenue_account_id,
            OLD.cogs_account_id, OLD.inventory_account_id)
            IS DISTINCT FROM
            ROW(NEW.sku, NEW.name, NEW.base_unit_code, NEW.active,
            NEW.standard_cost_minor, NEW.tax_code, NEW.revenue_account_id,
            NEW.cogs_account_id, NEW.inventory_account_id);
        price_changed := ROW(OLD.currency, OLD.list_price_minor)
            IS DISTINCT FROM ROW(NEW.currency, NEW.list_price_minor);
        IF NOT master_changed AND NOT price_changed THEN
            NEW.master_data_version := OLD.master_data_version;
            NEW.price_version := OLD.price_version;
            RETURN NEW;
        END IF;
    END IF;

    lock_key := 'master-data-version:' || COALESCE(NEW.tenant_id, OLD.tenant_id)::text || ':' || COALESCE(NEW.company_id, OLD.company_id)::text;
    PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(lock_key, 0));
    UPDATE itembaz.legal_companies
    SET master_data_version = master_data_version + CASE WHEN master_changed THEN 1 ELSE 0 END,
        price_version = price_version + CASE WHEN price_changed THEN 1 ELSE 0 END,
        catalog_snapshot_token = gen_random_uuid()
    WHERE tenant_id = COALESCE(NEW.tenant_id, OLD.tenant_id)
      AND id = COALESCE(NEW.company_id, OLD.company_id)
    RETURNING master_data_version, price_version INTO next_master, next_price;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    IF master_changed THEN NEW.master_data_version := next_master; END IF;
    IF price_changed THEN NEW.price_version := next_price; END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER products_publish_catalog_change
BEFORE INSERT OR UPDATE OR DELETE ON products
FOR EACH ROW EXECUTE FUNCTION publish_product_catalog_change();

CREATE FUNCTION publish_customer_catalog_change()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, itembaz
AS $$
DECLARE
    lock_key text;
BEGIN
    IF TG_OP = 'UPDATE' AND ROW(OLD.tenant_id, OLD.company_id) IS DISTINCT FROM ROW(NEW.tenant_id, NEW.company_id) THEN
        RAISE EXCEPTION 'customers cannot move between legal companies' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND ROW(OLD.code, OLD.name, OLD.active, OLD.is_general,
        OLD.credit_enabled, OLD.credit_limit_minor)
        IS NOT DISTINCT FROM ROW(NEW.code, NEW.name, NEW.active, NEW.is_general,
        NEW.credit_enabled, NEW.credit_limit_minor) THEN
        RETURN NEW;
    END IF;

    lock_key := 'master-data-version:' || COALESCE(NEW.tenant_id, OLD.tenant_id)::text || ':' || COALESCE(NEW.company_id, OLD.company_id)::text;
    PERFORM pg_catalog.pg_advisory_xact_lock(pg_catalog.hashtextextended(lock_key, 0));
    UPDATE itembaz.legal_companies
    SET master_data_version = master_data_version + 1,
        catalog_snapshot_token = gen_random_uuid()
    WHERE tenant_id = COALESCE(NEW.tenant_id, OLD.tenant_id)
      AND id = COALESCE(NEW.company_id, OLD.company_id);
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER customers_publish_catalog_change
BEFORE INSERT OR UPDATE OR DELETE ON customer_accounts
FOR EACH ROW EXECUTE FUNCTION publish_customer_catalog_change();

REVOKE ALL ON FUNCTION publish_product_catalog_change() FROM PUBLIC;
REVOKE ALL ON FUNCTION publish_product_catalog_change() FROM itembaz_runtime;
REVOKE ALL ON FUNCTION publish_product_catalog_change() FROM itembaz_worker_runtime;
REVOKE ALL ON FUNCTION publish_customer_catalog_change() FROM PUBLIC;
REVOKE ALL ON FUNCTION publish_customer_catalog_change() FROM itembaz_runtime;
REVOKE ALL ON FUNCTION publish_customer_catalog_change() FROM itembaz_worker_runtime;

GRANT UPDATE (catalog_snapshot_token) ON mobile_devices TO itembaz_runtime;
