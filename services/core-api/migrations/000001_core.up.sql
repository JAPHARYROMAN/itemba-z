CREATE SCHEMA IF NOT EXISTS itembaz;
SET search_path TO itembaz, public;

CREATE TABLE tenants (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE legal_companies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    base_currency char(3) NOT NULL DEFAULT 'TZS',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

CREATE TABLE branches (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id)
);

CREATE TABLE warehouses (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, company_id, branch_id, id),
    FOREIGN KEY (tenant_id, company_id, branch_id) REFERENCES branches(tenant_id, company_id, id)
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    email text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, email)
);

CREATE TABLE roles (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name)
);

CREATE TABLE permissions (
    code text PRIMARY KEY,
    description text NOT NULL
);

CREATE TABLE role_permissions (
    tenant_id uuid NOT NULL,
    role_id uuid NOT NULL,
    permission_code text NOT NULL REFERENCES permissions(code),
    PRIMARY KEY (tenant_id, role_id, permission_code),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id)
);

CREATE TABLE user_role_scopes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id, role_id, company_id, branch_id, warehouse_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id) REFERENCES warehouses(tenant_id, company_id, branch_id, id)
);

INSERT INTO permissions (code, description) VALUES
    ('sales.read', 'View sales in an assigned organizational scope'),
    ('sales.complete', 'Complete sales in an assigned organizational scope'),
    ('sales.reverse', 'Reverse sales in an assigned organizational scope');

CREATE TABLE customer_accounts (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    is_general boolean NOT NULL DEFAULT false,
    credit_enabled boolean NOT NULL DEFAULT false,
    credit_limit_minor bigint NOT NULL DEFAULT 0 CHECK (credit_limit_minor >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    CHECK (NOT is_general OR (NOT credit_enabled AND credit_limit_minor = 0))
);
CREATE UNIQUE INDEX one_general_customer_per_company
    ON customer_accounts (tenant_id, company_id) WHERE is_general;

CREATE TABLE products (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    sku text NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    currency char(3) NOT NULL,
    list_price_minor bigint NOT NULL CHECK (list_price_minor > 0),
    standard_cost_minor bigint NOT NULL CHECK (standard_cost_minor >= 0),
    tax_code text NOT NULL,
    revenue_account_id text NOT NULL,
    cogs_account_id text NOT NULL,
    inventory_account_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, company_id, id),
    UNIQUE (tenant_id, company_id, sku),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id)
);

CREATE TABLE tax_rules (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    code text NOT NULL,
    basis_points integer NOT NULL CHECK (basis_points BETWEEN 0 AND 10000),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, company_id, code, effective_from),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE TABLE fiscal_periods (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    is_open boolean NOT NULL DEFAULT false,
    UNIQUE (tenant_id, company_id, starts_at),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    CHECK (ends_at > starts_at)
);

CREATE TABLE sales_posting_config (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    receivable_account_id text NOT NULL,
    tax_payable_account_id text NOT NULL,
    cash_accounts jsonb NOT NULL CHECK (jsonb_typeof(cash_accounts) = 'object'),
    PRIMARY KEY (tenant_id, company_id),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id)
);

CREATE TABLE sales (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    record_type text NOT NULL CHECK (record_type IN ('SALE', 'REVERSAL')),
    sale_kind text NOT NULL CHECK (sale_kind IN ('CASH', 'CREDIT')),
    status text NOT NULL CHECK (status IN ('POSTED', 'REVERSED')),
    customer_id uuid NOT NULL,
    currency char(3) NOT NULL,
    subtotal_minor bigint NOT NULL CHECK (subtotal_minor >= 0),
    tax_minor bigint NOT NULL CHECK (tax_minor >= 0),
    total_minor bigint NOT NULL CHECK (total_minor = subtotal_minor + tax_minor),
    cogs_minor bigint NOT NULL CHECK (cogs_minor >= 0),
    payment_method text,
    reversal_of uuid,
    reversal_reason text,
    created_by uuid NOT NULL,
	correlation_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    reversed_at timestamptz,
    UNIQUE (tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id) REFERENCES warehouses(tenant_id, company_id, branch_id, id),
    FOREIGN KEY (tenant_id, company_id, customer_id) REFERENCES customer_accounts(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (reversal_of) REFERENCES sales(id),
    CHECK ((record_type = 'SALE' AND reversal_of IS NULL) OR (record_type = 'REVERSAL' AND reversal_of IS NOT NULL AND length(btrim(reversal_reason)) > 0)),
    CHECK ((sale_kind = 'CASH' AND payment_method IS NOT NULL) OR (sale_kind = 'CREDIT' AND payment_method IS NULL))
);
CREATE UNIQUE INDEX one_reversal_per_sale ON sales (tenant_id, company_id, reversal_of) WHERE reversal_of IS NOT NULL;

CREATE TABLE sale_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    sale_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity bigint NOT NULL CHECK (quantity > 0),
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor > 0),
    subtotal_minor bigint NOT NULL CHECK (subtotal_minor >= 0),
    tax_minor bigint NOT NULL CHECK (tax_minor >= 0),
    total_minor bigint NOT NULL CHECK (total_minor = subtotal_minor + tax_minor),
    unit_cost_minor bigint NOT NULL CHECK (unit_cost_minor >= 0),
    cogs_minor bigint NOT NULL CHECK (cogs_minor >= 0),
    FOREIGN KEY (tenant_id, company_id, sale_id) REFERENCES sales(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, company_id, product_id) REFERENCES products(tenant_id, company_id, id)
);

