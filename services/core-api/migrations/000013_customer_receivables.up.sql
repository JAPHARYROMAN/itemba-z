SET search_path TO itembaz, public;

INSERT INTO permissions (code, description) VALUES
    ('customers.accounts.read', 'View customer receivable ageing and effective credit policy in an assigned scope'),
    ('customers.credit.manage', 'Schedule reasoned effective-dated customer credit policy in an assigned scope');

CREATE TABLE customer_credit_policies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    credit_enabled boolean NOT NULL,
    credit_limit_minor bigint NOT NULL CHECK (credit_limit_minor >= 0),
    payment_terms_days integer NOT NULL CHECK (payment_terms_days BETWEEN 0 AND 365),
    max_overdue_days integer NOT NULL CHECK (max_overdue_days BETWEEN 0 AND 3650),
    risk_status text NOT NULL CHECK (risk_status IN ('STANDARD','WATCH','HOLD')),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    effective_from timestamptz NOT NULL,
    approved_by uuid,
    created_at timestamptz NOT NULL,
    correlation_id uuid NOT NULL,
    idempotency_key text,
    request_hash char(64),
    UNIQUE (tenant_id, company_id, customer_id, effective_from),
    UNIQUE (tenant_id, company_id, idempotency_key),
    FOREIGN KEY (tenant_id, company_id, customer_id) REFERENCES customer_accounts(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    CHECK (credit_enabled OR credit_limit_minor = 0),
    CHECK ((idempotency_key IS NULL AND request_hash IS NULL) OR
           (length(idempotency_key) BETWEEN 16 AND 128 AND request_hash ~ '^[0-9a-f]{64}$'))
);
CREATE INDEX customer_credit_policy_effective_lookup
    ON customer_credit_policies (tenant_id, company_id, customer_id, effective_from DESC);

CREATE TABLE customer_receivable_items (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('INVOICE','CREDIT_NOTE','RECEIPT')),
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency char(3) NOT NULL,
    document_at timestamptz NOT NULL,
    due_at timestamptz,
    occurred_at timestamptz NOT NULL,
    UNIQUE (tenant_id, company_id, customer_id, id),
    UNIQUE (tenant_id, company_id, source_type, source_id),
    FOREIGN KEY (tenant_id, company_id, customer_id) REFERENCES customer_accounts(tenant_id, company_id, id),
    CHECK ((kind='INVOICE' AND due_at IS NOT NULL AND due_at >= document_at) OR
           (kind<>'INVOICE' AND due_at IS NULL))
);
CREATE INDEX customer_receivable_item_ageing_lookup
    ON customer_receivable_items (tenant_id, company_id, customer_id, due_at, id);

CREATE TABLE customer_receivable_allocations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    debit_item_id uuid NOT NULL,
    credit_item_id uuid NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    occurred_at timestamptz NOT NULL,
    UNIQUE (tenant_id, company_id, debit_item_id, credit_item_id),
    FOREIGN KEY (tenant_id, company_id, customer_id, debit_item_id)
        REFERENCES customer_receivable_items(tenant_id, company_id, customer_id, id),
    FOREIGN KEY (tenant_id, company_id, customer_id, credit_item_id)
        REFERENCES customer_receivable_items(tenant_id, company_id, customer_id, id),
    CHECK (debit_item_id <> credit_item_id)
);

CREATE FUNCTION validate_customer_credit_policy() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, itembaz AS $$
DECLARE general_customer boolean;
BEGIN
    SELECT is_general INTO general_customer FROM customer_accounts
    WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND id=NEW.customer_id;
    IF general_customer AND (NEW.credit_enabled OR NEW.credit_limit_minor <> 0) THEN
        RAISE EXCEPTION 'General Customer cannot receive a credit policy' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER customer_credit_policy_guard
BEFORE INSERT ON customer_credit_policies
FOR EACH ROW EXECUTE FUNCTION validate_customer_credit_policy();

CREATE FUNCTION default_customer_credit_policy() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, itembaz AS $$
BEGIN
    INSERT INTO customer_credit_policies (
        id,tenant_id,company_id,customer_id,credit_enabled,credit_limit_minor,
        payment_terms_days,max_overdue_days,risk_status,reason,effective_from,
        approved_by,created_at,correlation_id
    ) VALUES (
        gen_random_uuid(),NEW.tenant_id,NEW.company_id,NEW.id,NEW.credit_enabled,NEW.credit_limit_minor,
        30,0,'STANDARD','Initial customer account policy',NEW.created_at,
        NULL,NEW.created_at,gen_random_uuid()
    );
    RETURN NEW;
END $$;
CREATE TRIGGER customer_default_credit_policy
AFTER INSERT ON customer_accounts
FOR EACH ROW EXECUTE FUNCTION default_customer_credit_policy();

CREATE FUNCTION protect_legacy_customer_credit() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (OLD.credit_enabled,OLD.credit_limit_minor) IS DISTINCT FROM
       (NEW.credit_enabled,NEW.credit_limit_minor) THEN
        RAISE EXCEPTION 'customer credit fields are policy-owned and immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER customer_credit_fields_policy_owned
BEFORE UPDATE OF credit_enabled, credit_limit_minor ON customer_accounts
FOR EACH ROW EXECUTE FUNCTION protect_legacy_customer_credit();

