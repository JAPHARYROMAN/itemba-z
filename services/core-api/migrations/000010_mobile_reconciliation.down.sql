SET search_path TO itembaz, public;

REVOKE ALL ON mobile_reconciliation_resolutions FROM itembaz_runtime;
REVOKE ALL ON mobile_reconciliation_cases FROM itembaz_runtime;
DROP TABLE mobile_reconciliation_resolutions;
DROP TABLE mobile_reconciliation_cases;
DELETE FROM role_permissions WHERE permission_code IN ('mobile.reconciliation.read', 'mobile.reconciliation.resolve');
DELETE FROM permissions WHERE code IN ('mobile.reconciliation.read', 'mobile.reconciliation.resolve');
