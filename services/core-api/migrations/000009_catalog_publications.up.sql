SET search_path TO itembaz, public;

-- A publication is the immutable, server-owned evidence behind one mobile
-- catalog token. Dynamic stock, credit exposure, fiscal periods, and device
-- limits intentionally remain live controls at posting time.
CREATE TABLE catalog_publications (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    catalog_snapshot_token uuid NOT NULL,
    master_data_version bigint NOT NULL CHECK (master_data_version > 0),
    price_version bigint NOT NULL CHECK (price_version > 0),
    published_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, company_id, catalog_snapshot_token),
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    CHECK (catalog_snapshot_token <> '00000000-0000-0000-0000-000000000000')
);

CREATE TABLE catalog_publication_customers (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    catalog_snapshot_token uuid NOT NULL,
    customer_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL,
    is_general boolean NOT NULL,
    credit_enabled boolean NOT NULL,
    credit_limit_minor bigint NOT NULL CHECK (credit_limit_minor >= 0),
    PRIMARY KEY (tenant_id, company_id, catalog_snapshot_token, customer_id),
    FOREIGN KEY (tenant_id, company_id, catalog_snapshot_token)
        REFERENCES catalog_publications(tenant_id, company_id, catalog_snapshot_token),
    CHECK (NOT is_general OR (NOT credit_enabled AND credit_limit_minor = 0))
);

CREATE TABLE catalog_publication_products (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    catalog_snapshot_token uuid NOT NULL,
    product_id uuid NOT NULL,
    sku text NOT NULL,
    name text NOT NULL,
    base_unit_code text NOT NULL,
    active boolean NOT NULL,
    currency char(3) NOT NULL,
    list_price_minor bigint NOT NULL CHECK (list_price_minor > 0),
    standard_cost_minor bigint NOT NULL CHECK (standard_cost_minor >= 0),
    tax_code text NOT NULL,
    revenue_account_id text NOT NULL,
    cogs_account_id text NOT NULL,
    inventory_account_id text NOT NULL,
    price_version bigint NOT NULL CHECK (price_version > 0),
    master_data_version bigint NOT NULL CHECK (master_data_version > 0),
    PRIMARY KEY (tenant_id, company_id, catalog_snapshot_token, product_id),
    FOREIGN KEY (tenant_id, company_id, catalog_snapshot_token)
        REFERENCES catalog_publications(tenant_id, company_id, catalog_snapshot_token)
);

CREATE TABLE catalog_publication_tax_rules (
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    catalog_snapshot_token uuid NOT NULL,
    tax_rule_id uuid NOT NULL,
    code text NOT NULL,
    basis_points integer NOT NULL CHECK (basis_points BETWEEN 0 AND 10000),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    PRIMARY KEY (tenant_id, company_id, catalog_snapshot_token, tax_rule_id),
    FOREIGN KEY (tenant_id, company_id, catalog_snapshot_token)
        REFERENCES catalog_publications(tenant_id, company_id, catalog_snapshot_token),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE INDEX catalog_publication_tax_rules_effective_lookup
    ON catalog_publication_tax_rules (
        tenant_id, company_id, catalog_snapshot_token, code, effective_from DESC
    );

-- Preserve the publication that is current at migration time. Subsequent
-- publications are captured by the API while holding the same version lock
-- used by governed customer, product, price, and tax mutations.
INSERT INTO catalog_publications (
    tenant_id, company_id, catalog_snapshot_token, master_data_version, price_version
)
SELECT tenant_id, id, catalog_snapshot_token, master_data_version, price_version
FROM legal_companies;

INSERT INTO catalog_publication_customers (
    tenant_id, company_id, catalog_snapshot_token, customer_id, code, name,
    active, is_general, credit_enabled, credit_limit_minor
)
SELECT c.tenant_id, c.company_id, company.catalog_snapshot_token, c.id, c.code,
       c.name, c.active, c.is_general, c.credit_enabled, c.credit_limit_minor
FROM customer_accounts c
JOIN legal_companies company
  ON company.tenant_id = c.tenant_id AND company.id = c.company_id;

INSERT INTO catalog_publication_products (
    tenant_id, company_id, catalog_snapshot_token, product_id, sku, name,
    base_unit_code, active, currency, list_price_minor, standard_cost_minor,
    tax_code, revenue_account_id, cogs_account_id, inventory_account_id,
    price_version, master_data_version
)
SELECT p.tenant_id, p.company_id, company.catalog_snapshot_token, p.id, p.sku,
       p.name, p.base_unit_code, p.active, p.currency, p.list_price_minor,
       p.standard_cost_minor, p.tax_code, p.revenue_account_id,
       p.cogs_account_id, p.inventory_account_id, p.price_version,
       p.master_data_version
FROM products p
JOIN legal_companies company
  ON company.tenant_id = p.tenant_id AND company.id = p.company_id;

INSERT INTO catalog_publication_tax_rules (
    tenant_id, company_id, catalog_snapshot_token, tax_rule_id, code,
    basis_points, effective_from, effective_to
)
SELECT r.tenant_id, r.company_id, company.catalog_snapshot_token, r.id, r.code,
       r.basis_points, r.effective_from, r.effective_to
FROM tax_rules r
JOIN legal_companies company
  ON company.tenant_id = r.tenant_id AND company.id = r.company_id;

ALTER TABLE catalog_publications ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_publication_customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_publication_products ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_publication_tax_rules ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON catalog_publications
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON catalog_publication_customers
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON catalog_publication_products
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON catalog_publication_tax_rules
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE TRIGGER catalog_publications_append_only
BEFORE UPDATE OR DELETE ON catalog_publications
FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER catalog_publication_customers_append_only
BEFORE UPDATE OR DELETE ON catalog_publication_customers
FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER catalog_publication_products_append_only
BEFORE UPDATE OR DELETE ON catalog_publication_products
FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER catalog_publication_tax_rules_append_only
BEFORE UPDATE OR DELETE ON catalog_publication_tax_rules
FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON catalog_publications FROM PUBLIC, itembaz_worker_runtime;
REVOKE ALL ON catalog_publication_customers FROM PUBLIC, itembaz_worker_runtime;
REVOKE ALL ON catalog_publication_products FROM PUBLIC, itembaz_worker_runtime;
REVOKE ALL ON catalog_publication_tax_rules FROM PUBLIC, itembaz_worker_runtime;
GRANT SELECT, INSERT ON catalog_publications TO itembaz_runtime;
GRANT SELECT, INSERT ON catalog_publication_customers TO itembaz_runtime;
GRANT SELECT, INSERT ON catalog_publication_products TO itembaz_runtime;
GRANT SELECT, INSERT ON catalog_publication_tax_rules TO itembaz_runtime;
