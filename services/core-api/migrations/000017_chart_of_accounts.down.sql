SET search_path TO itembaz, public;
DROP TABLE posting_mappings;
DROP TABLE gl_accounts;
DROP FUNCTION protect_account_governance();
DELETE FROM permissions WHERE code IN ('finance.accounts.read','finance.accounts.manage','finance.accounts.approve');
