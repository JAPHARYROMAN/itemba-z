# Wave 4 execution — production platform and operational resilience

Status: **IN PROGRESS**
Production exit: **BLOCKED — external platform and operating evidence required**

## Implemented repository slice

- Five deployed environment boundaries and mandatory fail-closed platform
  controls are represented in machine-readable contracts and Terraform.
- A protected-environment promotion preflight validates ordered promotion,
  immutable release/migration hashes, predecessor evidence, change authority
  and a rollback plan. It retains an authorization artifact for one year.
- Proposed critical-journey SLOs, error-budget policy and a complete alert-to-
  runbook catalog are versioned and executable through repository validation.
- Incident runbooks cover identity, database, migration, outbox, connector,
  fiscal, callback, device, breach and recovery scenarios.
- Proposed RTO/RPO values and a fail-closed production-like restore and
  reconciliation procedure are documented.

## Deliberately open platform work

The repository does not select or provision a production cloud. Provider and
region approval, managed service modules, registry/deployment adapter, protected
GitHub environment reviewers, telemetry backend and paging routes, backup/PITR
jobs, object-version retention, production-like load/restore/failover/DR
evidence, accountable owner approval and operational acceptance remain open.

The promotion workflow is therefore a preflight and authorization handoff, not
a pretend deployment. It fails closed at the provider boundary until an
approved adapter is implemented and separately reviewed.

## Exit criteria

Wave 4 completes only when the roadmap exit gate is evidenced: an attested
release candidate is promoted and rolled back without unmanaged access; load,
soak, failover, backup restore and DR exercises pass; recovered ledgers, stock,
audit, outbox and documents reconcile; and Operations accepts SLOs, alerts,
runbooks, ownership and escalation paths.
