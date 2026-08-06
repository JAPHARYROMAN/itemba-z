# Service levels and alert policy

The machine-readable source is `infra/observability/slo-catalog.json`. All
targets are **proposed** until the Platform/SRE lead and business owners approve
them against production-like load evidence. They must not be represented as a
current production guarantee.

Each 28-day SLO has an availability/error-budget indicator and a latency
objective. Exactly-once, balanced financial and stock posting is an invariant,
not a budget that may be spent. Fast-burn alerts page the on-call owner; slow
burn alerts create a tracked corrective action. A breached error budget freezes
non-remediation changes to the affected journey until the incident commander
and business owner approve recovery.

The alert contract covers availability, latency, request errors, database
saturation, outbox age, fiscal and payment delivery, mobile sync,
reconciliation exceptions, migrations, backups and suspected breaches. The
selected telemetry platform must bind every catalog signal to a measured query,
an owned route and the referenced runbook. Alerts are not accepted on presence
alone: staging must prove firing, notification, acknowledgement and resolution.

The core API now emits bounded HTTP histograms/counters, database-pool gauges
and aggregate-only outbox, integration and reconciliation signals from its
private metrics listener. `prometheus-rules.yaml` binds every catalog alert to
a rule and `grafana-platform-dashboard.json` provides the portable operations
view. Backup, migration and security-event signals still require the selected
provider/operations platform to publish their authoritative values.

Telemetry fields are allowlisted. Authorization/cookie headers, tokens,
passwords, bank details, payroll values, document bodies and unrestricted SQL
must never be exported. Correlation uses opaque request, actor, tenant/company,
branch/warehouse, device, source-document, transaction, journal, stock-movement,
audit and outbox identifiers. Access is least-privilege and retention follows
the approved data-class schedule.
