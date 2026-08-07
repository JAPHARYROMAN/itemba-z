# ADR-0002: Durable external integration spine

Date: 2026-08-06

Status: accepted

## Context

Release 1 must integrate TRA fiscalization, payments, banks, payroll outputs,
communications and receipt printers. These systems have independent uptime,
authentication, retry, duplicate and reconciliation behavior. A provider
timeout must never roll back or corrupt a committed ERP transaction, while a
lost or duplicated delivery must remain detectable and recoverable.

## Decision

External work is projected idempotently from the transactional outbox into a
provider-neutral delivery ledger. A versioned, independently approved route
selects the provider contract and stores only a managed-secret reference.
Delivery workers claim rows through bounded leases, call a typed connector with
a route-specific timeout, retain an append-only attempt, and transition the job
to success, scheduled retry or dead letter.

Retry delay uses capped exponential backoff with stable jitter. Route health
retains consecutive failures and an open-until circuit state. Provider
references are unique per route. Request facts are immutable; responses must be
redacted before persistence. Manual replay will be a separately authorized,
audited command and may not edit an earlier attempt.

For TRA fiscalization, `sale.posted` and `sale.reversed` events become work only
when an active, effective route exists. The sale remains posted regardless of
provider availability. Its fiscal state may follow only the controlled
`NOT_CONFIGURED → PENDING → FISCALIZED|FAILED` transitions, with
`FAILED → PENDING` reserved for governed replay.

## Consequences

- Provider adapters cannot write stock, subledgers or journals.
- Duplicate outbox publication cannot duplicate delivery work.
- Circuit state, dead letters and attempts are tenant-isolated operational
  evidence rather than log-only diagnostics.
- Production provider protocols, certificates and sandbox evidence remain
  separate Wave 3 deliverables.
- The modular boundary can later consume Kafka without changing the ERP sale or
  delivery state machine.

This decision applies the NEXT Constitution Sections 3, 5 and 6: modular event
boundaries, resilience, observability, zero-trust secret custody, explicit data
contracts and testable infrastructure.
