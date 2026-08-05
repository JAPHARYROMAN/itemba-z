SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('finance.accounts.read','View the legal-company chart of accounts and posting mappings'),
 ('finance.accounts.manage','Submit chart-of-accounts and posting-mapping changes'),
 ('finance.accounts.approve','Independently approve or reject accounting configuration');

CREATE TABLE gl_accounts (
 record_id uuid NOT NULL UNIQUE, tenant_id uuid NOT NULL, company_id uuid NOT NULL, id text NOT NULL,
 code text NOT NULL CHECK(code ~ '^[a-z0-9][a-z0-9._-]{1,63}$'), name text NOT NULL CHECK(length(btrim(name)) BETWEEN 2 AND 160),
 account_type text NOT NULL CHECK(account_type IN ('ASSET','LIABILITY','EQUITY','REVENUE','EXPENSE','UNCLASSIFIED')),
 parent_account_id text, control_account boolean NOT NULL DEFAULT false, allow_manual_posting boolean NOT NULL DEFAULT false,
 status text NOT NULL CHECK(status IN ('SUBMITTED','ACTIVE','REJECTED','INACTIVE')),
 created_by uuid, created_at timestamptz NOT NULL, approved_by uuid, approved_at timestamptz,
 create_idempotency_key text, create_request_hash char(64), decision_idempotency_key text, decision_request_hash char(64),
 PRIMARY KEY(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,code),
 FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id),
 FOREIGN KEY(tenant_id,company_id,parent_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id),
 CHECK(NOT(control_account AND allow_manual_posting)),
 UNIQUE(tenant_id,company_id,create_idempotency_key), UNIQUE(tenant_id,company_id,decision_idempotency_key)
);
CREATE TABLE posting_mappings (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL,
 mapping_key text NOT NULL CHECK(mapping_key IN ('SALES_RECEIVABLE','SALES_TAX_PAYABLE','PAYMENT_CASH','PAYMENT_MOBILE_MONEY','PAYMENT_BANK_CARD','PAYMENT_BANK_TRANSFER','PROCUREMENT_GRNI','PROCUREMENT_PAYABLE','INVENTORY_ADJUSTMENT','STOCK_IN_TRANSIT')),
 account_id text NOT NULL, effective_from timestamptz NOT NULL,
 status text NOT NULL CHECK(status IN ('SUBMITTED','ACTIVE','REJECTED')),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), created_by uuid, created_at timestamptz NOT NULL, approved_by uuid, approved_at timestamptz,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,mapping_key,effective_from),
 FOREIGN KEY(tenant_id,company_id,account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id)
);

CREATE FUNCTION protect_account_governance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '% cannot be deleted',TG_TABLE_NAME USING ERRCODE='55000'; END IF;
 IF to_jsonb(NEW)-ARRAY['status','approved_by','approved_at','decision_idempotency_key','decision_request_hash'] <> to_jsonb(OLD)-ARRAY['status','approved_by','approved_at','decision_idempotency_key','decision_request_hash'] THEN RAISE EXCEPTION '% facts are immutable',TG_TABLE_NAME USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gl_account_governance_guard BEFORE UPDATE OR DELETE ON gl_accounts FOR EACH ROW EXECUTE FUNCTION protect_account_governance();
CREATE TRIGGER posting_mapping_governance_guard BEFORE UPDATE OR DELETE ON posting_mappings FOR EACH ROW EXECUTE FUNCTION protect_account_governance();

