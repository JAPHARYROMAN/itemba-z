SET search_path TO itembaz, public;

INSERT INTO permissions(code,description) VALUES
 ('reports.financial.read','Run scoped legal-company financial statements and ledger drill-down'),
 ('reports.financial.export','Generate and retain audited financial report exports');

CREATE INDEX reporting_journal_date ON journals(tenant_id,company_id,currency,occurred_at,id);
CREATE INDEX reporting_journal_account ON journal_lines(tenant_id,company_id,account_id,journal_id,id);

CREATE TABLE report_exports (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 company_id uuid NOT NULL,
 report_type text NOT NULL CHECK(report_type IN ('TRIAL_BALANCE','GENERAL_LEDGER','PROFIT_AND_LOSS','BALANCE_SHEET','CASH_FLOW')),
 filename text NOT NULL CHECK(length(filename) BETWEEN 5 AND 200),
 media_type text NOT NULL CHECK(media_type='text/csv'),
 content_base64 text NOT NULL,
 generated_by uuid NOT NULL,
 generated_at timestamptz NOT NULL,
 FOREIGN KEY(tenant_id,company_id) REFERENCES legal_companies(tenant_id,id),
 FOREIGN KEY(tenant_id,generated_by) REFERENCES users(tenant_id,id)
);
CREATE TRIGGER report_exports_append_only BEFORE UPDATE OR DELETE ON report_exports FOR EACH ROW EXECUTE FUNCTION reject_mutation();
ALTER TABLE report_exports ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON report_exports USING (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid) WITH CHECK (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid);

REVOKE ALL ON report_exports FROM PUBLIC,itembaz_worker_runtime;
GRANT SELECT,INSERT ON report_exports TO itembaz_runtime;
