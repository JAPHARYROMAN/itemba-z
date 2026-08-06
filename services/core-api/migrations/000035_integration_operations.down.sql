SET search_path TO itembaz, public;

ALTER TABLE integration_deliveries DROP CONSTRAINT integration_deliveries_max_attempts_check;
ALTER TABLE integration_deliveries ADD CONSTRAINT integration_deliveries_max_attempts_check CHECK (max_attempts BETWEEN 1 AND 20);

DROP TABLE IF EXISTS integration_circuit_resets;
DROP TABLE IF EXISTS integration_delivery_replays;
DROP TABLE IF EXISTS integration_route_transitions;

CREATE OR REPLACE FUNCTION protect_integration_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
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
    RETURN NEW;
END;
$$;
