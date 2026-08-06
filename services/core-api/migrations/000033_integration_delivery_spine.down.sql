SET search_path TO itembaz, public;

REVOKE UPDATE (fiscal_status) ON sales FROM itembaz_worker_runtime;
DROP FUNCTION IF EXISTS pending_integration_tenant_ids();
DROP TRIGGER integration_attempts_append_only ON integration_delivery_attempts;
DROP TRIGGER integration_delivery_guard ON integration_deliveries;
DROP TRIGGER integration_route_guard ON integration_routes;
DROP FUNCTION protect_integration_delivery();
DROP FUNCTION protect_integration_route();
DROP TABLE integration_delivery_attempts;
DROP TABLE integration_deliveries;
DROP TABLE integration_route_health;
DROP TABLE integration_routes;

CREATE OR REPLACE FUNCTION protect_posted_sale() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'posted sales cannot be deleted' USING ERRCODE = '55000';
    END IF;
    IF to_jsonb(NEW) - ARRAY['status', 'reversed_at'] <> to_jsonb(OLD) - ARRAY['status', 'reversed_at']
       OR OLD.status <> 'POSTED' OR NEW.status <> 'REVERSED' OR NEW.reversed_at IS NULL THEN
        RAISE EXCEPTION 'posted sales may only transition from POSTED to REVERSED' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

DELETE FROM role_permissions WHERE permission_code IN (
    'integrations.read', 'integrations.manage', 'integrations.replay'
);
DELETE FROM permissions WHERE code IN (
    'integrations.read', 'integrations.manage', 'integrations.replay'
);
