SET search_path TO itembaz, public;
DROP FUNCTION IF EXISTS protect_commercial_status_only() CASCADE;
DROP TABLE IF EXISTS sourcing_awards, supplier_quote_lines, supplier_quotes, rfq_lines, rfqs, master_data_transitions, master_data_revisions CASCADE;
ALTER TABLE suppliers DROP COLUMN IF EXISTS tax_id, DROP COLUMN IF EXISTS email, DROP COLUMN IF EXISTS phone;
DELETE FROM role_permissions WHERE permission_code IN ('masterdata.read','masterdata.suppliers.manage','masterdata.products.manage','masterdata.approve','purchases.sourcing.read','purchases.sourcing.manage','purchases.sourcing.approve');
DELETE FROM permissions WHERE code IN ('masterdata.read','masterdata.suppliers.manage','masterdata.products.manage','masterdata.approve','purchases.sourcing.read','purchases.sourcing.manage','purchases.sourcing.approve');
