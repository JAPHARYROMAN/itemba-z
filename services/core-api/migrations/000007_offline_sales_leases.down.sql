SET search_path TO itembaz, public;

DROP TRIGGER tax_rule_offline_lease_guard ON tax_rules;
DROP FUNCTION guard_tax_rule_against_offline_leases();

REVOKE SELECT, INSERT ON mobile_device_offline_leases FROM itembaz_runtime;
DROP TRIGGER mobile_device_offline_leases_append_only ON mobile_device_offline_leases;
DROP TABLE mobile_device_offline_leases;

ALTER TABLE mobile_devices
    DROP CONSTRAINT mobile_devices_offline_sales_lease_order,
    DROP COLUMN offline_sales_valid_from,
    DROP COLUMN offline_sales_valid_until;
