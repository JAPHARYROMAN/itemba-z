SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('operations.read','View scoped sales, procurement, and inventory documents'),
 ('sales.orders.manage','Create and advance quotations and sales orders'),
 ('purchases.requests.manage','Create and submit purchase requests'),
 ('purchases.orders.manage','Create and advance purchase orders'),
 ('purchases.receive','Post goods receipts and purchase returns'),
 ('purchases.invoices.post','Post matched supplier invoices'),
 ('purchases.payments.post','Post supplier payments'),
 ('inventory.transfers.manage','Dispatch and receive warehouse transfers'),
 ('inventory.counts.manage','Create counts and post approved variances'),
 ('inventory.adjustments.post','Post reasoned stock adjustments');
INSERT INTO permissions(code,description) VALUES ('customers.collections.post','Post and allocate customer collections');

CREATE TABLE suppliers (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 code text NOT NULL,
 name text NOT NULL,
 active boolean NOT NULL DEFAULT true,
 payment_terms_days integer NOT NULL DEFAULT 30 CHECK(payment_terms_days BETWEEN 0 AND 365),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,code),
 FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id)
);

CREATE TABLE operation_documents (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 branch_id uuid NOT NULL,
 warehouse_id uuid NOT NULL,
 number text NOT NULL,
 document_type text NOT NULL CHECK(document_type IN ('QUOTATION','SALES_ORDER','PURCHASE_REQUEST','PURCHASE_ORDER','GOODS_RECEIPT','SUPPLIER_INVOICE','SUPPLIER_PAYMENT','PURCHASE_RETURN','STOCK_TRANSFER','STOCK_COUNT','STOCK_ADJUSTMENT')),
 status text NOT NULL CHECK(status IN ('DRAFT','SUBMITTED','APPROVED','REJECTED','POSTED','DISPATCHED','RECEIVED','CLOSED','REVERSED')),
 party_type text NOT NULL CHECK(party_type IN ('CUSTOMER','SUPPLIER','NONE')),
 party_id uuid,
 source_document_id uuid,
 destination_warehouse_id uuid,
 currency char(3) NOT NULL,
 subtotal_minor bigint NOT NULL CHECK(subtotal_minor>=0),
 total_minor bigint NOT NULL CHECK(total_minor=subtotal_minor),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 submitted_at timestamptz,
 approved_at timestamptz,
 posted_at timestamptz,
 correlation_id uuid NOT NULL,
 idempotency_key text NOT NULL CHECK(length(idempotency_key) BETWEEN 16 AND 128),
 request_hash char(64) NOT NULL CHECK(request_hash ~ '^[0-9a-f]{64}$'),
 UNIQUE(tenant_id,company_id,id),
 UNIQUE(tenant_id,company_id,number),
 UNIQUE(tenant_id,company_id,idempotency_key),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,company_id,source_document_id) REFERENCES operation_documents(tenant_id,company_id,id),
 CHECK((party_type='NONE')=(party_id IS NULL)),
 CHECK((document_type='STOCK_TRANSFER')=(destination_warehouse_id IS NOT NULL))
);
CREATE INDEX operation_documents_scope_list ON operation_documents(tenant_id,company_id,branch_id,warehouse_id,created_at DESC,id);

CREATE TABLE operation_document_lines (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 document_id uuid NOT NULL,
 product_id uuid NOT NULL,
 quantity bigint NOT NULL,
 unit_price_minor bigint NOT NULL CHECK(unit_price_minor>=0),
 amount_minor bigint NOT NULL CHECK(amount_minor>=0),
 UNIQUE(tenant_id,company_id,document_id,product_id),
 FOREIGN KEY(tenant_id,company_id,document_id) REFERENCES operation_documents(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,product_id) REFERENCES products(tenant_id,company_id,id),
 CHECK(amount_minor=abs(quantity)*unit_price_minor)
);

