SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('finance.bank.read','View scoped cash and bank accounts and reconciliations'),
 ('finance.bank.import','Import immutable bank statements'),
 ('finance.bank.match','Match statement lines to general-ledger cash entries'),
 ('finance.bank.reconcile','Approve a fully matched bank reconciliation');

CREATE TABLE bank_accounts (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 branch_id uuid NOT NULL,
 warehouse_id uuid NOT NULL,
 code text NOT NULL,
 name text NOT NULL,
 account_type text NOT NULL CHECK(account_type IN ('BANK','CASH','MOBILE_MONEY')),
 currency char(3) NOT NULL,
 gl_account_id text NOT NULL,
 active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,company_id,id),
 UNIQUE(tenant_id,company_id,branch_id,warehouse_id,id),
 UNIQUE(tenant_id,company_id,branch_id,warehouse_id,code),
 UNIQUE(tenant_id,company_id,branch_id,warehouse_id,gl_account_id),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id)
);

CREATE TABLE bank_statements (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 branch_id uuid NOT NULL,
 warehouse_id uuid NOT NULL,
 account_id uuid NOT NULL,
 external_reference text NOT NULL CHECK(length(btrim(external_reference)) BETWEEN 3 AND 120),
 currency char(3) NOT NULL,
 period_start timestamptz NOT NULL,
 period_end timestamptz NOT NULL CHECK(period_end>period_start),
 opening_minor bigint NOT NULL,
 closing_minor bigint NOT NULL,
 imported_by uuid NOT NULL,
 imported_at timestamptz NOT NULL,
 correlation_id uuid NOT NULL,
 idempotency_key text NOT NULL CHECK(length(idempotency_key) BETWEEN 16 AND 128),
 request_hash char(64) NOT NULL CHECK(request_hash ~ '^[0-9a-f]{64}$'),
 UNIQUE(tenant_id,company_id,id),
 UNIQUE(tenant_id,company_id,account_id,external_reference),
 UNIQUE(tenant_id,company_id,idempotency_key),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id,account_id) REFERENCES bank_accounts(tenant_id,company_id,branch_id,warehouse_id,id),
 FOREIGN KEY(tenant_id,imported_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id)
);
CREATE INDEX bank_statements_scope_list ON bank_statements(tenant_id,company_id,branch_id,warehouse_id,imported_at DESC,id);

CREATE TABLE bank_statement_lines (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 statement_id uuid NOT NULL,
 transaction_at timestamptz NOT NULL,
 external_reference text NOT NULL DEFAULT '' CHECK(length(external_reference)<=120),
 description text NOT NULL CHECK(length(btrim(description)) BETWEEN 3 AND 300),
 amount_minor bigint NOT NULL CHECK(amount_minor<>0),
 UNIQUE(tenant_id,company_id,statement_id,id),
 FOREIGN KEY(tenant_id,company_id,statement_id) REFERENCES bank_statements(tenant_id,company_id,id)
);

CREATE TABLE bank_statement_matches (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 statement_id uuid NOT NULL,
 statement_line_id uuid NOT NULL,
 journal_line_id bigint NOT NULL,
 actor_id uuid NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 occurred_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,statement_line_id),
 UNIQUE(tenant_id,company_id,journal_line_id),
 FOREIGN KEY(tenant_id,company_id,statement_id,statement_line_id) REFERENCES bank_statement_lines(tenant_id,company_id,statement_id,id),
 FOREIGN KEY(journal_line_id) REFERENCES journal_lines(id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);

CREATE TABLE bank_statement_reconciliations (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 statement_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 occurred_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,statement_id),
 FOREIGN KEY(tenant_id,company_id,statement_id) REFERENCES bank_statements(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);

CREATE FUNCTION validate_bank_statement_total() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,itembaz AS $$
DECLARE movement bigint; opening_balance bigint; closing_balance bigint;
BEGIN
 SELECT COALESCE(sum(amount_minor),0),s.opening_minor,s.closing_minor INTO movement,opening_balance,closing_balance
 FROM bank_statement_lines l RIGHT JOIN bank_statements s ON s.tenant_id=l.tenant_id AND s.company_id=l.company_id AND s.id=l.statement_id
 WHERE s.tenant_id=NEW.tenant_id AND s.company_id=NEW.company_id AND s.id=NEW.id GROUP BY s.opening_minor,s.closing_minor;
 IF opening_balance+movement<>closing_balance THEN RAISE EXCEPTION 'bank statement does not balance' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER bank_statement_total_guard AFTER INSERT ON bank_statement_lines DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_bank_statement_total();

DO $$ DECLARE table_name text; BEGIN
 FOREACH table_name IN ARRAY ARRAY['bank_accounts','bank_statements','bank_statement_lines','bank_statement_matches','bank_statement_reconciliations'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',table_name);
 END LOOP;
END $$;

CREATE TRIGGER bank_statement_guard BEFORE UPDATE OR DELETE ON bank_statements FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER bank_statement_line_guard BEFORE UPDATE OR DELETE ON bank_statement_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER bank_statement_match_guard BEFORE UPDATE OR DELETE ON bank_statement_matches FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER bank_statement_reconciliation_guard BEFORE UPDATE OR DELETE ON bank_statement_reconciliations FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON bank_accounts,bank_statements,bank_statement_lines,bank_statement_matches,bank_statement_reconciliations FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT ON bank_accounts TO itembaz_runtime;
GRANT SELECT,INSERT ON bank_statements,bank_statement_lines,bank_statement_matches,bank_statement_reconciliations TO itembaz_runtime;
