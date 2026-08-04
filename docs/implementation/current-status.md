# Current implementation status

Updated: 2026-08-04

This repository is the first executable ITEMBA-Z foundation. It proves the architecture and the internal golden transaction without representing the full production Release 1 acceptance boundary.

## Implemented in this milestone

- Monorepo structure, CI checks, local container dependencies, provider-neutral Terraform foundations, telemetry configuration, security model, ADRs, and versioned contracts.
- Go modular core for scoped cash and credit sales, server-owned prices/tax/cost, stock effects, customer balances, balanced journals, payment recording, audit entries, idempotency, immutable reversals, and transactional outbox events.
- PostgreSQL schema, row-level security policies, append-only guards, transactional repository, migration command, and capability-separated API/outbox-worker logins; an in-memory adapter remains available for unit tests.
- OIDC/JWT production authentication with tenant, legal-company, branch, and warehouse scope derived from verified claims. Header-based identity is restricted to explicit local development mode.
- Versioned OpenAPI read and command boundaries for working context, customers, products, sales, linked reversals, device enrollment, and idempotent mobile synchronization, with checked TypeScript and Dart bindings.
- Next.js bilingual Control Center sales workspace backed only by the live API: scoped master data, sale entry, immutable register/detail, linked reversal, server-side credential forwarding, and lost-response idempotency recovery. Other module screens remain demonstrations until their APIs are built.
- Flutter Android Sales POS live transport with encrypted SQLCipher state, Android-Keystore-backed credentials, immutable catalog-snapshot download and acknowledgement, bounded server-issued cache leases, authoritative offline limits and stock allocations, durable queued sales, idempotent synchronization, and truthful internal-versus-fiscal receipt state. Offline posting is currently fail-closed except for physical CASH and zero-rated lines.
- A legal-company catalog snapshot token now rotates with governed customer, product, price, and tax publication under the acknowledgement lock. Every paginated mobile download is pinned to one token; the encrypted install, device acknowledgement, offline lease, sync command, posted sale, audit, and outbox evidence retain that identity.
- Each acknowledged token now owns append-only historical customer, product, price, cost, posting-account, and effective tax facts. Lease-authorized offline cash posts from that immutable publication after later configuration drift, while stock, limits, and fiscal-period checks remain live. Missing evidence returns `offline_reconciliation_required`; the POS preserves the exact command in a bilingual, restart-safe reconciliation state and excludes it from automatic retry.
- A reproducible local application profile plus PostgreSQL-backed CI verifies migration, development seed, API startup, an allocated offline cash sale, duplicate replay/conflict behavior, General Customer credit rejection, and linked reversal through HTTP.

## Remaining before Release 1 can be claimed

- Complete live APIs and posting rules for customer/supplier master data, purchasing, returns, stock operations and costing, AR/AP, cash/bank, budgeting, assets, treasury, intercompany, consolidation, HR, attendance, leave, loans, payroll, reports, and settings.
- Complete authoritative receivable aging and effective credit policy (overdue amount, due date, risk/approval state, and expected post-sale exposure) before enabling mobile credit-sale entry; the current POS intentionally fails closed.
- Select and integrate the production identity provider and login/session lifecycle; deploy the API and clients behind managed TLS, provision production devices, and add governed operator workflows for device suspension, allocation changes, queue support, and reconciliation.
- Implement and professionally validate TRA EFD/VFD, payment, banking-import, email, WhatsApp, payroll/statutory, privacy, hosting, and cross-border-transfer integrations.
- Add effective-dated Tanzanian configuration only after accountant, tax, payroll, and legal approval; no statutory values may be hardcoded.
- Add the server-side governed reconciliation case register, operator permissions, evidence view, approved resolution actions, and audit/outbox workflow for the rare offline command whose historical publication evidence is unavailable. The POS currently preserves and pauses such commands but cannot authorize a resolution itself.
- Define and implement the production offline document-time versus posting-time policy, including trusted clock/skew controls, fiscal-period handling, accounting dates, and reconciliation when synchronization crosses a period boundary. The current slice posts only when the server-time period is open.
- Execute source-data assessment, cleansing, two trial migrations, stock verification, opening-balance reconciliation, parallel payroll runs, training, pilot rollout, hypercare, penetration/load/recovery tests, and every gate in `release-gates.md`.
- Add production secret management, backup/restore automation, point-in-time recovery evidence, deployment promotion controls, dashboards, alerts, and operational runbooks for the selected hosting platform.

## Contract convention

OpenAPI operations marked `x-implementation-status: implemented` are available in the current Go HTTP boundary. Operations marked `planned` are reviewed forward contracts and must not be treated as deployed capabilities.
Exactly 100 minor units equal TZS 1. Public numeric money, quantity, count, and
version values stay within JavaScript's exact integer range; PostgreSQL bigint
storage does not widen the JSON contract.
