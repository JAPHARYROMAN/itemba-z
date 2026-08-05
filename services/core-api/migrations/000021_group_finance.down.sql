SET search_path TO itembaz, public;
DROP TABLE IF EXISTS intercompany_transitions,intercompany_transactions;
DROP FUNCTION IF EXISTS protect_intercompany();
DELETE FROM role_permissions WHERE permission_code IN('finance.intercompany.read','finance.intercompany.manage','finance.intercompany.approve','finance.consolidation.read');
DELETE FROM permissions WHERE code IN('finance.intercompany.read','finance.intercompany.manage','finance.intercompany.approve','finance.consolidation.read');
