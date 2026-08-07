SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('finance.journals.read','View governed financial documents and journals'),
 ('finance.journals.manage','Create and submit financial documents'),
 ('finance.journals.post','Independently reject or post financial documents'),
 ('finance.cash.transfer','Create controlled transfers between scoped cash and bank accounts'),
 ('finance.bank.adjust','Create controlled bank fee and suspense adjustments'),
 ('finance.periods.read','View fiscal periods and close evidence'),
 ('finance.periods.close','Request and independently approve fiscal-period close'),
 ('finance.periods.reopen','Request and independently approve fiscal-period reopen');

ALTER TABLE fiscal_periods ADD CONSTRAINT fiscal_periods_scope_id_unique UNIQUE(tenant_id,company_id,id);

CREATE TABLE financial_documents (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, branch_id uuid NOT NULL, warehouse_id uuid NOT NULL,
 number text NOT NULL, document_type text NOT NULL CHECK(document_type IN ('MANUAL_JOURNAL','CASH_TRANSFER','BANK_ADJUSTMENT','REVERSAL')),
 status text NOT NULL CHECK(status IN ('DRAFT','SUBMITTED','POSTED','REJECTED')), currency char(3) NOT NULL,
 accounting_at timestamptz NOT NULL, reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 from_account_id uuid, to_account_id uuid, reverses_document_id uuid, journal_id uuid,
 created_by uuid NOT NULL, created_at timestamptz NOT NULL, submitted_by uuid, posted_by uuid,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,number), UNIQUE(tenant_id,company_id,reverses_document_id),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,submitted_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,posted_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,company_id,from_account_id) REFERENCES bank_accounts(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,to_account_id) REFERENCES bank_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,reverses_document_id) REFERENCES financial_documents(tenant_id,company_id,id), FOREIGN KEY(journal_id) REFERENCES journals(id)
);
CREATE INDEX financial_documents_scope_list ON financial_documents(tenant_id,company_id,branch_id,warehouse_id,created_at DESC,id);

CREATE TABLE financial_document_lines (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, document_id uuid NOT NULL,
 account_id text NOT NULL, debit_minor bigint NOT NULL DEFAULT 0 CHECK(debit_minor>=0), credit_minor bigint NOT NULL DEFAULT 0 CHECK(credit_minor>=0), memo text NOT NULL DEFAULT '',
 CHECK((debit_minor>0)<>(credit_minor>0)), FOREIGN KEY(tenant_id,company_id,document_id) REFERENCES financial_documents(tenant_id,company_id,id)
);
CREATE TABLE financial_document_transitions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, document_id uuid NOT NULL,
 from_status text NOT NULL, to_status text NOT NULL, actor_id uuid NOT NULL, reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id,document_id) REFERENCES financial_documents(tenant_id,company_id,id), FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE fiscal_period_action_requests (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, period_id uuid NOT NULL,
 action text NOT NULL CHECK(action IN ('CLOSE','REOPEN')), reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 requested_by uuid NOT NULL, requested_at timestamptz NOT NULL, approved_by uuid, approved_at timestamptz,
 UNIQUE(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,period_id) REFERENCES fiscal_periods(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,requested_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id)
);

DO $$ DECLARE table_name text; BEGIN
 FOREACH table_name IN ARRAY ARRAY['financial_documents','financial_document_lines','financial_document_transitions','fiscal_period_action_requests'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',table_name);
 END LOOP;
END $$;
CREATE TRIGGER financial_document_line_guard BEFORE UPDATE OR DELETE ON financial_document_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER financial_document_transition_guard BEFORE UPDATE OR DELETE ON financial_document_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER fiscal_period_action_delete_guard BEFORE DELETE ON fiscal_period_action_requests FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON financial_documents,financial_document_lines,financial_document_transitions,fiscal_period_action_requests FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON financial_documents,fiscal_period_action_requests TO itembaz_runtime;
GRANT SELECT,INSERT ON financial_document_lines,financial_document_transitions TO itembaz_runtime;
GRANT USAGE,SELECT ON SEQUENCE financial_document_lines_id_seq TO itembaz_runtime;
GRANT UPDATE(is_open) ON fiscal_periods TO itembaz_runtime;
