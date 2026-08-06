# Wave 4 execution — production platform and operational resilience

Repository status: **COMPLETE**
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
- The Go runtimes now use credential-redacting structured logs. HTTP telemetry
  excludes raw URLs and exposes bounded route/status/latency metrics through a
  mandatory private listener outside local development.
- Aggregate database saturation, outbox, integration and reconciliation
  signals are exposed through an audited, aggregate-only database function.
- Production-shaped OpenTelemetry, Prometheus alert and Grafana dashboard
  contracts are versioned and configuration-validated.
- Scheduled/manual GitHub exercises now prove deterministic PostgreSQL
  dump/restore reconciliation across migrations, journals, stock, subledgers,
  audit, outbox, integrations and document metadata. Pull requests also prove
  the previous API binary starts after candidate migrations.
- A bounded concurrent load probe retains latency/error evidence and proves
  metrics do not contain tenant identifiers. It is deliberately classified as
  isolated CI evidence rather than production capacity proof.
- The release bundle includes the recovery-manifest tool; provider deployment
  and operational acceptance interfaces are explicit and fail closed.

## Deliberately open platform work

The repository does not select or provision a production cloud. Provider and
region approval, managed service modules, registry/deployment adapter binding,
protected GitHub environment reviewers, telemetry backend and paging routes,
managed backup/PITR jobs, object-version retention, production-like
load/restore/failover/DR evidence, accountable owner approval and operational
acceptance remain open.

The promotion workflow is therefore a preflight and authorization handoff, not
a pretend deployment. It fails closed at the provider boundary until an
approved adapter is implemented and separately reviewed.

Repository completion is not production Wave 4 acceptance. The latter cannot
truthfully reach 100% until every item in
`docs/operations/operational-acceptance-checklist.md` has real retained evidence
and named owner approval.

## Exit criteria

Wave 4 completes only when the roadmap exit gate is evidenced: an attested
release candidate is promoted and rolled back without unmanaged access; load,
soak, failover, backup restore and DR exercises pass; recovered ledgers, stock,
audit, outbox and documents reconcile; and Operations accepts SLOs, alerts,
runbooks, ownership and escalation paths.
