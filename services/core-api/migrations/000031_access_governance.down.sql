SET search_path TO itembaz, public;

DROP TABLE IF EXISTS access_review_items;
DROP TABLE IF EXISTS access_review_campaigns;
DROP TABLE IF EXISTS access_assignment_events;

DROP INDEX IF EXISTS one_active_role_scope_assignment;

ALTER TABLE user_role_scopes
    DROP CONSTRAINT IF EXISTS user_role_scopes_tenant_id_last_reviewed_by_fkey,
    DROP CONSTRAINT IF EXISTS user_role_scopes_tenant_id_revoked_by_fkey,
    DROP CONSTRAINT IF EXISTS user_role_scopes_tenant_id_approved_by_fkey,
    DROP CONSTRAINT IF EXISTS user_role_scopes_tenant_id_granted_by_fkey,
    DROP CONSTRAINT IF EXISTS user_role_scopes_tenant_id_source_user_id_fkey,
    DROP CONSTRAINT IF EXISTS access_assignment_review_metadata,
    DROP CONSTRAINT IF EXISTS access_assignment_revocation_metadata,
    DROP CONSTRAINT IF EXISTS access_assignment_temporary_window,
    DROP CONSTRAINT IF EXISTS access_assignment_governed_metadata,
    DROP CONSTRAINT IF EXISTS access_assignment_valid_window,
    DROP COLUMN IF EXISTS last_reviewed_by,
    DROP COLUMN IF EXISTS last_reviewed_at,
    DROP COLUMN IF EXISTS revocation_reason,
    DROP COLUMN IF EXISTS revoked_by,
    DROP COLUMN IF EXISTS revoked_at,
    DROP COLUMN IF EXISTS ticket_reference,
    DROP COLUMN IF EXISTS reason,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS granted_by,
    DROP COLUMN IF EXISTS source_user_id,
    DROP COLUMN IF EXISTS valid_until,
    DROP COLUMN IF EXISTS valid_from,
    DROP COLUMN IF EXISTS assignment_type;

ALTER TABLE user_role_scopes
    ADD CONSTRAINT user_role_scopes_unique_assignment
    UNIQUE (tenant_id, user_id, role_id, company_id, branch_id, warehouse_id);

DELETE FROM permissions WHERE code IN (
    'security.access.read', 'security.access.manage',
    'security.access.review', 'security.emergency.activate'
);
