SET search_path TO itembaz, public;

-- A device has not installed any governed cache until it explicitly
-- acknowledges both versions. Zero is the durable unacknowledged state.
ALTER TABLE mobile_devices
    DROP CONSTRAINT mobile_devices_master_data_version_check,
    DROP CONSTRAINT mobile_devices_price_version_check,
    ADD CONSTRAINT mobile_devices_master_data_version_check CHECK (master_data_version >= 0),
    ADD CONSTRAINT mobile_devices_price_version_check CHECK (price_version >= 0);

-- The current milestone permits controlled offline sales only for physical
-- cash and zero-rated lines. The application enforces the effective rule per
-- line; this constraint is a final persistence guard on the posted totals.
ALTER TABLE sales
    DROP CONSTRAINT sales_offline_requires_mobile_cash,
    ADD CONSTRAINT sales_offline_requires_mobile_cash CHECK (
        NOT offline OR (
            record_type = 'SALE' AND
            sale_kind = 'CASH' AND
            payment_method = 'CASH' AND
            device_id IS NOT NULL AND
            tax_minor = 0
        )
    );

CREATE OR REPLACE FUNCTION bump_master_data_version_for_tax_rule()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, itembaz
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE itembaz.legal_companies
        SET master_data_version = master_data_version + 1
        WHERE tenant_id = NEW.tenant_id AND id = NEW.company_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE itembaz.legal_companies
        SET master_data_version = master_data_version + 1
        WHERE tenant_id = OLD.tenant_id AND id = OLD.company_id;
        RETURN OLD;
    ELSIF ROW(OLD.tenant_id, OLD.company_id, OLD.code, OLD.basis_points, OLD.effective_from, OLD.effective_to)
          IS DISTINCT FROM
          ROW(NEW.tenant_id, NEW.company_id, NEW.code, NEW.basis_points, NEW.effective_from, NEW.effective_to) THEN
        UPDATE itembaz.legal_companies
        SET master_data_version = master_data_version + 1
        WHERE tenant_id = OLD.tenant_id AND id = OLD.company_id;
        IF ROW(OLD.tenant_id, OLD.company_id) IS DISTINCT FROM ROW(NEW.tenant_id, NEW.company_id) THEN
            UPDATE itembaz.legal_companies
            SET master_data_version = master_data_version + 1
            WHERE tenant_id = NEW.tenant_id AND id = NEW.company_id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

REVOKE ALL ON FUNCTION bump_master_data_version_for_tax_rule() FROM PUBLIC;
REVOKE ALL ON FUNCTION bump_master_data_version_for_tax_rule() FROM itembaz_runtime;
REVOKE ALL ON FUNCTION bump_master_data_version_for_tax_rule() FROM itembaz_worker_runtime;

CREATE TRIGGER tax_rule_bumps_master_data_version
AFTER INSERT OR UPDATE OR DELETE ON tax_rules
FOR EACH ROW EXECUTE FUNCTION bump_master_data_version_for_tax_rule();
