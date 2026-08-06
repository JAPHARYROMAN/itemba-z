SET search_path TO itembaz, public;

INSERT INTO permissions(code, description) VALUES
    ('security.access.read', 'Inspect access assignments and review evidence in an assigned organizational scope'),
    ('security.access.manage', 'Request and execute governed access lifecycle changes'),
    ('security.access.review', 'Attest periodic access reviews without granting operational permissions'),
    ('security.emergency.activate', 'Activate approved, time-bound emergency access')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE user_role_scopes
    ADD COLUMN assignment_type text NOT NULL DEFAULT 'STANDARD'
        CHECK (assignment_type IN ('STANDARD', 'DELEGATED', 'BREAK_GLASS')),
    ADD COLUMN valid_from timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN valid_until timestamptz,
    ADD COLUMN source_user_id uuid,
    ADD COLUMN granted_by uuid,
    ADD COLUMN approved_by uuid,
    ADD COLUMN reason text,
    ADD COLUMN ticket_reference text,
    ADD COLUMN revoked_at timestamptz,
    ADD COLUMN revoked_by uuid,
    ADD COLUMN revocation_reason text,
    ADD COLUMN last_reviewed_at timestamptz,
    ADD COLUMN last_reviewed_by uuid,
    ADD CONSTRAINT access_assignment_valid_window CHECK (valid_until IS NULL OR valid_until > valid_from),
    ADD CONSTRAINT access_assignment_governed_metadata CHECK (
        (granted_by IS NULL AND approved_by IS NULL AND reason IS NULL AND ticket_reference IS NULL)
        OR
        (granted_by IS NOT NULL AND approved_by IS NOT NULL AND granted_by <> approved_by
         AND length(btrim(reason)) BETWEEN 8 AND 500
         AND length(btrim(ticket_reference)) BETWEEN 3 AND 200)
    ),
    ADD CONSTRAINT access_assignment_temporary_window CHECK (
        (assignment_type = 'STANDARD' AND source_user_id IS NULL)
        OR
        (assignment_type = 'DELEGATED' AND source_user_id IS NOT NULL AND valid_until IS NOT NULL
         AND valid_until <= valid_from + interval '30 days')
        OR
        (assignment_type = 'BREAK_GLASS' AND source_user_id IS NULL AND valid_until IS NOT NULL
         AND valid_until <= valid_from + interval '2 hours')
    ),
    ADD CONSTRAINT access_assignment_revocation_metadata CHECK (
        (revoked_at IS NULL AND revoked_by IS NULL AND revocation_reason IS NULL)
        OR
        (revoked_at IS NOT NULL AND revoked_by IS NOT NULL
         AND length(btrim(revocation_reason)) BETWEEN 8 AND 500)
    ),
    ADD CONSTRAINT access_assignment_review_metadata CHECK (
        (last_reviewed_at IS NULL AND last_reviewed_by IS NULL)
        OR (last_reviewed_at IS NOT NULL AND last_reviewed_by IS NOT NULL)
    ),
    ADD FOREIGN KEY (tenant_id, source_user_id) REFERENCES users(tenant_id, id),
    ADD FOREIGN KEY (tenant_id, granted_by) REFERENCES users(tenant_id, id),
    ADD FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    ADD FOREIGN KEY (tenant_id, revoked_by) REFERENCES users(tenant_id, id),
    ADD FOREIGN KEY (tenant_id, last_reviewed_by) REFERENCES users(tenant_id, id);

DO $$
DECLARE
    existing_unique text;
BEGIN
    SELECT conname INTO existing_unique
    FROM pg_constraint
    WHERE conrelid = 'user_role_scopes'::regclass
      AND contype = 'u'
      AND pg_get_constraintdef(oid) LIKE 'UNIQUE (tenant_id, user_id, role_id, company_id, branch_id, warehouse_id)%';
    IF existing_unique IS NOT NULL THEN
        EXECUTE format('ALTER TABLE user_role_scopes DROP CONSTRAINT %I', existing_unique);
    END IF;
END;
$$;
CREATE UNIQUE INDEX one_active_role_scope_assignment
    ON user_role_scopes(tenant_id, user_id, role_id, company_id, branch_id, warehouse_id)
    WHERE revoked_at IS NULL;

CREATE TABLE access_assignment_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    assignment_id uuid,
    target_user_id uuid NOT NULL,
    event_type text NOT NULL CHECK (event_type IN (
        'USER_ENABLED', 'USER_DISABLED', 'ASSIGNMENT_GRANTED', 'ASSIGNMENT_REVOKED',
        'DELEGATION_ACTIVATED', 'BREAK_GLASS_ACTIVATED', 'ACCESS_REVIEW_ATTESTED'
    )),
    actor_id uuid NOT NULL,
    approver_id uuid NOT NULL,
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 500),
    ticket_reference text NOT NULL CHECK (length(btrim(ticket_reference)) BETWEEN 3 AND 200),
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(evidence) = 'object'),
    correlation_id uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, target_user_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, approver_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (assignment_id) REFERENCES user_role_scopes(id),
    CHECK (actor_id <> approver_id)
);
CREATE INDEX access_assignment_events_target ON access_assignment_events(tenant_id, target_user_id, occurred_at DESC);

CREATE TABLE access_review_campaigns (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    company_id uuid NOT NULL,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 3 AND 200),
    status text NOT NULL CHECK (status IN ('OPEN', 'COMPLETED', 'CANCELLED')),
    due_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    approved_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    CHECK (created_by <> approved_by),
    CHECK ((status = 'COMPLETED') = (completed_at IS NOT NULL))
);

CREATE TABLE access_review_items (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    campaign_id uuid NOT NULL REFERENCES access_review_campaigns(id),
    assignment_id uuid NOT NULL REFERENCES user_role_scopes(id),
    decision text CHECK (decision IN ('RETAIN', 'REVOKE')),
    decided_by uuid,
    reason text,
    decided_at timestamptz,
    UNIQUE (campaign_id, assignment_id),
    FOREIGN KEY (tenant_id, decided_by) REFERENCES users(tenant_id, id),
    CHECK (
        (decision IS NULL AND decided_by IS NULL AND reason IS NULL AND decided_at IS NULL)
        OR
        (decision IS NOT NULL AND decided_by IS NOT NULL AND length(btrim(reason)) BETWEEN 8 AND 500 AND decided_at IS NOT NULL)
    )
);

GRANT SELECT ON access_assignment_events, access_review_campaigns, access_review_items TO itembaz_runtime;