DO $$ DECLARE table_name text; BEGIN
 FOREACH table_name IN ARRAY ARRAY['gl_accounts','posting_mappings'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',table_name);
 END LOOP;
END $$;

INSERT INTO gl_accounts(record_id,tenant_id,company_id,id,code,name,account_type,control_account,allow_manual_posting,status,created_at)
SELECT gen_random_uuid(),tenant_id,company_id,account_id,account_id,initcap(replace(account_id,'-',' ')),account_type,control_account,false,'ACTIVE',now() FROM (
 SELECT DISTINCT ON (tenant_id,company_id,account_id) tenant_id,company_id,account_id,account_type,control_account FROM (
 SELECT tenant_id,company_id,receivable_account_id account_id,'ASSET' account_type,true control_account FROM sales_posting_config
 UNION SELECT tenant_id,company_id,tax_payable_account_id,'LIABILITY',true FROM sales_posting_config
 UNION SELECT tenant_id,company_id,value,'ASSET',false FROM sales_posting_config,jsonb_each_text(cash_accounts)
 UNION SELECT tenant_id,company_id,revenue_account_id,'REVENUE',false FROM products
 UNION SELECT tenant_id,company_id,cogs_account_id,'EXPENSE',false FROM products
 UNION SELECT tenant_id,company_id,inventory_account_id,'ASSET',true FROM products
 UNION SELECT tenant_id,company_id,grni_account_id,'LIABILITY',true FROM procurement_posting_config
 UNION SELECT tenant_id,company_id,payable_account_id,'LIABILITY',true FROM procurement_posting_config
 UNION SELECT tenant_id,company_id,inventory_adjustment_account_id,'EXPENSE',false FROM procurement_posting_config
 UNION SELECT tenant_id,company_id,stock_in_transit_account_id,'ASSET',true FROM procurement_posting_config
 UNION SELECT tenant_id,company_id,value,'ASSET',false FROM procurement_posting_config,jsonb_each_text(cash_accounts)
 UNION SELECT tenant_id,company_id,gl_account_id,'ASSET',false FROM bank_accounts
 ) raw ORDER BY tenant_id,company_id,account_id,control_account DESC,account_type
) known WHERE account_id ~ '^[a-z0-9][a-z0-9._-]{1,63}$' ON CONFLICT DO NOTHING;
INSERT INTO gl_accounts(record_id,tenant_id,company_id,id,code,name,account_type,control_account,allow_manual_posting,status,created_at)
SELECT gen_random_uuid(),tenant_id,company_id,account_id,account_id,initcap(replace(account_id,'-',' ')),'UNCLASSIFIED',false,false,'ACTIVE',now() FROM (SELECT DISTINCT tenant_id,company_id,account_id FROM journal_lines) journal_accounts
WHERE account_id ~ '^[a-z0-9][a-z0-9._-]{1,63}$' ON CONFLICT DO NOTHING;

INSERT INTO posting_mappings(id,tenant_id,company_id,mapping_key,account_id,effective_from,status,reason,created_at)
SELECT gen_random_uuid(),tenant_id,company_id,mapping_key,account_id,'1970-01-01T00:00:00Z','ACTIVE','Imported from legacy posting configuration',now() FROM (
 SELECT tenant_id,company_id,'SALES_RECEIVABLE' mapping_key,receivable_account_id account_id FROM sales_posting_config
 UNION ALL SELECT tenant_id,company_id,'SALES_TAX_PAYABLE',tax_payable_account_id FROM sales_posting_config
 UNION ALL SELECT tenant_id,company_id,'PAYMENT_'||key,value FROM sales_posting_config,jsonb_each_text(cash_accounts)
 UNION ALL SELECT tenant_id,company_id,'PROCUREMENT_GRNI',grni_account_id FROM procurement_posting_config
 UNION ALL SELECT tenant_id,company_id,'PROCUREMENT_PAYABLE',payable_account_id FROM procurement_posting_config
 UNION ALL SELECT tenant_id,company_id,'INVENTORY_ADJUSTMENT',inventory_adjustment_account_id FROM procurement_posting_config
 UNION ALL SELECT tenant_id,company_id,'STOCK_IN_TRANSIT',stock_in_transit_account_id FROM procurement_posting_config
) mappings WHERE mapping_key IN ('SALES_RECEIVABLE','SALES_TAX_PAYABLE','PAYMENT_CASH','PAYMENT_MOBILE_MONEY','PAYMENT_BANK_CARD','PAYMENT_BANK_TRANSFER','PROCUREMENT_GRNI','PROCUREMENT_PAYABLE','INVENTORY_ADJUSTMENT','STOCK_IN_TRANSIT') ON CONFLICT DO NOTHING;

REVOKE ALL ON gl_accounts,posting_mappings FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON gl_accounts,posting_mappings TO itembaz_runtime;
