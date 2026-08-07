SET search_path TO itembaz, public;

ALTER TABLE integration_deliveries DROP CONSTRAINT integration_deliveries_max_attempts_check;
ALTER TABLE integration_deliveries ADD CONSTRAINT integration_deliveries_max_attempts_check CHECK (max_attempts BETWEEN 1 AND 1000);

CREATE TABLE integration_route_transitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    route_id uuid NOT NULL,
    from_status text NOT NULL,
    to_status text NOT NULL,
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    actor_id uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id, route_id) REFERENCES integration_routes(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id)
);

CREATE TABLE integration_delivery_replays (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    previous_attempt_count integer NOT NULL CHECK (previous_attempt_count > 0),
    new_attempt_budget integer NOT NULL CHECK (new_attempt_budget > previous_attempt_count),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    actor_id uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id, delivery_id) REFERENCES integration_deliveries(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id)
);

CREATE TABLE integration_circuit_resets (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    route_id uuid NOT NULL,
    previous_failures integer NOT NULL CHECK (previous_failures >= 0),
    previous_opened_until timestamptz,
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    actor_id uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id, route_id) REFERENCES integration_routes(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id)
);

CREATE TRIGGER integration_route_transitions_append_only BEFORE UPDATE OR DELETE ON integration_route_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER integration_delivery_replays_append_only BEFORE UPDATE OR DELETE ON integration_delivery_replays FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER integration_circuit_resets_append_only BEFORE UPDATE OR DELETE ON integration_circuit_resets FOR EACH ROW EXECUTE FUNCTION reject_mutation();

CREATE OR REPLACE FUNCTION protect_integration_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'integration deliveries cannot be deleted' USING ERRCODE = '55000';
    END IF;
    IF to_jsonb(NEW) - ARRAY[
        'status','attempt_count','max_attempts','available_at','locked_by','locked_until',
        'provider_reference','response_payload','last_error_code','last_error_message','completed_at'
    ] <> to_jsonb(OLD) - ARRAY[
        'status','attempt_count','max_attempts','available_at','locked_by','locked_until',
        'provider_reference','response_payload','last_error_code','last_error_message','completed_at'
    ] THEN
        RAISE EXCEPTION 'integration delivery request facts are immutable' USING ERRCODE = '55000';
    END IF;
    IF NEW.max_attempts IS DISTINCT FROM OLD.max_attempts AND NOT (
        OLD.status = 'DEAD_LETTER' AND NEW.status = 'PENDING'
        AND NEW.max_attempts > OLD.attempt_count
    ) THEN
        RAISE EXCEPTION 'attempt budget may only expand during governed dead-letter replay' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

DO $$
DECLARE table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['integration_route_transitions','integration_delivery_replays','integration_circuit_resets'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)', table_name);
    END LOOP;
END;
$$;

REVOKE ALL ON integration_route_transitions, integration_delivery_replays, integration_circuit_resets FROM PUBLIC;
GRANT SELECT, INSERT ON integration_route_transitions, integration_delivery_replays, integration_circuit_resets TO itembaz_runtime;
GRANT INSERT, UPDATE ON integration_route_health TO itembaz_runtime;
GRANT SELECT ON integration_delivery_replays TO itembaz_worker_runtime;
