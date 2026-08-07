# Wave 4 operational acceptance checklist

This checklist is fail-closed. Repository tests may populate technical evidence,
but only named accountable owners may approve production operation.

## Platform and delivery

- [ ] Hosting provider, legal entity, primary/recovery region and data-transfer
      decision are approved and linked.
- [ ] All five environments exist from reviewed Terraform with isolated data,
      identities, secrets, networks and change authority.
- [ ] GitHub environments have named reviewers, protected branches/tags and
      workload-identity federation; no long-lived cloud credential is stored.
- [ ] The provider adapter satisfies `infra/deployment/adapter-contract.json`.
- [ ] A release is promoted configuration → test → staging → pilot → production,
      and the prior release is restored without unmanaged production access.
- [ ] Forward migration and previous-binary rollback compatibility evidence is
      retained for the release candidate.

## Observability and response

- [ ] The production collector config exports through managed TLS and the
      telemetry backend validates ingestion, retention and access control.
- [ ] Every alert-catalog signal is bound to a query, owned notification route,
      primary/secondary on-call role, severity and tested acknowledgement path.
- [ ] SLOs and error budgets are approved by Platform/SRE and business owners.
- [ ] Dashboard panels show production signals and alert drills prove firing,
      delivery, acknowledgement, escalation and resolution.
- [ ] Redaction tests pass and an authorized reviewer confirms logs contain no
      credentials, payroll values, bank details, payload bodies or raw paths.
- [ ] Identity, database, migration, outbox, connector, fiscal, callback,
      compromised-device, breach and recovery tabletop exercises are accepted.

## Recovery and resilience

- [ ] RTO/RPO values are approved per service/data class.
- [ ] Managed PostgreSQL encrypted backup, PITR and immutable secondary copy are
      enabled with separate recovery identity and alerting.
- [ ] Object versioning, retention, legal hold and attachment-manifest restore
      are implemented and verified.
- [ ] A production-like restore reconciles migrations, journals, journal lines,
      stock, customer/supplier subledgers, audit, outbox, integrations and
      document checksums at the selected recovery point.
- [ ] Load, soak, database failover, worker outage, connector outage and regional
      recovery exercises meet approved objectives with retained raw evidence.
- [ ] Exercise environments and restored data are securely disposed under the
      approved retention procedure.

## Sign-off record

The retained acceptance record must include release commit, evidence index,
open risks, decision time and signatures from Platform/SRE, security/privacy,
database, finance, inventory, Operations/support and executive release owners.
Any unchecked item keeps `release_exit_status=BLOCKED_EXTERNAL`.