CREATE FUNCTION validate_receivable_allocation() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, itembaz AS $$
DECLARE debit_kind text; credit_kind text; debit_amount bigint; credit_amount bigint;
DECLARE debit_allocated bigint; credit_allocated bigint; debit_currency char(3); credit_currency char(3);
BEGIN
    SELECT kind,amount_minor,currency INTO debit_kind,debit_amount,debit_currency
    FROM customer_receivable_items WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id
      AND customer_id=NEW.customer_id AND id=NEW.debit_item_id FOR UPDATE;
    SELECT kind,amount_minor,currency INTO credit_kind,credit_amount,credit_currency
    FROM customer_receivable_items WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id
      AND customer_id=NEW.customer_id AND id=NEW.credit_item_id FOR UPDATE;
    IF debit_kind IS DISTINCT FROM 'INVOICE' OR credit_kind NOT IN ('CREDIT_NOTE','RECEIPT') OR
       debit_currency IS DISTINCT FROM credit_currency THEN
        RAISE EXCEPTION 'invalid receivable allocation relationship' USING ERRCODE='23514';
    END IF;
    SELECT COALESCE(sum(amount_minor),0) INTO debit_allocated FROM customer_receivable_allocations
      WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND debit_item_id=NEW.debit_item_id;
    SELECT COALESCE(sum(amount_minor),0) INTO credit_allocated FROM customer_receivable_allocations
      WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND credit_item_id=NEW.credit_item_id;
    IF debit_allocated+NEW.amount_minor > debit_amount OR credit_allocated+NEW.amount_minor > credit_amount THEN
        RAISE EXCEPTION 'receivable allocation exceeds an item balance' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER customer_receivable_allocation_guard
BEFORE INSERT ON customer_receivable_allocations
FOR EACH ROW EXECUTE FUNCTION validate_receivable_allocation();

INSERT INTO customer_credit_policies (
    id,tenant_id,company_id,customer_id,credit_enabled,credit_limit_minor,
    payment_terms_days,max_overdue_days,risk_status,reason,effective_from,
    approved_by,created_at,correlation_id
)
SELECT gen_random_uuid(),c.tenant_id,c.company_id,c.id,c.credit_enabled,c.credit_limit_minor,
       30,0,'STANDARD','Migrated customer account policy',c.created_at,
       NULL,c.created_at,gen_random_uuid()
FROM customer_accounts c;

INSERT INTO customer_receivable_items (
    id,tenant_id,company_id,customer_id,kind,source_type,source_id,amount_minor,
    currency,document_at,due_at,occurred_at
)
SELECT gen_random_uuid(),s.tenant_id,s.company_id,s.customer_id,'INVOICE','SALE',s.id,s.total_minor,
       s.currency,s.document_at,s.document_at + make_interval(days => COALESCE((
         SELECT p.payment_terms_days FROM customer_credit_policies p
         WHERE p.tenant_id=s.tenant_id AND p.company_id=s.company_id AND p.customer_id=s.customer_id
           AND p.effective_from <= s.accounting_at ORDER BY p.effective_from DESC LIMIT 1
       ),30)),s.accounting_at
FROM sales s WHERE s.record_type='SALE' AND s.sale_kind='CREDIT';

INSERT INTO customer_receivable_items (
    id,tenant_id,company_id,customer_id,kind,source_type,source_id,amount_minor,
    currency,document_at,due_at,occurred_at
)
SELECT gen_random_uuid(),s.tenant_id,s.company_id,s.customer_id,'CREDIT_NOTE','REVERSAL',s.id,s.total_minor,
       s.currency,s.document_at,NULL,s.accounting_at
FROM sales s WHERE s.record_type='REVERSAL' AND s.sale_kind='CREDIT';

INSERT INTO customer_receivable_allocations (
    id,tenant_id,company_id,customer_id,debit_item_id,credit_item_id,amount_minor,occurred_at
)
SELECT gen_random_uuid(),reversal.tenant_id,reversal.company_id,reversal.customer_id,
       invoice.id,credit.id,LEAST(invoice.amount_minor,credit.amount_minor),reversal.accounting_at
FROM sales reversal
JOIN customer_receivable_items credit ON credit.tenant_id=reversal.tenant_id AND
     credit.company_id=reversal.company_id AND credit.source_type='REVERSAL' AND credit.source_id=reversal.id
JOIN customer_receivable_items invoice ON invoice.tenant_id=reversal.tenant_id AND
     invoice.company_id=reversal.company_id AND invoice.source_type='SALE' AND invoice.source_id=reversal.reversal_of
WHERE reversal.record_type='REVERSAL' AND reversal.sale_kind='CREDIT';

ALTER TABLE customer_credit_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_receivable_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_receivable_allocations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON customer_credit_policies
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON customer_receivable_items
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON customer_receivable_allocations
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE TRIGGER customer_credit_policies_append_only
BEFORE UPDATE OR DELETE ON customer_credit_policies FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER customer_receivable_items_append_only
BEFORE UPDATE OR DELETE ON customer_receivable_items FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER customer_receivable_allocations_append_only
BEFORE UPDATE OR DELETE ON customer_receivable_allocations FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON customer_credit_policies, customer_receivable_items, customer_receivable_allocations
    FROM PUBLIC, itembaz_worker_runtime;
GRANT SELECT, INSERT ON customer_credit_policies, customer_receivable_items, customer_receivable_allocations
    TO itembaz_runtime;
