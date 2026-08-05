SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('finance.intercompany.read','View intercompany transactions involving the assigned legal company'),
 ('finance.intercompany.manage','Create and submit source-company intercompany transactions'),
 ('finance.intercompany.approve','Approve intercompany transactions for the assigned source or counterparty company'),
 ('finance.consolidation.read','View tenant-group consolidated financial results and eliminations');

CREATE TABLE intercompany_transactions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, source_company_id uuid NOT NULL, source_branch_id uuid NOT NULL, source_warehouse_id uuid NOT NULL,
 counterparty_company_id uuid NOT NULL CHECK(counterparty_company_id<>source_company_id), reference text NOT NULL CHECK(length(btrim(reference)) BETWEEN 2 AND 60),
 transaction_type text NOT NULL CHECK(transaction_type IN('CASH_TRANSFER','COST_ALLOCATION')), status text NOT NULL CHECK(status IN('DRAFT','SUBMITTED','SOURCE_APPROVED','POSTED','REJECTED')),
 currency char(3) NOT NULL, amount_minor bigint NOT NULL CHECK(amount_minor>0), occurred_at timestamptz NOT NULL,
 source_debit_account_id text NOT NULL, source_credit_account_id text NOT NULL, counterparty_debit_account_id text NOT NULL, counterparty_credit_account_id text NOT NULL,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 8 AND 500), created_by uuid NOT NULL, created_at timestamptz NOT NULL,
 source_approved_by uuid, source_approved_at timestamptz, posted_by uuid, posted_at timestamptz, source_journal_id uuid, counterparty_journal_id uuid,
 UNIQUE(tenant_id,source_company_id,id), UNIQUE(tenant_id,source_company_id,reference),
 FOREIGN KEY(tenant_id,source_company_id,source_branch_id,source_warehouse_id) REFERENCES warehouses(tenant_id,company_id,branch_id,id),
 FOREIGN KEY(tenant_id,counterparty_company_id) REFERENCES legal_companies(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,source_approved_by) REFERENCES users(tenant_id,id), FOREIGN KEY(tenant_id,posted_by) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,source_company_id,source_debit_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,source_company_id,source_credit_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,counterparty_company_id,counterparty_debit_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(tenant_id,counterparty_company_id,counterparty_credit_account_id) REFERENCES gl_accounts(tenant_id,company_id,id),
 FOREIGN KEY(source_journal_id) REFERENCES journals(id), FOREIGN KEY(counterparty_journal_id) REFERENCES journals(id)
);
CREATE INDEX intercompany_source_queue ON intercompany_transactions(tenant_id,source_company_id,status,created_at DESC);
CREATE INDEX intercompany_counterparty_queue ON intercompany_transactions(tenant_id,counterparty_company_id,status,created_at DESC);
CREATE TABLE intercompany_transitions (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, source_company_id uuid NOT NULL, transaction_id uuid NOT NULL,
 acting_company_id uuid NOT NULL, from_status text NOT NULL, to_status text NOT NULL, reason text NOT NULL, actor_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,source_company_id,transaction_id) REFERENCES intercompany_transactions(tenant_id,source_company_id,id), FOREIGN KEY(tenant_id,acting_company_id) REFERENCES legal_companies(tenant_id,id), FOREIGN KEY(tenant_id,actor_id) REFERENCES users(tenant_id,id)
);

CREATE FUNCTION protect_intercompany() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'intercompany transaction cannot be deleted' USING ERRCODE='55000'; END IF;
 IF to_jsonb(NEW)-ARRAY['status','source_approved_by','source_approved_at','posted_by','posted_at','source_journal_id','counterparty_journal_id'] <> to_jsonb(OLD)-ARRAY['status','source_approved_by','source_approved_at','posted_by','posted_at','source_journal_id','counterparty_journal_id'] THEN RAISE EXCEPTION 'intercompany facts are immutable' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER intercompany_guard BEFORE UPDATE OR DELETE ON intercompany_transactions FOR EACH ROW EXECUTE FUNCTION protect_intercompany();
CREATE TRIGGER intercompany_transitions_guard BEFORE UPDATE OR DELETE ON intercompany_transitions FOR EACH ROW EXECUTE FUNCTION reject_mutation();
ALTER TABLE intercompany_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE intercompany_transitions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON intercompany_transactions USING (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON intercompany_transitions USING (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid);
REVOKE ALL ON intercompany_transactions,intercompany_transitions FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT,UPDATE ON intercompany_transactions TO itembaz_runtime;
GRANT SELECT,INSERT ON intercompany_transitions TO itembaz_runtime;
