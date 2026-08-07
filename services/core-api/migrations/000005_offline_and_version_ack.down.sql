SET search_path TO itembaz, public;

DROP TRIGGER tax_rule_bumps_master_data_version ON tax_rules;
DROP FUNCTION bump_master_data_version_for_tax_rule();

ALTER TABLE sales
    DROP CONSTRAINT sales_offline_requires_mobile_cash,
    ADD CONSTRAINT sales_offline_requires_mobile_cash CHECK (
        NOT offline OR (record_type = 'SALE' AND sale_kind = 'CASH' AND device_id IS NOT NULL)
    );

UPDATE mobile_devices AS device
SET master_data_version = company.master_data_version,
    price_version = company.price_version
FROM legal_companies AS company
WHERE device.tenant_id = company.tenant_id
  AND device.company_id = company.id
  AND (device.master_data_version = 0 OR device.price_version = 0);

ALTER TABLE mobile_devices
    DROP CONSTRAINT mobile_devices_master_data_version_check,
    DROP CONSTRAINT mobile_devices_price_version_check,
    ADD CONSTRAINT mobile_devices_master_data_version_check CHECK (master_data_version > 0),
    ADD CONSTRAINT mobile_devices_price_version_check CHECK (price_version > 0);
