SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('finance.budgets.read','View legal-company budgets and budget-versus-actual results'),
 ('finance.budgets.manage','Create and submit legal-company budgets'),
 ('finance.budgets.approve','Independently approve or reject legal-company budgets'),
 ('finance.assets.read','View the scoped fixed-asset register and depreciation evidence'),
 ('finance.assets.manage','Create and submit fixed assets'),
 ('finance.assets.approve','Independently capitalize or reject fixed assets'),
 ('finance.assets.depreciate','Post controlled fixed-asset depreciation'),
 ('finance.assets.dispose','Post controlled fixed-asset disposals');

CREATE TABLE budgets (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 3 AND 120), fiscal_year integer NOT NULL CHECK(fiscal_year BETWEEN 2000 AND 2200),
 currency char(3) NOT NULL, status text NOT NULL CHECK(status IN('DRAFT','SUBMITTED','APPROVED','REJECTED')),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), created_by uuid NOT NULL, created_at timestamptz NOT NULL,
 approved_by uuid, approved_at timestamptz, UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,name,fiscal_year),
 FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id)
);
CREATE TABLE budget_lines (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, budget_id uuid NOT NULL,
 account_id text NOT NULL, month date NOT NULL CHECK(EXTRACT(day FROM month)=1), amount_minor bigint NOT NULL CHECK(amount_minor>=0),
 UNIQUE(tenant_id,company_id,budget_id,account_id,month),
 FOREIGN KEY(tenant_id,company_id,budget_id) REFERENCES budgets(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,account_id) REFERENCES gl_accounts(tenant_id,company_id,id)
);
CREATE TABLE budget_transitions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, budget_id uuid NOT NULL,
 from_status text NOT NULL, to_status text NOT NULL, reason text NOT NULL, actor_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id,budget_id) REFERENCES budgets(tenant_id,company_id,id), FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);

CREATE TABLE fixed_assets (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, branch_id uuid NOT NULL, warehouse_id uuid NOT NULL,
 code text NOT NULL CHECK(length(btrim(code)) BETWEEN 2 AND 40), name text NOT NULL CHECK(length(btrim(name)) BETWEEN 3 AND 160), category text NOT NULL CHECK(length(btrim(category)) BETWEEN 2 AND 80),
 status text NOT NULL CHECK(status IN('DRAFT','SUBMITTED','ACTIVE','REJECTED','DISPOSED')), currency char(3) NOT NULL, acquired_at timestamptz NOT NULL,
 cost_minor bigint NOT NULL CHECK(cost_minor>0), residual_minor bigint NOT NULL CHECK(residual_minor>=0 AND residual_minor<cost_minor), useful_life_months integer NOT NULL CHECK(useful_life_months BETWEEN 1 AND 1200),
 accumulated_depreciation_minor bigint NOT NULL DEFAULT 0 CHECK(accumulated_depreciation_minor>=0),
 asset_account_id text NOT NULL, accumulated_depreciation_account_id text NOT NULL, depreciation_expense_account_id text NOT NULL,
 capitalization_offset_account_id text NOT NULL, disposal_gain_account_id text NOT NULL, disposal_loss_account_id text NOT NULL,
 capitalization_journal_id uuid, disposal_journal_id uuid, reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 created_by uuid NOT NULL, created_at timestamptz NOT NULL, approved_by uuid, approved_at timestamptz, disposed_by uuid, disposed_at timestamptz,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,code),
 FOREIGN KEY(tenant_id,company_id,branch_id,warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,disposed_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,company_id,asset_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,accumulated_depreciation_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,depreciation_expense_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,capitalization_offset_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,disposal_gain_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,disposal_loss_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(capitalization_journal_id) REFERENCES journals(id), FOREIGN KEY(disposal_journal_id) REFERENCES journals(id)
);
CREATE INDEX fixed_assets_scope_list ON fixed_assets(tenant_id,company_id,branch_id,warehouse_id,status,code);
CREATE TABLE fixed_asset_transitions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, asset_id uuid NOT NULL,
 from_status text NOT NULL, to_status text NOT NULL, reason text NOT NULL, actor_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id,asset_id) REFERENCES fixed_assets(tenant_id,company_id,id), FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE fixed_asset_depreciation (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, asset_id uuid NOT NULL, period date NOT NULL CHECK(EXTRACT(day FROM period)=1),
 amount_minor bigint NOT NULL CHECK(amount_minor>0), journal_id uuid NOT NULL, posted_by uuid NOT NULL, posted_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,asset_id,period), FOREIGN KEY(tenant_id,company_id,asset_id) REFERENCES fixed_assets(tenant_id,company_id,id),
 FOREIGN KEY(journal_id) REFERENCES journals(id), FOREIGN KEY(tenant_id,posted_by) REFERENCES users(tenant_id,id)
);

CREATE FUNCTION protect_advanced_finance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION '% cannot be deleted',TG_TABLE_NAME USING ERRCODE='55000'; END IF;
 IF TG_TABLE_NAME='budgets' AND to_jsonb(NEW)-ARRAY['status','approved_by','approved_at'] <> to_jsonb(OLD)-ARRAY['status','approved_by','approved_at'] THEN RAISE EXCEPTION 'budget facts are immutable' USING ERRCODE='55000'; END IF;
 IF TG_TABLE_NAME='fixed_assets' AND to_jsonb(NEW)-ARRAY['status','accumulated_depreciation_minor','capitalization_journal_id','disposal_journal_id','approved_by','approved_at','disposed_by','disposed_at'] <> to_jsonb(OLD)-ARRAY['status','accumulated_depreciation_minor','capitalization_journal_id','disposal_journal_id','approved_by','approved_at','disposed_by','disposed_at'] THEN RAISE EXCEPTION 'asset facts are immutable' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER budgets_guard BEFORE UPDATE OR DELETE ON budgets FOR EACH ROW EXECUTE FUNCTION protect_advanced_finance();
CREATE TRIGGER fixed_assets_guard BEFORE UPDATE OR DELETE ON fixed_assets FOR EACH ROW EXECUTE FUNCTION protect_advanced_finance();
CREATE TRIGGER budget_lines_guard BEFORE UPDATE OR DELETE ON budget_lines FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER budget_transitions_guard BEFORE UPDATE OR DELETE ON budget_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER fixed_asset_transitions_guard BEFORE UPDATE OR DELETE ON fixed_asset_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER fixed_asset_depreciation_guard BEFORE UPDATE OR DELETE ON fixed_asset_depreciation FOR EACH ROW EXECUTE FUNCTION reject_mutation();

DO $$ DECLARE table_name text; BEGIN
 FOREACH table_name IN ARRAY ARRAY['budgets','budget_lines','budget_transitions','fixed_assets','fixed_asset_transitions','fixed_asset_depreciation'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',table_name);
 END LOOP;
END $$;
REVOKE ALL ON budgets,budget_lines,budget_transitions,fixed_assets,fixed_asset_transitions,fixed_asset_depreciation FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON budgets,fixed_assets TO itembaz_runtime;
GRANT SELECT,INSERT ON budget_lines,budget_transitions,fixed_asset_transitions,fixed_asset_depreciation TO itembaz_runtime;
GRANT USAGE,SELECT ON SEQUENCE budget_lines_id_seq TO itembaz_runtime;
