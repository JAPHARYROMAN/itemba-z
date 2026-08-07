SET search_path TO itembaz, public;

DROP TABLE IF EXISTS privacy_processor_versions;
DROP TABLE IF EXISTS privacy_disposal_manifests;
DROP TABLE IF EXISTS privacy_case_events;
DROP TABLE IF EXISTS privacy_cases;
DROP TABLE IF EXISTS privacy_legal_holds;
DROP TABLE IF EXISTS privacy_retention_policies;

DELETE FROM permissions WHERE code IN (
    'privacy.cases.read', 'privacy.cases.manage',
    'privacy.holds.manage', 'privacy.retention.manage'
);
