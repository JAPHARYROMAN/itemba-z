SET search_path TO itembaz, public;

DROP INDEX IF EXISTS one_mobile_sale_per_client_transaction;
ALTER TABLE sales
    DROP CONSTRAINT IF EXISTS sales_mobile_device_scope_fkey,
    DROP CONSTRAINT IF EXISTS sales_offline_requires_mobile_cash,
    DROP CONSTRAINT IF EXISTS sales_mobile_identity_pair,
    DROP COLUMN IF EXISTS fiscal_status,
    DROP COLUMN IF EXISTS receipt_reference,
    DROP COLUMN IF EXISTS offline,
    DROP COLUMN IF EXISTS client_transaction_id,
    DROP COLUMN IF EXISTS device_id;

DROP TABLE IF EXISTS mobile_device_stock_allocations;
DROP TABLE IF EXISTS mobile_devices;

DELETE FROM role_permissions WHERE permission_code IN (
    'customers.read', 'products.read', 'mobile.devices.enroll', 'mobile.sales.sync'
);
DELETE FROM permissions WHERE code IN (
    'customers.read', 'products.read', 'mobile.devices.enroll', 'mobile.sales.sync'
);

ALTER TABLE products
    DROP COLUMN IF EXISTS master_data_version,
    DROP COLUMN IF EXISTS price_version,
    DROP COLUMN IF EXISTS base_unit_code;
ALTER TABLE customer_accounts DROP COLUMN IF EXISTS code;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
ALTER TABLE legal_companies
    DROP COLUMN IF EXISTS business_timezone,
    DROP COLUMN IF EXISTS price_version,
    DROP COLUMN IF EXISTS master_data_version;
