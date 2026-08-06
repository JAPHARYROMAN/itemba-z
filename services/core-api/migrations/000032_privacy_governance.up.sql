SET search_path TO itembaz, public;

INSERT INTO permissions(code, description) VALUES
    ('privacy.cases.read', 'Inspect privacy-rights case evidence in the assigned legal company'),
    ('privacy.cases.manage', 'Record verified privacy-rights case decisions and fulfillment evidence'),
    ('privacy.holds.manage', 'Create and release approved legal holds'),
    ('privacy.retention.manage', 'Approve effective-dated retention and disposal rules')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE privacy_retention_policies (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    company_id uuid NOT NULL,
    data_class text NOT NULL CHECK (data_class IN ('INTERNAL', 'RESTRICTED', 'HIGHLY_RESTRICTED')),
    data_set text NOT NULL CHECK (length(btrim(data_set)) BETWEEN 3 AND 100),
    trigger_event text NOT NULL CHECK (length(btrim(trigger_event)) BETWEEN 3 AND 200),
    retain_for_days integer NOT NULL CHECK (retain_for_days BETWEEN 1 AND 36500),
    authority_reference text NOT NULL CHECK (length(btrim(authority_reference)) BETWEEN 3 AND 500),
    disposal_method text NOT NULL CHECK (length(btrim(disposal_method)) BETWEEN 3 AND 200),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_by uuid NOT NULL,
    approved_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    UNIQUE (tenant_id, company_id, data_set, effective_from),
    CHECK (created_by <> approved_by),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE TABLE privacy_legal_holds (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    company_id uuid NOT NULL,
    reference text NOT NULL CHECK (length(btrim(reference)) BETWEEN 3 AND 200),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 8 AND 1000),
    selector jsonb NOT NULL CHECK (jsonb_typeof(selector) = 'object' AND selector <> '{}'::jsonb),
    created_by uuid NOT NULL,
    approved_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    review_due_at timestamptz NOT NULL,
    released_at timestamptz,
    released_by uuid,
    release_approved_by uuid,
    release_reason text,
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, released_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, release_approved_by) REFERENCES users(tenant_id, id),
    UNIQUE (tenant_id, company_id, reference),
    CHECK (created_by <> approved_by),
    CHECK (review_due_at > created_at),
    CHECK (
        (released_at IS NULL AND released_by IS NULL AND release_approved_by IS NULL AND release_reason IS NULL)
        OR
        (released_at IS NOT NULL AND released_by IS NOT NULL AND release_approved_by IS NOT NULL
         AND released_by <> release_approved_by AND length(btrim(release_reason)) BETWEEN 8 AND 1000)
    )
);

CREATE TABLE privacy_cases (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    company_id uuid NOT NULL,
    request_type text NOT NULL CHECK (request_type IN ('ACCESS', 'CORRECTION', 'RESTRICTION', 'OBJECTION', 'PORTABILITY', 'DELETION')),
    status text NOT NULL CHECK (status IN ('RECEIVED', 'IDENTITY_VERIFIED', 'IN_REVIEW', 'FULFILLED', 'PARTIALLY_FULFILLED', 'REJECTED', 'CANCELLED')),
    subject_reference_hash text NOT NULL CHECK (length(subject_reference_hash) BETWEEN 32 AND 200),
    received_at timestamptz NOT NULL,
    due_at timestamptz NOT NULL,
    owner_id uuid NOT NULL,
    reviewer_id uuid NOT NULL,
    closed_at timestamptz,
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, reviewer_id) REFERENCES users(tenant_id, id),
    CHECK (owner_id <> reviewer_id),
    CHECK (due_at > received_at),
    CHECK ((status IN ('FULFILLED', 'PARTIALLY_FULFILLED', 'REJECTED', 'CANCELLED')) = (closed_at IS NOT NULL))
);

CREATE TABLE privacy_case_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    case_id uuid NOT NULL REFERENCES privacy_cases(id),
    event_type text NOT NULL CHECK (event_type IN ('RECEIVED', 'IDENTITY_VERIFIED', 'SEARCH_COMPLETED', 'LEGAL_REVIEWED', 'RESPONSE_APPROVED', 'DELIVERED', 'CLOSED')),
    actor_id uuid NOT NULL,
    detail text NOT NULL CHECK (length(btrim(detail)) BETWEEN 8 AND 2000),
    evidence_reference text CHECK (evidence_reference IS NULL OR length(btrim(evidence_reference)) BETWEEN 3 AND 500),
    correlation_id uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, actor_id) REFERENCES users(tenant_id, id)
);
CREATE INDEX privacy_case_events_case ON privacy_case_events(tenant_id, case_id, occurred_at);

CREATE TABLE privacy_disposal_manifests (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    company_id uuid NOT NULL,
    retention_policy_id uuid NOT NULL REFERENCES privacy_retention_policies(id),
    selector_hash text NOT NULL CHECK (length(selector_hash) BETWEEN 32 AND 200),
    record_count bigint NOT NULL CHECK (record_count >= 0),
    legal_hold_check_at timestamptz NOT NULL,
    disposed_at timestamptz NOT NULL,
    executed_by uuid NOT NULL,
    approved_by uuid NOT NULL,
    provider_evidence_reference text NOT NULL CHECK (length(btrim(provider_evidence_reference)) BETWEEN 3 AND 500),
    correlation_id uuid NOT NULL,
    FOREIGN KEY (tenant_id, company_id) REFERENCES legal_companies(tenant_id, id),
    FOREIGN KEY (tenant_id, executed_by) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, approved_by) REFERENCES users(tenant_id, id),
    CHECK (executed_by <> approved_by),
    CHECK (disposed_at >= legal_hold_check_at)
);

CREATE TABLE privacy_processor_versions (
    id uuid PRIMARY KEY,
    processor_key text NOT NULL,
    legal_name text NOT NULL,
    role text NOT NULL CHECK (role IN ('PROCESSOR', 'SUBPROCESSOR', 'INDEPENDENT_CONTROLLER')),
    purposes text[] NOT NULL CHECK (cardinality(purposes) > 0),
    data_classes text[] NOT NULL CHECK (cardinality(data_classes) > 0),
    processing_regions text[] NOT NULL CHECK (cardinality(processing_regions) > 0),
    agreement_reference text NOT NULL,
    transfer_assessment_reference text,
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    approved_by text NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (processor_key, effective_from),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

GRANT SELECT ON privacy_retention_policies, privacy_legal_holds, privacy_cases,
    privacy_case_events, privacy_disposal_manifests, privacy_processor_versions
TO itembaz_runtime;