CREATE TABLE operation_document_transitions (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 document_id uuid NOT NULL,
 from_status text NOT NULL,
 to_status text NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 actor_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL,
 correlation_id uuid NOT NULL,
 UNIQUE(tenant_id,company_id,document_id,to_status),
 FOREIGN KEY(tenant_id,company_id,document_id) REFERENCES operation_documents(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);

ALTER TABLE sales ADD COLUMN source_document_id uuid;
ALTER TABLE sales ADD CONSTRAINT sales_source_document_fk FOREIGN KEY(tenant_id,company_id,source_document_id) REFERENCES operation_documents(tenant_id,company_id,id);
CREATE UNIQUE INDEX sales_source_document_unique ON sales(tenant_id,company_id,source_document_id) WHERE source_document_id IS NOT NULL AND record_type='SALE';

CREATE FUNCTION protect_sale_source_document() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.source_document_id IS DISTINCT FROM NEW.source_document_id THEN RAISE EXCEPTION 'sale source document is immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER sale_source_document_guard BEFORE UPDATE ON sales FOR EACH ROW EXECUTE FUNCTION protect_sale_source_document();

CREATE TABLE inventory_reservation_ledger (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL,
 branch_id uuid NOT NULL, warehouse_id uuid NOT NULL, product_id uuid NOT NULL,
 source_type text NOT NULL, source_id uuid NOT NULL, quantity bigint NOT NULL CHECK(quantity<>0),
 occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id),
 FOREIGN KEY(tenant_id,company_id,product_id) REFERENCES products(tenant_id,company_id,id)
);
CREATE INDEX inventory_reservations_balance ON inventory_reservation_ledger(tenant_id,company_id,branch_id,warehouse_id,product_id);

CREATE TABLE supplier_ledger (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, supplier_id uuid NOT NULL,
 source_type text NOT NULL, source_id uuid NOT NULL, amount_minor bigint NOT NULL CHECK(amount_minor<>0),
 currency char(3) NOT NULL, occurred_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,source_type,source_id),
 FOREIGN KEY(tenant_id,company_id,supplier_id) REFERENCES suppliers(tenant_id,company_id,id)
);

CREATE TABLE supplier_payable_items (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, supplier_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('INVOICE','CREDIT_NOTE','PAYMENT')),
 source_type text NOT NULL, source_id uuid NOT NULL, amount_minor bigint NOT NULL CHECK(amount_minor>0),
 currency char(3) NOT NULL, document_at timestamptz NOT NULL, due_at timestamptz,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,source_type,source_id),
 FOREIGN KEY(tenant_id,company_id,supplier_id) REFERENCES suppliers(tenant_id,company_id,id),
 CHECK((kind='INVOICE' AND due_at IS NOT NULL) OR (kind<>'INVOICE' AND due_at IS NULL))
);

CREATE TABLE supplier_payable_allocations (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, supplier_id uuid NOT NULL,
 debit_item_id uuid NOT NULL, credit_item_id uuid NOT NULL, amount_minor bigint NOT NULL CHECK(amount_minor>0), occurred_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,debit_item_id,credit_item_id),
 FOREIGN KEY(tenant_id,company_id,debit_item_id) REFERENCES supplier_payable_items(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,credit_item_id) REFERENCES supplier_payable_items(tenant_id,company_id,id)
);

CREATE TABLE supplier_payments (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, supplier_id uuid NOT NULL,
 supplier_invoice_id uuid NOT NULL, account_id text NOT NULL, method text NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), currency char(3) NOT NULL, occurred_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,supplier_invoice_id,id),
 FOREIGN KEY(tenant_id,company_id,supplier_id) REFERENCES suppliers(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,supplier_invoice_id) REFERENCES operation_documents(tenant_id,company_id,id)
);

CREATE TABLE procurement_posting_config (
 tenant_id uuid NOT NULL, company_id uuid NOT NULL,
 grni_account_id text NOT NULL, payable_account_id text NOT NULL,
 inventory_adjustment_account_id text NOT NULL, stock_in_transit_account_id text NOT NULL,
 cash_accounts jsonb NOT NULL CHECK(jsonb_typeof(cash_accounts)='object'),
 PRIMARY KEY(tenant_id,company_id), FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id)
);

CREATE TABLE customer_collections (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, customer_id uuid NOT NULL, invoice_sale_id uuid NOT NULL,
 account_id text NOT NULL, method text NOT NULL, amount_minor bigint NOT NULL CHECK(amount_minor>0), currency char(3) NOT NULL,
 occurred_at timestamptz NOT NULL, created_by uuid NOT NULL, correlation_id uuid NOT NULL,
 UNIQUE(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,customer_id) REFERENCES customer_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,invoice_sale_id) REFERENCES sales(tenant_id,company_id,id), FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id)
);

