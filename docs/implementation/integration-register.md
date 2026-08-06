# Integration register

Every release-1 connector must complete this record before build commitment.

| Field | Required evidence |
| --- | --- |
| Provider and business owner | Named provider, Itemba owner, support contacts, legal company scope |
| Purpose and criticality | Transaction, notification, import, reconciliation, or optional enrichment |
| Contract | Versioned API/file/device specification and sandbox access |
| Authentication | Secret owner, storage, rotation, signing, certificate lifecycle |
| Idempotency | Provider key, request hash, duplicate response, replay window |
| Failure behavior | Timeouts, retry budget, circuit break, dead-letter queue, manual replay |
| Reconciliation | Internal document, provider reference, settlement/receipt evidence, exception report |
| Security and privacy | Data classification, processor role, residency, retention, incident duty |
| Operations | Health check, rate limit, monitoring, escalation, planned maintenance |

Initial categories are TRA EFD/VFD, approved banks, mobile-money/payment methods, receipt printers, email, WhatsApp where approved, and biometric attendance where required. Failed external delivery never silently reverses a successfully posted core transaction.

## Repository delivery boundary

Migration 33 and `internal/integrations` implement the shared delivery state
machine, outbox projection, idempotency, retry/dead-letter, circuit-health and
attempt-evidence boundary. A category remains **not integrated** until its row
in this register identifies the approved provider and links the versioned
contract, protected test evidence, reconciliation and owner sign-off.
