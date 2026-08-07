# Recovery and disaster-recovery control

## Proposed objectives

These values are planning proposals, not approved production commitments.

| Data/service class | Proposed RPO | Proposed RTO | Recovery authority |
| --- | ---: | ---: | --- |
| PostgreSQL ledgers, stock, subledgers, audit and outbox | 5 minutes | 2 hours | Platform/SRE plus finance and inventory owners |
| Encrypted documents and receipts | 1 hour | 4 hours | Platform/SRE plus records owner |
| Control Center and stateless workers | 0 retained state | 1 hour | Platform/SRE |
| Redis coordination/cache | no authoritative recovery | 1 hour | Platform/SRE |

The Platform/SRE lead, security lead and named business owners must approve or
replace these objectives after provider selection and a production-like test.

## Required provider controls

- Managed PostgreSQL point-in-time recovery, encrypted automated backups and an
  immutable secondary copy outside the routine runtime identity boundary.
- Encrypted, versioned object storage with lifecycle policy, legal-hold support
  and a tested manifest connecting database attachment records to object
  versions.
- Separate backup operator and restore operator identities, audited emergency
  access, protected keys and documented regional/cross-account recovery.
- Automated backup success and restore-verification signals bound to
  `ALERT-BACKUP`.

## Production-like restore exercise

1. Record exercise ID, release, source environment, target isolation boundary,
   approved recovery point and expected RTO/RPO.
2. Capture pre-recovery signed counts and hashes for journal headers/lines,
   stock movements, customer/supplier subledgers, audit events, outbox events
   and attachment version manifests.
3. Restore database and object versions into an isolated environment using
   recovery identities. No restored endpoint may reach production providers.
4. Run migrations only according to the restored release compatibility plan.
5. Reconcile balanced journals, control accounts, stock quantities/cost layers,
   subledgers, immutable audit sequence, outbox publication state and document
   checksums at the selected recovery point.
6. Exercise application start, worker replay idempotency, identity controls and
   a representative read-only report. Do not fiscalize, pay or notify externally.
7. Record actual RPO/RTO, exceptions, alert evidence and owner decisions. Securely
   dispose of the isolated recovery copy under the approved retention policy.

Any unexplained mismatch, missing document, external side effect, objective
breach or unaudited privilege makes the exercise fail. Quarterly restore and
annual disaster-recovery exercises are proposed; accountable owners must set
the final cadence.

## Automated repository exercise

`.github/workflows/recovery-assurance.yml` performs a scheduled and manual
isolated PostgreSQL dump/restore drill. The deterministic `recoverymanifest`
tool compares migration checksums, sequence state and authoritative table streams for journals,
stock, customer/supplier subledgers, audit, outbox, integration delivery and
document/export metadata. It reapplies migrations to the restored database and
proves the prior API binary starts against the candidate schema on pull requests.
Artifacts are retained for the repository maximum of 90 days.

This exercise proves the procedure and reconciliation tooling. It does not
claim managed-provider encryption, PITR, object-blob recovery, regional failure
or approved RTO/RPO; those remain required production-like evidence.
