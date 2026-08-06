# Wave 3 execution record

Updated: 2026-08-06

Wave 3 delivers external integrations and professionally approved statutory
configuration. Repository frameworks do not constitute TRA certification,
provider approval or professional tax/payroll sign-off.

Repository status: **COMPLETE**

Wave 3 exit gate: **BLOCKED — EXTERNAL EVIDENCE REQUIRED**

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

External completion blockers:

- The current official TRA contract, certified adapter, signing/certificate
  implementation and sandbox/certification evidence.
- Payment webhook, settlement, bank-file, payroll-output, communication and
  printer provider adapters.
- Provider dashboards, alerts, escalation contacts, data-processing decisions
  and signed business/professional reconciliations.

## Slice 2 — Governed integration operations

Status: repository implementation complete

Implemented evidence:

- Five versioned OpenAPI operations expose the exact-scope integration
  workspace, immutable route creation, maker-checker transitions, circuit reset
  and dead-letter replay.
- Route activation rejects the maker. Route transitions, circuit resets and
  replay decisions retain append-only reasons, actors and timestamps plus
  audit/outbox correlation.
- Replay preserves every prior attempt and request fact while expanding only a
  fresh bounded attempt budget. It cannot replay a non-dead-letter delivery or
  use a non-active route.
- The bilingual Control Center shows route health, reconciliation totals,
  delivery/provider evidence, attempt history and permission-shaped controls.
- PostgreSQL acceptance proves maker-checker rejection, independent activation,
  circuit recovery, replay and reconciliation without weakening the existing
  accounting, stock, tenancy or idempotency evidence.
- CI validates the fail-closed Wave 3 evidence register. No provider is marked
  integrated or certified without approved external evidence.

## External execution required to close the release gate

1. Implement the approved TRA adapter and protected sandbox harness against the
   confirmed current contract.
2. Add payment callback and settlement reconciliation adapters.
3. Add bank import/export adapters and payroll statutory configuration packs.
4. Add governed communications and supported printer adapters.
5. Complete provider dashboards, outage drills, reconciliations and accountable
   sign-off using the selected providers and production organization facts.
