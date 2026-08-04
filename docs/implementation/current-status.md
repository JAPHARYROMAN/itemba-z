# Current implementation status

Updated: 2026-08-04

This repository is the first executable ITEMBA-Z foundation. It proves the architecture and the internal golden transaction without representing the full production Release 1 acceptance boundary.

## Implemented in this milestone

- Monorepo structure, CI checks, local container dependencies, provider-neutral Terraform foundations, telemetry configuration, security model, ADRs, and versioned contracts.
- Go modular core for scoped cash and credit sales, server-owned prices/tax/cost, stock effects, customer balances, balanced journals, payment recording, audit entries, idempotency, immutable reversals, and transactional outbox events.
- PostgreSQL schema, row-level security policies, append-only guards, transactional repository, migration command, and outbox worker; an in-memory adapter remains available for unit tests.
- OIDC/JWT production authentication with tenant, legal-company, branch, and warehouse scope derived from verified claims. Header-based identity is restricted to explicit local development mode.
- Next.js bilingual Control Center experience across the Release 1 navigation and representative screens, backed by typed demonstration data pending live module APIs.
- Flutter Android Sales POS flow with bilingual operation, controlled cash/offline policy, credit checks, idempotent sync behavior, and encrypted local persistence foundations.

## Remaining before Release 1 can be claimed

- Complete live APIs and posting rules for customer/supplier master data, purchasing, returns, stock operations and costing, AR/AP, cash/bank, budgeting, assets, treasury, intercompany, consolidation, HR, attendance, leave, loans, payroll, reports, and settings.
- Connect the Control Center and POS to authenticated deployed APIs, generate TypeScript/Dart clients, and complete device enrollment, allocation, conflict, receipt, and fiscal synchronization workflows.
- Implement and professionally validate TRA EFD/VFD, payment, banking-import, email, WhatsApp, payroll/statutory, privacy, hosting, and cross-border-transfer integrations.
- Add effective-dated Tanzanian configuration only after accountant, tax, payroll, and legal approval; no statutory values may be hardcoded.
- Execute source-data assessment, cleansing, two trial migrations, stock verification, opening-balance reconciliation, parallel payroll runs, training, pilot rollout, hypercare, penetration/load/recovery tests, and every gate in `release-gates.md`.
- Add production secret management, backup/restore automation, point-in-time recovery evidence, deployment promotion controls, dashboards, alerts, and operational runbooks for the selected hosting platform.

## Contract convention

OpenAPI operations marked `x-implementation-status: implemented` are available in the current Go HTTP boundary. Operations marked `planned` are reviewed forward contracts and must not be treated as deployed capabilities.
