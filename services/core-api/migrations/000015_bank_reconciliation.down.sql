SET search_path TO itembaz, public;
DROP TABLE bank_statement_reconciliations;
DROP TABLE bank_statement_matches;
DROP TABLE bank_statement_lines;
DROP FUNCTION validate_bank_statement_total();
DROP TABLE bank_statements;
DROP TABLE bank_accounts;
DELETE FROM role_permissions WHERE permission_code IN ('finance.bank.read','finance.bank.import','finance.bank.match','finance.bank.reconcile');
DELETE FROM permissions WHERE code IN ('finance.bank.read','finance.bank.import','finance.bank.match','finance.bank.reconcile');
