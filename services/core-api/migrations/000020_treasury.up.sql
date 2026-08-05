SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('finance.treasury.read','View legal-company treasury facilities and balances'),
 ('finance.treasury.manage','Create and submit treasury facilities'),
 ('finance.treasury.approve','Independently approve, reject, or close treasury facilities'),
 ('finance.treasury.transact','Post governed treasury drawdowns, interest, and repayments');

CREATE TABLE treasury_facilities (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL,
 reference text NOT NULL CHECK(length(btrim(reference)) BETWEEN 2 AND 60), lender text NOT NULL CHECK(length(btrim(lender)) BETWEEN 3 AND 160),
 facility_type text NOT NULL CHECK(facility_type IN('TERM_LOAN','OVERDRAFT')), status text NOT NULL CHECK(status IN('DRAFT','SUBMITTED','ACTIVE','REJECTED','CLOSED')),
 currency char(3) NOT NULL, limit_minor bigint NOT NULL CHECK(limit_minor>0), annual_interest_basis_points bigint NOT NULL CHECK(annual_interest_basis_points BETWEEN 0 AND 100000),
 start_date date NOT NULL, maturity_date date NOT NULL CHECK(maturity_date>start_date),
 bank_account_id text NOT NULL, principal_account_id text NOT NULL, interest_expense_account_id text NOT NULL, accrued_interest_account_id text NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), created_by uuid NOT NULL, created_at timestamptz NOT NULL,
 approved_by uuid, approved_at timestamptz, closed_by uuid, closed_at timestamptz,
 UNIQUE(tenant_id,company_id,id), UNIQUE(tenant_id,company_id,reference),
 FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,approved_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,closed_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,company_id,bank_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,principal_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,interest_expense_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,company_id,accrued_interest_account_id) REFERENCES gl_accounts(tenant_id,company_id,id)
);
CREATE INDEX treasury_facilities_scope_list ON treasury_facilities(tenant_id,company_id,status,reference);

CREATE TABLE treasury_facility_transitions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, facility_id uuid NOT NULL,
 from_status text NOT NULL, to_status text NOT NULL, reason text NOT NULL, actor_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id,facility_id) REFERENCES treasury_facilities(tenant_id,company_id,id), FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);
CREATE TABLE treasury_transactions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, company_id uuid NOT NULL, facility_id uuid NOT NULL,
 transaction_type text NOT NULL CHECK(transaction_type IN('DRAWDOWN','PRINCIPAL_REPAYMENT','INTEREST_ACCRUAL','INTEREST_PAYMENT')),
 amount_minor bigint NOT NULL CHECK(amount_minor>0), occurred_at timestamptz NOT NULL, reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500),
 journal_id uuid NOT NULL, posted_by uuid NOT NULL, posted_at timestamptz NOT NULL,
 UNIQUE(tenant_id,company_id,id), FOREIGN KEY(tenant_id,company_id,facility_id) REFERENCES treasury_facilities(tenant_id,company_id,id),
 FOREIGN KEY(journal_id) REFERENCES journals(id), FOREIGN KEY(tenant_id,posted_by) REFERENCES users(tenant_id,id)
);
CREATE INDEX treasury_transactions_facility ON treasury_transactions(tenant_id,company_id,facility_id,occurred_at,id);

CREATE FUNCTION protect_treasury_facility() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'treasury facility cannot be deleted' USING ERRCODE='55000'; END IF;
 IF to_jsonb(NEW)-ARRAY['status','approved_by','approved_at','closed_by','closed_at'] <> to_jsonb(OLD)-ARRAY['status','approved_by','approved_at','closed_by','closed_at'] THEN RAISE EXCEPTION 'treasury facility facts are immutable' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER treasury_facilities_guard BEFORE UPDATE OR DELETE ON treasury_facilities FOR EACH ROW EXECUTE FUNCTION protect_treasury_facility();
CREATE TRIGGER treasury_transitions_guard BEFORE UPDATE OR DELETE ON treasury_facility_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER treasury_transactions_guard BEFORE UPDATE OR DELETE ON treasury_transactions FOR EACH ROW EXECUTE FUNCTION reject_mutation();

DO $$ DECLARE table_name text; BEGIN
 FOREACH table_name IN ARRAY ARRAY['treasury_facilities','treasury_facility_transitions','treasury_transactions'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',table_name);
  EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting(''app.tenant_id'',true),'''')::uuid)',table_name);
 END LOOP;
END $$;
REVOKE ALL ON treasury_facilities,treasury_facility_transitions,treasury_transactions FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON treasury_facilities TO itembaz_runtime;
GRANT SELECT,INSERT ON treasury_facility_transitions,treasury_transactions TO itembaz_runtime;
