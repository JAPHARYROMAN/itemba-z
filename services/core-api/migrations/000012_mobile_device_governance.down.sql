SET search_path TO itembaz, public;

REVOKE UPDATE (status, authorization_epoch) ON mobile_devices FROM itembaz_runtime;
REVOKE UPDATE (allocated_quantity, updated_at) ON mobile_device_stock_allocations FROM itembaz_runtime;
REVOKE INSERT ON mobile_device_stock_allocations FROM itembaz_runtime;
REVOKE ALL ON mobile_device_allocation_changes FROM itembaz_runtime;
REVOKE ALL ON mobile_device_status_changes FROM itembaz_runtime;
DROP TABLE mobile_device_allocation_changes;
DROP TABLE mobile_device_status_changes;
ALTER TABLE mobile_device_offline_leases
    DROP CONSTRAINT mobile_device_offline_leases_pkey,
    ADD PRIMARY KEY (
        tenant_id, device_id, app_version, master_data_version,
        price_version, valid_from, valid_until
    ),
    DROP COLUMN authorization_epoch;
ALTER TABLE mobile_devices DROP COLUMN authorization_epoch;
DELETE FROM role_permissions WHERE permission_code IN ('mobile.devices.read','mobile.devices.manage');
DELETE FROM permissions WHERE code IN ('mobile.devices.read','mobile.devices.manage');
