SET search_path TO itembaz, public;

ALTER TABLE legal_companies
    ADD COLUMN master_data_version bigint NOT NULL DEFAULT 1 CHECK (master_data_version > 0),
    ADD COLUMN price_version bigint NOT NULL DEFAULT 1 CHECK (price_version > 0),
    ADD COLUMN business_timezone text NOT NULL DEFAULT 'Africa/Dar_es_Salaam'
        CHECK (length(btrim(business_timezone)) > 0);

ALTER TABLE users ADD COLUMN display_name text NOT NULL DEFAULT '';

ALTER TABLE customer_accounts ADD COLUMN code text;
UPDATE customer_accounts SET code = 'C-' || upper(left(id::text, 8)) WHERE code IS NULL;
ALTER TABLE customer_accounts ALTER COLUMN code SET NOT NULL;
ALTER TABLE customer_accounts ADD UNIQUE (tenant_id, company_id, code);

ALTER TABLE products
    ADD COLUMN base_unit_code text NOT NULL DEFAULT 'EA' CHECK (length(btrim(base_unit_code)) > 0),
    ADD COLUMN price_version bigint NOT NULL DEFAULT 1 CHECK (price_version > 0),
    ADD COLUMN master_data_version bigint NOT NULL DEFAULT 1 CHECK (master_data_version > 0);

INSERT INTO permissions (code, description) VALUES
    ('customers.read', 'View customers in an assigned organizational scope'),
    ('products.read', 'View products and available stock in an assigned organizational scope'),
    ('mobile.devices.enroll', 'Enroll a POS device into the actor assigned organizational scope'),
    ('mobile.sales.sync', 'Synchronize POS sales from an enrolled device')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE mobile_devices (
    id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED', 'REVOKED')),
    device_name text NOT NULL CHECK (length(btrim(device_name)) > 0),
    app_version text NOT NULL CHECK (length(btrim(app_version)) > 0),
    master_data_version bigint NOT NULL CHECK (master_data_version > 0),
    price_version bigint NOT NULL CHECK (price_version > 0),
    offline_enabled boolean NOT NULL DEFAULT false,
    offline_transaction_limit_minor bigint NOT NULL DEFAULT 0 CHECK (offline_transaction_limit_minor >= 0),
    offline_daily_limit_minor bigint NOT NULL DEFAULT 0 CHECK (offline_daily_limit_minor >= 0),
    enrolled_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, company_id, branch_id, warehouse_id, id),
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id)
        REFERENCES warehouses(tenant_id, company_id, branch_id, id),
    CHECK (last_seen_at >= enrolled_at)
);

CREATE TABLE mobile_device_stock_allocations (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    device_id uuid NOT NULL,
    product_id uuid NOT NULL,
    allocated_quantity bigint NOT NULL CHECK (allocated_quantity >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, device_id, product_id),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, device_id)
        REFERENCES mobile_devices(tenant_id, company_id, branch_id, warehouse_id, id),
    FOREIGN KEY (tenant_id, company_id, product_id)
        REFERENCES products(tenant_id, company_id, id)
);

ALTER TABLE sales
    ADD COLUMN device_id uuid,
    ADD COLUMN client_transaction_id uuid,
    ADD COLUMN offline boolean NOT NULL DEFAULT false,
    ADD COLUMN receipt_reference text,
    ADD COLUMN fiscal_status text NOT NULL DEFAULT 'NOT_CONFIGURED'
        CHECK (fiscal_status IN ('NOT_CONFIGURED', 'PENDING', 'FISCALIZED', 'FAILED')),
    ADD CONSTRAINT sales_mobile_identity_pair CHECK (
        (device_id IS NULL AND client_transaction_id IS NULL) OR
        (device_id IS NOT NULL AND client_transaction_id IS NOT NULL)
    ),
    ADD CONSTRAINT sales_offline_requires_mobile_cash CHECK (
        NOT offline OR (record_type = 'SALE' AND sale_kind = 'CASH' AND device_id IS NOT NULL)
    ),
    ADD CONSTRAINT sales_mobile_device_scope_fkey
        FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, device_id)
        REFERENCES mobile_devices(tenant_id, company_id, branch_id, warehouse_id, id);

-- The 000001 immutability trigger correctly rejects ordinary posted-sale
-- updates. Disable only that trigger for this one migration backfill; the
-- migration runner wraps the file in a transaction, so failure cannot leave it
-- disabled.
ALTER TABLE sales DISABLE TRIGGER sales_immutable;
UPDATE sales SET receipt_reference = id::text WHERE receipt_reference IS NULL;
ALTER TABLE sales ENABLE TRIGGER sales_immutable;
ALTER TABLE sales ALTER COLUMN receipt_reference SET NOT NULL;
ALTER TABLE sales ADD UNIQUE (tenant_id, company_id, receipt_reference);

CREATE UNIQUE INDEX one_mobile_sale_per_client_transaction
    ON sales (tenant_id, device_id, client_transaction_id)
    WHERE device_id IS NOT NULL;

ALTER TABLE mobile_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE mobile_device_stock_allocations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mobile_devices
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON mobile_device_stock_allocations
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
