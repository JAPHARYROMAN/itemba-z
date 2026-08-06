SET search_path TO itembaz, public;

INSERT INTO permissions(code, description) VALUES
    ('integrations.read', 'View external-integration delivery and reconciliation evidence'),
    ('integrations.manage', 'Create and govern external-integration routes'),
    ('integrations.replay', 'Replay an independently reviewed dead-letter delivery')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE integration_routes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    capability text NOT NULL CHECK (capability IN (
        'TRA_FISCALIZATION', 'PAYMENT_CALLBACK', 'BANK_STATEMENT_IMPORT',
        'PAYROLL_EXPORT', 'EMAIL', 'WHATSAPP', 'RECEIPT_PRINT'
    )),
    provider_code text NOT NULL CHECK (provider_code ~ '^[A-Z0-9][A-Z0-9_-]{1,63}$'),
    contract_version text NOT NULL CHECK (length(btrim(contract_version)) BETWEEN 1 AND 100),
    endpoint_url text NOT NULL CHECK (endpoint_url ~ '^https://'),
    secret_reference text NOT NULL CHECK (
        secret_reference ~ '^[a-z][a-z0-9+.-]*://'
        AND secret_reference !~ '[[:space:]]'
    ),
    timeout_milliseconds integer NOT NULL CHECK (timeout_milliseconds BETWEEN 1000 AND 120000),
    max_attempts integer NOT NULL CHECK (max_attempts BETWEEN 1 AND 20),
    base_backoff_seconds integer NOT NULL CHECK (base_backoff_seconds BETWEEN 1 AND 3600),
    max_backoff_seconds integer NOT NULL CHECK (
        max_backoff_seconds BETWEEN base_backoff_seconds AND 86400
    ),
    circuit_failure_threshold integer NOT NULL CHECK (circuit_failure_threshold BETWEEN 1 AND 100),
    circuit_open_seconds integer NOT NULL CHECK (circuit_open_seconds BETWEEN 1 AND 86400),
    status text NOT NULL CHECK (status IN ('DRAFT', 'ACTIVE', 'SUSPENDED', 'RETIRED')),
    valid_from timestamptz NOT NULL,
    valid_until timestamptz,
    created_by uuid NOT NULL,
    approved_by uuid,
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    created_at timestamptz NOT NULL,
    approved_at timestamptz,
    UNIQUE (tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    CHECK (valid_until IS NULL OR valid_until > valid_from),
    CHECK (
        (status = 'DRAFT' AND approved_by IS NULL AND approved_at IS NULL)
        OR
        (status <> 'DRAFT' AND approved_by IS NOT NULL AND approved_at IS NOT NULL
         AND approved_by <> created_by)
    )
);
CREATE UNIQUE INDEX one_active_integration_route
    ON integration_routes(tenant_id, company_id, capability)
    WHERE status = 'ACTIVE';

CREATE TABLE integration_route_health (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    route_id uuid NOT NULL,
    consecutive_failures integer NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    opened_until timestamptz,
    last_success_at timestamptz,
    last_failure_at timestamptz,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, route_id),
    FOREIGN KEY (tenant_id, company_id, route_id)
        REFERENCES integration_routes(tenant_id, company_id, id)
);

CREATE TABLE integration_deliveries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    route_id uuid NOT NULL,
    capability text NOT NULL,
    operation text NOT NULL CHECK (length(btrim(operation)) BETWEEN 1 AND 100),
    source_event_id uuid NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    correlation_id uuid NOT NULL,
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 200),
    request_hash char(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    request_payload jsonb NOT NULL CHECK (jsonb_typeof(request_payload) = 'object'),
    status text NOT NULL CHECK (status IN (
        'PENDING', 'IN_FLIGHT', 'RETRY_SCHEDULED', 'SUCCEEDED', 'DEAD_LETTER', 'CANCELLED'
    )),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts integer NOT NULL CHECK (max_attempts BETWEEN 1 AND 20),
    available_at timestamptz NOT NULL,
    locked_by text,
    locked_until timestamptz,
    provider_reference text,
    response_payload jsonb,
    last_error_code text,
    last_error_message text,
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    UNIQUE (tenant_id, source_event_id, capability),
    UNIQUE (tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, company_id, route_id)
        REFERENCES integration_routes(tenant_id, company_id, id),
    CHECK ((locked_by IS NULL) = (locked_until IS NULL)),
    CHECK ((status = 'SUCCEEDED') = (completed_at IS NOT NULL)),
    CHECK (response_payload IS NULL OR jsonb_typeof(response_payload) = 'object')
);
CREATE INDEX pending_integration_deliveries
    ON integration_deliveries(available_at, locked_until, created_at)
    WHERE status IN ('PENDING', 'RETRY_SCHEDULED');
CREATE UNIQUE INDEX unique_provider_delivery_reference
    ON integration_deliveries(tenant_id, route_id, provider_reference)
    WHERE provider_reference IS NOT NULL;

