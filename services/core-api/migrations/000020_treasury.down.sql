SET search_path TO itembaz, public;
DROP TABLE IF EXISTS treasury_transactions,treasury_facility_transitions,treasury_facilities;
DROP FUNCTION IF EXISTS protect_treasury_facility();
DELETE FROM role_permissions WHERE permission_code IN('finance.treasury.read','finance.treasury.manage','finance.treasury.approve','finance.treasury.transact');
DELETE FROM permissions WHERE code IN('finance.treasury.read','finance.treasury.manage','finance.treasury.approve','finance.treasury.transact');
