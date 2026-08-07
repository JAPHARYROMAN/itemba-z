SET search_path TO itembaz, public;

-- Enrollment acknowledgements and tax-rule version bumps share this advisory
-- lock namespace. This makes "acknowledged current version" a commit-time
-- invariant instead of a read-then-write race.
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
