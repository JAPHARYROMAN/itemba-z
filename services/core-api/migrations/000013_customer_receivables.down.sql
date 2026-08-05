SET search_path TO itembaz, public;

DROP TRIGGER customer_credit_fields_policy_owned ON customer_accounts;
DROP TRIGGER customer_default_credit_policy ON customer_accounts;
DROP FUNCTION protect_legacy_customer_credit();
DROP FUNCTION default_customer_credit_policy();
DROP TABLE customer_receivable_allocations;
DROP FUNCTION validate_receivable_allocation();
DROP TABLE customer_receivable_items;
DROP TABLE customer_credit_policies;
DROP FUNCTION validate_customer_credit_policy();
DELETE FROM role_permissions WHERE permission_code IN ('customers.accounts.read','customers.credit.manage');
DELETE FROM permissions WHERE code IN ('customers.accounts.read','customers.credit.manage');
