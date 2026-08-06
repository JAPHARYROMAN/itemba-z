# Platform incident runbooks

Every incident starts with an incident ID, UTC detection time, environment,
tenant/company scope, commander and evidence location. Preserve immutable ERP
records; containment never edits or deletes posted transactions. Customer,
regulator and provider communications require the authorized business,
privacy/legal or security owner.

## RB-IDENTITY — identity outage

Confirm whether the IdP, callback path, token validation or claims policy is
failing. Freeze sensitive access changes, keep header identity disabled, and
use approved time-limited break-glass access only when documented. Recover the
IdP path, revoke emergency sessions, review audit events and reconcile actions
performed during the outage.

## RB-DATABASE — database pressure or loss of service

Inspect connections, latency, locks, storage, replication and the last release
without exposing query parameters. Throttle non-critical reports and workers
before transactional posting. Do not terminate an unknown financial
transaction without database-owner review. Fail over only through the managed
service procedure, then prove journals, stock, subledgers and outbox continuity.

## RB-MIGRATION — stuck database migration

Stop further promotion, identify the exact migration and transaction state,
and preserve logs and checksums. Use the reviewed forward-fix or rollback plan;
never hand-edit schema history. Re-run compatibility tests and reconcile posted
transactions before reopening writes.

## RB-OUTBOX — outbox lag

Measure oldest unpublished event and backlog by tenant, event type and worker.
Keep core posting online when safe because the outbox is transactional. Restore
the worker or downstream destination, apply bounded replay, and verify consumer
idempotency before clearing the incident.

## RB-CONNECTOR — external connector outage

Open the circuit for the affected route, preserve durable delivery attempts and
respect provider retry limits. Core transactions remain authoritative. Move
only eligible items to governed replay after provider recovery and reconcile
provider acknowledgements with local delivery records.

## RB-FISCAL — fiscal backlog

Stop representing pending sales as fiscally accepted, preserve receipt and
request evidence, and notify the tax/compliance owner. Restore the approved TRA
path, replay through the governed delivery control, detect duplicates, and
retain professional validation of the cleared backlog.

## RB-CALLBACK — duplicate or failing payment callbacks

Validate the provider signature and timestamp before processing. Quarantine
invalid traffic, use provider event ID plus local idempotency controls, and
never create a second receipt or journal for a replay. Reconcile settlement,
payment, sale and callback audit evidence before closure.

## RB-DEVICE — compromised or failing POS device

Suspend the device and its offline authorization, revoke sessions, preserve its
last sync and allocation evidence, and prevent further offline posting. Review
client transaction IDs for gaps or duplicates, rotate credentials, reconcile
cash and stock, and require approved re-enrolment or secure disposal.

## RB-BREACH — suspected data breach

Activate the security and personal-data incident procedure immediately.
Contain credentials, workload identity and egress; preserve logs without
copying unnecessary personal data. Determine affected tenants, countries and
data classes. Privacy/legal owns statutory notification decisions. Restore from
verified sources and complete independent security and reconciliation review.

## RB-RECOVERY — backup, restore or reconciliation failure

Freeze promotion and follow `recovery-and-dr.md`. Escalate any missing backup,
failed restore, unmatched ledger/stock/audit/outbox count or unavailable
document version as critical. Production writes resume only after the recovery
authority and finance/inventory owners accept the recovery-point evidence.
