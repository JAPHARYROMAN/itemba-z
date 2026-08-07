SET search_path TO itembaz, public;

DROP INDEX pending_integration_deliveries;
CREATE INDEX pending_integration_deliveries
    ON integration_deliveries(available_at, locked_until, created_at)
    WHERE status IN ('PENDING', 'RETRY_SCHEDULED', 'IN_FLIGHT');

CREATE OR REPLACE FUNCTION protect_integration_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    transition_allowed boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'integration deliveries cannot be deleted' USING ERRCODE = '55000';
    END IF;
    IF to_jsonb(NEW) - ARRAY[
        'status','attempt_count','available_at','locked_by','locked_until',
        'provider_reference','response_payload','last_error_code','last_error_message','completed_at'
    ] <> to_jsonb(OLD) - ARRAY[
        'status','attempt_count','available_at','locked_by','locked_until',
        'provider_reference','response_payload','last_error_code','last_error_message','completed_at'
    ] THEN
        RAISE EXCEPTION 'integration delivery request facts are immutable' USING ERRCODE = '55000';
    END IF;

    transition_allowed :=
        (OLD.status IN ('PENDING','RETRY_SCHEDULED')
         AND NEW.status = 'IN_FLIGHT'
         AND NEW.attempt_count = OLD.attempt_count + 1
         AND NEW.locked_by IS NOT NULL AND NEW.locked_until IS NOT NULL)
        OR
        (OLD.status = 'IN_FLIGHT' AND OLD.locked_until < clock_timestamp()
         AND NEW.status = 'IN_FLIGHT'
         AND NEW.attempt_count = OLD.attempt_count + 1
         AND NEW.locked_by IS NOT NULL AND NEW.locked_until IS NOT NULL)
        OR
        (OLD.status = 'IN_FLIGHT'
         AND NEW.status IN ('SUCCEEDED','RETRY_SCHEDULED','DEAD_LETTER')
         AND NEW.attempt_count = OLD.attempt_count
         AND NEW.locked_by IS NULL AND NEW.locked_until IS NULL)
        OR
        (OLD.status = 'DEAD_LETTER' AND NEW.status = 'PENDING'
         AND NEW.attempt_count = OLD.attempt_count
         AND NEW.locked_by IS NULL AND NEW.locked_until IS NULL);

    IF NOT transition_allowed THEN
        RAISE EXCEPTION 'invalid integration delivery transition' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION pending_integration_tenant_ids()
RETURNS TABLE (tenant_id uuid)
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog, itembaz
AS $$
    SELECT DISTINCT delivery.tenant_id
    FROM itembaz.integration_deliveries AS delivery
    JOIN itembaz.integration_routes AS route
      ON route.tenant_id = delivery.tenant_id AND route.id = delivery.route_id
    LEFT JOIN itembaz.integration_route_health AS health
      ON health.tenant_id = route.tenant_id AND health.route_id = route.id
    WHERE delivery.status IN ('PENDING', 'RETRY_SCHEDULED', 'IN_FLIGHT')
      AND delivery.available_at <= clock_timestamp()
      AND (delivery.locked_until IS NULL OR delivery.locked_until < clock_timestamp())
      AND (health.opened_until IS NULL OR health.opened_until <= clock_timestamp())
    ORDER BY delivery.tenant_id
$$;
REVOKE ALL ON FUNCTION pending_integration_tenant_ids() FROM PUBLIC, itembaz_runtime;
GRANT EXECUTE ON FUNCTION pending_integration_tenant_ids() TO itembaz_worker_runtime;
