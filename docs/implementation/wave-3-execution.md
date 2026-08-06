# Wave 3 execution record

Updated: 2026-08-06

Wave 3 delivers external integrations and professionally approved statutory
configuration. Repository frameworks do not constitute TRA certification,
provider approval or professional tax/payroll sign-off.

Repository status: **IN PROGRESS**

Wave 3 exit gate: **OPEN**

## Slice 1 — Durable integration delivery spine

Status: repository implementation complete

Implemented evidence:

- Versioned legal-company integration routes cover TRA, payment, bank, payroll,
  email, WhatsApp and receipt-printer capabilities without embedding a provider
  selection.
- Routes require an HTTPS endpoint, a managed-secret reference, explicit
  contract version, timeout, retry budget, circuit policy, validity window and
  independent approval before activation.
- `sale.posted` and `sale.reversed` outbox events project idempotently into TRA
  delivery work only when an approved effective route exists.
- The committed ERP sale is never rolled back by an unavailable provider.
  Fiscal state changes are narrowly controlled and attempt evidence is
  append-only.
- Delivery claims use expiring worker leases and `SKIP LOCKED`; requests retain
  a checksum and provider idempotency key. Expired `IN_FLIGHT` leases are
  reclaimable after worker termination and every database transition is
  constrained to the delivery state machine.
- Typed connectors receive a bounded timeout. Failures use capped exponential
  backoff with stable jitter, a finite attempt budget, dead-letter state and
  route circuit health.
- A fiscal success requires a provider reference. Provider references are
  unique per route, and response persistence is explicitly a redacted evidence
  boundary.
- The outbox worker projects integration work before marking the domain event
  published, so a projection failure remains retryable.

Verification:

- Unit tests cover successful delivery, retry, exhausted retry budget,
  unavailable connectors and sale-event projection filtering.
- The PostgreSQL cross-module acceptance test proves one posted-sale event
  creates exactly one delivery under duplicate projection, records one attempt
  and transitions the sale to `FISCALIZED` after connector acceptance.
- Migration 33 applies successfully to the running development database.

Not yet complete:

- Governed route-management, delivery-register, reconciliation and manual
  replay APIs and Control Center screens.
- The current official TRA contract, certified adapter, signing/certificate
  implementation and sandbox/certification evidence.
- Payment webhook, settlement, bank-file, payroll-output, communication and
  printer provider adapters.
- Provider dashboards, alerts, escalation contacts, data-processing decisions
  and signed business/professional reconciliations.

## Next slices

1. Expose the governed integration route and delivery/replay APIs plus the
   bilingual operations register.
2. Implement the approved TRA adapter and protected sandbox harness against the
   confirmed current contract.
3. Add payment callback and settlement reconciliation adapters.
4. Add bank import/export adapters and payroll statutory configuration packs.
5. Add governed communications and supported printer adapters.
