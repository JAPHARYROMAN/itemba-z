SET search_path TO itembaz, public;
DROP TABLE report_exports;
DROP INDEX reporting_journal_account;
DROP INDEX reporting_journal_date;
DELETE FROM permissions WHERE code IN ('reports.financial.read','reports.financial.export');
