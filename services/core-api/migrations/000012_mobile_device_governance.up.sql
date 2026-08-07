SET search_path TO itembaz, public;

INSERT INTO permissions (code, description) VALUES
    ('mobile.devices.read', 'View enrolled POS devices and offline allocations in an assigned scope'),
    ('mobile.devices.manage', 'Suspend or reactivate POS devices and govern offline stock allocations');

ALTER TABLE mobile_devices
    ADD COLUMN authorization_epoch uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE mobile_device_offline_leases
    ADD COLUMN authorization_epoch uuid;
ALTER TABLE mobile_device_offline_leases DISABLE TRIGGER mobile_device_offline_leases_append_only;
UPDATE mobile_device_offline_leases lease
SET authorization_epoch = device.authorization_epoch
FROM mobile_devices device
WHERE device.tenant_id = lease.tenant_id AND device.id = lease.device_id;
ALTER TABLE mobile_device_offline_leases ENABLE TRIGGER mobile_device_offline_leases_append_only;
ALTER TABLE mobile_device_offline_leases
    ALTER COLUMN authorization_epoch SET NOT NULL,
    DROP CONSTRAINT mobile_device_offline_leases_pkey,
    ADD PRIMARY KEY (
        tenant_id, device_id, app_version, master_data_version,
        price_version, valid_from, valid_until, authorization_epoch
    );

CREATE TABLE mobile_device_status_changes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    device_id uuid NOT NULL,
    previous_status text NOT NULL CHECK (previous_status IN ('ACTIVE','SUSPENDED')),
    new_status text NOT NULL CHECK (new_status IN ('ACTIVE','SUSPENDED')),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    changed_by uuid NOT NULL,
    correlation_id uuid NOT NULL,
    changed_at timestamptz NOT NULL,
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    request_hash char(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    UNIQUE (tenant_id, company_id, idempotency_key),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, device_id)
        REFERENCES mobile_devices(tenant_id, company_id, branch_id, warehouse_id, id),
    FOREIGN KEY (tenant_id, changed_by) REFERENCES users(tenant_id, id),
    CHECK (previous_status <> new_status)
);

CREATE TABLE mobile_device_allocation_changes (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    company_id uuid NOT NULL,
    branch_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    device_id uuid NOT NULL,
    product_id uuid NOT NULL,
    previous_quantity bigint NOT NULL CHECK (previous_quantity >= 0),
    new_quantity bigint NOT NULL CHECK (new_quantity >= 0),
    consumed_quantity bigint NOT NULL CHECK (consumed_quantity >= 0),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    changed_by uuid NOT NULL,
    correlation_id uuid NOT NULL,
    changed_at timestamptz NOT NULL,
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    request_hash char(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    UNIQUE (tenant_id, company_id, idempotency_key),
    FOREIGN KEY (tenant_id, company_id, branch_id, warehouse_id, device_id)
        REFERENCES mobile_devices(tenant_id, company_id, branch_id, warehouse_id, id),
    FOREIGN KEY (tenant_id, company_id, product_id) REFERENCES products(tenant_id, company_id, id),
    FOREIGN KEY (tenant_id, changed_by) REFERENCES users(tenant_id, id),
    CHECK (new_quantity >= consumed_quantity)
);

ALTER TABLE mobile_device_status_changes ENABLE ROW LEVEL SECURITY;
ALTER TABLE mobile_device_allocation_changes ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mobile_device_status_changes
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON mobile_device_allocation_changes
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE TRIGGER mobile_device_status_changes_append_only
BEFORE UPDATE OR DELETE ON mobile_device_status_changes
FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER mobile_device_allocation_changes_append_only
BEFORE UPDATE OR DELETE ON mobile_device_allocation_changes
FOR EACH ROW EXECUTE FUNCTION reject_mutation();

REVOKE ALL ON mobile_device_status_changes FROM PUBLIC, itembaz_worker_runtime;
REVOKE ALL ON mobile_device_allocation_changes FROM PUBLIC, itembaz_worker_runtime;
GRANT SELECT, INSERT ON mobile_device_status_changes TO itembaz_runtime;
GRANT SELECT, INSERT ON mobile_device_allocation_changes TO itembaz_runtime;
GRANT INSERT ON mobile_device_stock_allocations TO itembaz_runtime;
GRANT UPDATE (allocated_quantity, updated_at) ON mobile_device_stock_allocations TO itembaz_runtime;
GRANT UPDATE (status, authorization_epoch) ON mobile_devices TO itembaz_runtime;