CREATE FUNCTION protect_operation_document() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'DELETE' THEN
		RAISE EXCEPTION 'operation documents cannot be deleted';
	END IF;
 IF (OLD.tenant_id,OLD.company_id,OLD.branch_id,OLD.warehouse_id,OLD.number,OLD.document_type,OLD.party_type,OLD.party_id,
     OLD.source_document_id,OLD.destination_warehouse_id,OLD.currency,OLD.subtotal_minor,OLD.total_minor,OLD.reason,OLD.created_by,OLD.created_at,
     OLD.correlation_id,OLD.idempotency_key,OLD.request_hash) IS DISTINCT FROM
    (NEW.tenant_id,NEW.company_id,NEW.branch_id,NEW.warehouse_id,NEW.number,NEW.document_type,NEW.party_type,NEW.party_id,
     NEW.source_document_id,NEW.destination_warehouse_id,NEW.currency,NEW.subtotal_minor,NEW.total_minor,NEW.reason,NEW.created_by,NEW.created_at,
     NEW.correlation_id,NEW.idempotency_key,NEW.request_hash) THEN
   RAISE EXCEPTION 'operation document facts are immutable';
 END IF;
 IF OLD.status IN ('POSTED','RECEIVED','CLOSED','REVERSED','REJECTED') THEN
   RAISE EXCEPTION 'terminal operation documents are immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER operation_document_guard BEFORE UPDATE OR DELETE ON operation_documents FOR EACH ROW EXECUTE FUNCTION protect_operation_document();

CREATE FUNCTION validate_supplier_allocation() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,itembaz AS $$
DECLARE debit_kind text; credit_kind text; debit_amount bigint; credit_amount bigint; debit_used bigint; credit_used bigint;
BEGIN
 SELECT kind,amount_minor INTO debit_kind,debit_amount FROM supplier_payable_items WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND supplier_id=NEW.supplier_id AND id=NEW.debit_item_id FOR UPDATE;
 SELECT kind,amount_minor INTO credit_kind,credit_amount FROM supplier_payable_items WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND supplier_id=NEW.supplier_id AND id=NEW.credit_item_id FOR UPDATE;
 IF debit_kind IS DISTINCT FROM 'INVOICE' OR credit_kind NOT IN ('CREDIT_NOTE','PAYMENT') THEN RAISE EXCEPTION 'invalid supplier allocation' USING ERRCODE='23514'; END IF;
 SELECT COALESCE(sum(amount_minor),0) INTO debit_used FROM supplier_payable_allocations WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND debit_item_id=NEW.debit_item_id;
 SELECT COALESCE(sum(amount_minor),0) INTO credit_used FROM supplier_payable_allocations WHERE tenant_id=NEW.tenant_id AND company_id=NEW.company_id AND credit_item_id=NEW.credit_item_id;
 IF debit_used+NEW.amount_minor>debit_amount OR credit_used+NEW.amount_minor>credit_amount THEN RAISE EXCEPTION 'supplier allocation exceeds open balance' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER supplier_allocation_guard BEFORE INSERT ON supplier_payable_allocations FOR EACH ROW EXECUTE FUNCTION validate_supplier_allocation();

DO $$ DECLARE table_name text; BEGIN
 FOREACH table_name IN ARRAY ARRAY['suppliers','operation_documents','operation_document_lines','operation_document_transitions','inventory_reservation_ledger','supplier_ledger','supplier_payable_items','supplier_payable_allocations','supplier_payments','procurement_posting_config','customer_collections'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',table_name);
 END LOOP;
END $$;

CREATE TRIGGER operation_lines_append_only BEFORE UPDATE OR DELETE ON operation_document_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER operation_transitions_append_only BEFORE UPDATE OR DELETE ON operation_document_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER reservation_ledger_append_only BEFORE UPDATE OR DELETE ON inventory_reservation_ledger FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER supplier_ledger_append_only BEFORE UPDATE OR DELETE ON supplier_ledger FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER supplier_payable_items_append_only BEFORE UPDATE OR DELETE ON supplier_payable_items FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER supplier_payable_allocations_append_only BEFORE UPDATE OR DELETE ON supplier_payable_allocations FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER supplier_payments_append_only BEFORE UPDATE OR DELETE ON supplier_payments FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER customer_collections_append_only BEFORE UPDATE OR DELETE ON customer_collections FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON suppliers,operation_documents,operation_document_lines,operation_document_transitions,inventory_reservation_ledger,supplier_ledger,supplier_payable_items,supplier_payable_allocations,supplier_payments,procurement_posting_config,customer_collections FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON suppliers,operation_documents TO itembaz_runtime;
GRANT SELECT,INSERT ON operation_document_lines,operation_document_transitions,inventory_reservation_ledger,supplier_ledger,supplier_payable_items,supplier_payable_allocations,supplier_payments TO itembaz_runtime;
GRANT SELECT ON procurement_posting_config TO itembaz_runtime;
GRANT SELECT,INSERT ON customer_collections TO itembaz_runtime;