CREATE TABLE inventory_stock_ledger (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    product_id uuid NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    quantity bigint NOT NULL CHECK (quantity <> 0),
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id) REFERENCES warehouses(tenant_id, company_id, branch_id, id),
    FOREIGN KEY (tenant_id, company_id, product_id) REFERENCES products(tenant_id, company_id, id)
);
CREATE INDEX stock_balance_lookup ON inventory_stock_ledger (tenant_id, company_id, branch_id, warehouse_id, product_id);
CREATE INDEX stock_source_lookup ON inventory_stock_ledger (tenant_id, company_id, source_type, source_id);

CREATE TABLE customer_ledger (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor <> 0),
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id, customer_id) REFERENCES customer_accounts(tenant_id, company_id, id)
);
CREATE INDEX customer_exposure_lookup ON customer_ledger (tenant_id, company_id, customer_id, occurred_at);

CREATE TABLE payments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    sale_id uuid NOT NULL,
    account_id text NOT NULL,
    method text NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor <> 0),
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id, sale_id) REFERENCES sales(tenant_id, company_id, id) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE journals (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    currency char(3) NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE (tenant_id, company_id, source_type, source_id)
);

CREATE TABLE journal_lines (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    journal_id uuid NOT NULL REFERENCES journals(id),
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    account_id text NOT NULL,
    debit_minor bigint NOT NULL DEFAULT 0 CHECK (debit_minor >= 0),
    credit_minor bigint NOT NULL DEFAULT 0 CHECK (credit_minor >= 0),
    memo text NOT NULL DEFAULT '',
    CHECK ((debit_minor > 0 AND credit_minor = 0) OR (credit_minor > 0 AND debit_minor = 0))
);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
	correlation_id uuid NOT NULL,
	causation_id text NOT NULL,
    data jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id)
);
CREATE INDEX audit_timeline_lookup ON audit_events (tenant_id, company_id, entity_type, entity_id, occurred_at);
CREATE INDEX audit_correlation_lookup ON audit_events (tenant_id, correlation_id, occurred_at);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
	correlation_id uuid NOT NULL,
	causation_id text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    available_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text,
    locked_by text,
    locked_until timestamptz
);
CREATE INDEX pending_outbox_events ON outbox_events (available_at, locked_until, occurred_at) WHERE processed_at IS NULL;
CREATE INDEX outbox_correlation_lookup ON outbox_events (tenant_id, correlation_id, occurred_at);

CREATE TABLE idempotency_keys (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    operation text NOT NULL,
    idempotency_key text NOT NULL,
    request_hash char(64) NOT NULL,
    result_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    PRIMARY KEY (tenant_id, company_id, operation, idempotency_key),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    CHECK ((result_id IS NULL AND completed_at IS NULL) OR (result_id IS NOT NULL AND completed_at IS NOT NULL))
);

CREATE FUNCTION reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER stock_ledger_append_only BEFORE UPDATE OR DELETE ON inventory_stock_ledger FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER customer_ledger_append_only BEFORE UPDATE OR DELETE ON customer_ledger FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER journal_append_only BEFORE UPDATE OR DELETE ON journals FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER journal_lines_append_only BEFORE UPDATE OR DELETE ON journal_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER audit_append_only BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER sale_lines_append_only BEFORE UPDATE OR DELETE ON sale_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER payments_append_only BEFORE UPDATE OR DELETE ON payments FOR EACH ROW EXECUTE FUNCTION reject_mutation();

CREATE FUNCTION protect_posted_sale() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER sales_immutable BEFORE UPDATE OR DELETE ON sales FOR EACH ROW EXECUTE FUNCTION protect_posted_sale();

CREATE FUNCTION enforce_balanced_journal(target_journal uuid) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    debits bigint;
    credits bigint;
BEGIN
    SELECT COALESCE(sum(debit_minor), 0), COALESCE(sum(credit_minor), 0)
      INTO debits, credits FROM journal_lines WHERE journal_id = target_journal;
    IF debits = 0 OR debits <> credits THEN
        RAISE EXCEPTION 'journal % is unbalanced: debits %, credits %', target_journal, debits, credits USING ERRCODE = '23514';
    END IF;
END;
$$;

-- Every tenant-owned table is protected even if an application query omits its
-- tenant predicate. The API must SET LOCAL app.tenant_id inside each transaction.
ALTER TABLE legal_companies ENABLE ROW LEVEL SECURITY;
ALTER TABLE branches ENABLE ROW LEVEL SECURITY;
ALTER TABLE warehouses ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_role_scopes ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE tax_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_posting_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales ENABLE ROW LEVEL SECURITY;
ALTER TABLE sale_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_stock_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE journals ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;

DO $$
DECLARE table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['legal_companies','branches','warehouses','users','roles','role_permissions','user_role_scopes','customer_accounts','products','tax_rules','fiscal_periods','sales_posting_config','sales','sale_lines','inventory_stock_ledger','customer_ledger','payments','journals','journal_lines','audit_events','outbox_events','idempotency_keys']
    LOOP
        EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)', table_name);
    END LOOP;
END;
$$;
