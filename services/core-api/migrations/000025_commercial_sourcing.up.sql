SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('masterdata.read','View governed supplier and product master data'),
 ('masterdata.suppliers.manage','Create supplier master revisions'),
 ('masterdata.products.manage','Create product master revisions'),
 ('masterdata.approve','Independently approve master-data revisions'),
 ('purchases.sourcing.read','View RFQs, supplier quotations, comparisons, and awards'),
 ('purchases.sourcing.manage','Create and submit RFQs and supplier quotations'),
 ('purchases.sourcing.approve','Independently approve RFQs and award purchase orders');

ALTER TABLE suppliers ADD COLUMN tax_id text NOT NULL DEFAULT '', ADD COLUMN email text NOT NULL DEFAULT '', ADD COLUMN phone text NOT NULL DEFAULT '';
CREATE TABLE master_data_revisions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL,
 entity_type text NOT NULL CHECK(entity_type IN ('SUPPLIER','PRODUCT')), entity_id uuid NOT NULL,
 status text NOT NULL CHECK(status IN ('DRAFT','SUBMITTED','ACTIVE','REJECTED')),
 supplier_data jsonb, product_data jsonb,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 created_by uuid NOT NULL, created_at timestamptz NOT NULL, approved_by uuid, approved_at timestamptz,
 UNIQUE(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id),
 CHECK((entity_type='SUPPLIER' AND supplier_data IS NOT NULL AND product_data IS NULL) OR (entity_type='PRODUCT' AND product_data IS NOT NULL AND supplier_data IS NULL))
);
CREATE INDEX master_data_revisions_scope ON master_data_revisions(tenant_id,company_id,created_at DESC);
CREATE UNIQUE INDEX one_active_master_revision ON master_data_revisions(tenant_id,company_id,entity_type,entity_id) WHERE status='ACTIVE';

CREATE TABLE master_data_transitions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, company_id uuid NOT NULL, revision_id uuid NOT NULL,
 from_status text NOT NULL, to_status text NOT NULL, reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 actor_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id,revision_id) REFERENCES master_data_revisions(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);

CREATE TABLE rfqs (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, branch_id uuid NOT NULL, warehouse_id uuid NOT NULL,
 number text NOT NULL, status text NOT NULL CHECK(status IN ('DRAFT','SUBMITTED','APPROVED','CLOSED','CANCELLED')),
 currency char(3) NOT NULL, response_due_at timestamptz NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), created_by uuid NOT NULL, created_at timestamptz NOT NULL,
 approved_by uuid, approved_at timestamptz,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,number),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id)
);
CREATE TABLE rfq_lines (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, rfq_id uuid NOT NULL, product_id uuid NOT NULL, quantity bigint NOT NULL CHECK(quantity>0),
 UNIQUE(tenant_id,company_id,rfq_id,product_id), FOREIGN KEY(tenant_id,company_id,rfq_id) REFERENCES rfqs(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,product_id) REFERENCES products(tenant_id,company_id,id)
);
CREATE TABLE supplier_quotes (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, rfq_id uuid NOT NULL, supplier_id uuid NOT NULL,
 reference text NOT NULL, status text NOT NULL CHECK(status IN ('DRAFT','SUBMITTED','SELECTED','REJECTED')), currency char(3) NOT NULL,
 delivery_days integer NOT NULL CHECK(delivery_days BETWEEN 0 AND 3650), payment_terms_days integer NOT NULL CHECK(payment_terms_days BETWEEN 0 AND 3650), valid_until timestamptz NOT NULL,
 total_minor bigint NOT NULL CHECK(total_minor>0), reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), created_by uuid NOT NULL, created_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,rfq_id,supplier_id),
 FOREIGN KEY(tenant_id,company_id,rfq_id) REFERENCES rfqs(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,supplier_id) REFERENCES suppliers(tenant_id,company_id,id), FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id)
);
CREATE TABLE supplier_quote_lines (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, quote_id uuid NOT NULL, product_id uuid NOT NULL,
 quantity bigint NOT NULL CHECK(quantity>0), unit_price_minor bigint NOT NULL CHECK(unit_price_minor>0), amount_minor bigint NOT NULL CHECK(amount_minor=quantity*unit_price_minor),
 UNIQUE(tenant_id,company_id,quote_id,product_id), FOREIGN KEY(tenant_id,company_id,quote_id) REFERENCES supplier_quotes(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,product_id) REFERENCES products(tenant_id,company_id,id)
);
CREATE TABLE sourcing_awards (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, rfq_id uuid NOT NULL, quote_id uuid NOT NULL, supplier_id uuid NOT NULL, purchase_order_id uuid NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), selected_by uuid NOT NULL, selected_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,rfq_id), UNIQUE(tenant_id,company_id,quote_id), UNIQUE(tenant_id,company_id,purchase_order_id),
 FOREIGN KEY(tenant_id,company_id,rfq_id) REFERENCES rfqs(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,quote_id) REFERENCES supplier_quotes(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,supplier_id) REFERENCES suppliers(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,purchase_order_id) REFERENCES operation_documents(tenant_id,company_id,id), FOREIGN KEY(tenant_id,selected_by) REFERENCES users(tenant_id,id)
);

CREATE FUNCTION protect_commercial_status_only() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP='DELETE' THEN RAISE EXCEPTION 'commercial record cannot be deleted' USING ERRCODE='55000'; END IF; IF to_jsonb(NEW)-ARRAY['status','approved_by','approved_at']<>to_jsonb(OLD)-ARRAY['status','approved_by','approved_at'] THEN RAISE EXCEPTION 'commercial facts are immutable' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER master_revision_guard BEFORE UPDATE OR DELETE ON master_data_revisions FOR EACH ROW EXECUTE FUNCTION protect_commercial_status_only();
CREATE TRIGGER rfq_guard BEFORE UPDATE OR DELETE ON rfqs FOR EACH ROW EXECUTE FUNCTION protect_commercial_status_only();
CREATE TRIGGER supplier_quote_guard BEFORE UPDATE OR DELETE ON supplier_quotes FOR EACH ROW EXECUTE FUNCTION protect_commercial_status_only();
CREATE TRIGGER master_transition_guard BEFORE UPDATE OR DELETE ON master_data_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER rfq_lines_guard BEFORE UPDATE OR DELETE ON rfq_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER quote_lines_guard BEFORE UPDATE OR DELETE ON supplier_quote_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER sourcing_awards_guard BEFORE UPDATE OR DELETE ON sourcing_awards FOR EACH ROW EXECUTE FUNCTION reject_mutation();
DO $$ DECLARE n text; BEGIN FOREACH n IN ARRAY ARRAY['master_data_revisions','master_data_transitions','rfqs','rfq_lines','supplier_quotes','supplier_quote_lines','sourcing_awards'] LOOP EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',n); EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',n); END LOOP; END $$;
REVOKE ALL ON master_data_revisions,master_data_transitions,rfqs,rfq_lines,supplier_quotes,supplier_quote_lines,sourcing_awards FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON master_data_revisions,rfqs,supplier_quotes TO itembaz_runtime;
GRANT SELECT,INSERT ON master_data_transitions,rfq_lines,supplier_quote_lines,sourcing_awards TO itembaz_runtime;
GRANT UPDATE ON suppliers TO itembaz_runtime;
GRANT INSERT,UPDATE ON products TO itembaz_runtime;