CREATE TABLE integration_delivery_attempts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    attempt_number integer NOT NULL CHECK (attempt_number > 0),
    worker_id text NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('SUCCEEDED', 'RETRY_SCHEDULED', 'DEAD_LETTER')),
    error_code text,
    error_message text,
    provider_reference text,
    response_payload jsonb,
    started_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL,
    UNIQUE (tenant_id, delivery_id, attempt_number),
    FOREIGN KEY (tenant_id, company_id, delivery_id)
        REFERENCES integration_deliveries(tenant_id, company_id, id),
    CHECK (completed_at >= started_at),
    CHECK (response_payload IS NULL OR jsonb_typeof(response_payload) = 'object')
);

CREATE FUNCTION protect_integration_route() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'integration routes cannot be deleted' USING ERRCODE = '55000';
    END IF;
    IF to_jsonb(NEW) - ARRAY['status', 'approved_by', 'approved_at']
       <> to_jsonb(OLD) - ARRAY['status', 'approved_by', 'approved_at'] THEN
        RAISE EXCEPTION 'integration route facts are immutable; create a new version' USING ERRCODE = '55000';
    END IF;
    IF NOT (
        (OLD.status = 'DRAFT' AND NEW.status IN ('ACTIVE', 'RETIRED'))
        OR (OLD.status = 'ACTIVE' AND NEW.status IN ('SUSPENDED', 'RETIRED'))
        OR (OLD.status = 'SUSPENDED' AND NEW.status IN ('ACTIVE', 'RETIRED'))
    ) THEN
        RAISE EXCEPTION 'invalid integration route transition' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER integration_route_guard
    BEFORE UPDATE OR DELETE ON integration_routes
    FOR EACH ROW EXECUTE FUNCTION protect_integration_route();

CREATE FUNCTION protect_integration_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER integration_delivery_guard
    BEFORE UPDATE OR DELETE ON integration_deliveries
    FOR EACH ROW EXECUTE FUNCTION protect_integration_delivery();
CREATE TRIGGER integration_attempts_append_only
    BEFORE UPDATE OR DELETE ON integration_delivery_attempts
    FOR EACH ROW EXECUTE FUNCTION reject_mutation();

CREATE OR REPLACE FUNCTION protect_posted_sale() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    business_changed boolean;
    fiscal_changed boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'posted sales cannot be deleted' USING ERRCODE = '55000';
    END IF;
    IF to_jsonb(NEW) - ARRAY['status', 'reversed_at', 'fiscal_status']
       <> to_jsonb(OLD) - ARRAY['status', 'reversed_at', 'fiscal_status'] THEN
        RAISE EXCEPTION 'posted sale facts are immutable' USING ERRCODE = '55000';
    END IF;
    business_changed := NEW.status IS DISTINCT FROM OLD.status
        OR NEW.reversed_at IS DISTINCT FROM OLD.reversed_at;
    fiscal_changed := NEW.fiscal_status IS DISTINCT FROM OLD.fiscal_status;
    IF business_changed AND NOT (
        OLD.status = 'POSTED' AND NEW.status = 'REVERSED' AND NEW.reversed_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'posted sales may only transition from POSTED to REVERSED' USING ERRCODE = '55000';
    END IF;
    IF fiscal_changed AND NOT (
        (OLD.fiscal_status = 'NOT_CONFIGURED' AND NEW.fiscal_status = 'PENDING')
        OR (OLD.fiscal_status = 'PENDING' AND NEW.fiscal_status IN ('FISCALIZED', 'FAILED'))
        OR (OLD.fiscal_status = 'FAILED' AND NEW.fiscal_status = 'PENDING')
    ) THEN
        RAISE EXCEPTION 'invalid fiscalization status transition' USING ERRCODE = '55000';
    END IF;
    IF NOT business_changed AND NOT fiscal_changed THEN
        RAISE EXCEPTION 'posted sale update contains no permitted transition' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'integration_routes', 'integration_route_health',
        'integration_deliveries', 'integration_delivery_attempts'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',
            table_name
        );
    END LOOP;
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
    WHERE delivery.status IN ('PENDING', 'RETRY_SCHEDULED')
      AND delivery.available_at <= clock_timestamp()
      AND (delivery.locked_until IS NULL OR delivery.locked_until < clock_timestamp())
      AND (health.opened_until IS NULL OR health.opened_until <= clock_timestamp())
    ORDER BY delivery.tenant_id
$$;
REVOKE ALL ON FUNCTION pending_integration_tenant_ids() FROM PUBLIC, itembaz_runtime;
GRANT EXECUTE ON FUNCTION pending_integration_tenant_ids() TO itembaz_worker_runtime;

REVOKE ALL ON integration_routes, integration_route_health,
    integration_deliveries, integration_delivery_attempts FROM PUBLIC;
GRANT SELECT ON integration_routes, integration_route_health,
    integration_deliveries, integration_delivery_attempts TO itembaz_runtime;
GRANT INSERT, UPDATE ON integration_routes TO itembaz_runtime;
GRANT SELECT ON integration_routes, integration_route_health,
    integration_deliveries, integration_delivery_attempts TO itembaz_worker_runtime;
GRANT INSERT ON integration_deliveries, integration_delivery_attempts,
    integration_route_health TO itembaz_worker_runtime;
GRANT UPDATE ON integration_deliveries, integration_route_health TO itembaz_worker_runtime;
GRANT SELECT ON sales TO itembaz_worker_runtime;
GRANT UPDATE (fiscal_status) ON sales TO itembaz_worker_runtime;
