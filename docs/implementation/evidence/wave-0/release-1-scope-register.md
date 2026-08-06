# Wave 0 Release 1 scope register

- Baseline date: 2026-08-06
- Source commit: `a42e1fb5937454a1a0b7d12764d9319028747aad`
- Scope authority: Release 1 product specification, screen catalog, OpenAPI contract and executable repository

## Status definitions

| Status | Meaning |
| --- | --- |
| LIVE | Authoritative backend boundary, persistence, permission checks and executable UI exist |
| PARTIAL | Important executable capability exists, but one or more Release 1 flows or evidence gates remain |
| DEMONSTRATION | UI uses repository mock data or non-authoritative behavior and cannot be treated as production |
| PLANNED | Reviewed contract or requirement exists but the executable capability is not implemented |
| EXTERNAL | Completion depends on provider, professional, business, regulatory or source-data evidence |
| POST-LAUNCH | Explicitly excluded from Release 1 |

## Executable inventory summary

The machine-readable inventory is `repository-inventory.json` in this evidence directory.

| Surface | Count | Classification |
| --- | ---: | --- |
| OpenAPI paths | 93 | Versioned `/v1` contract |
| OpenAPI operations | 105 | 104 implemented, 1 planned |
| Control Center page routes | 20 | 15 live, 5 demonstration |
| Control Center BFF routes | 36 | Live API mediation boundaries |
| Go top-level internal modules | 26 | Modular-monolith packages, including platform |
| PostgreSQL migrations | 28 | Ordered up migrations |
| Flutter presentation entry files | 2 | Sales shell and new-sale flow |

## Release 1 capability register

| ID | Area | Release 1 capability | Status | Evidence and remaining boundary |
| --- | --- | --- | --- | --- |
| R1-001 | Shell | Governed executive dashboard | PLANNED | `/v1/dashboard` is the only planned OpenAPI operation; `/` currently reads mock dashboard data |
| R1-002 | Shell | Working context | LIVE | `/v1/context`, live context strip and verified scope claims |
| R1-003 | Shell | Global search | DEMONSTRATION | `/search` reads the mock ERP repository and must be replaced or excluded from production navigation |
| R1-004 | Shell | Approval inbox | PARTIAL | Maker-checker queues exist inside modules; one authoritative cross-module inbox is not complete |
| R1-005 | Shell | Notification center | PARTIAL | Governed configuration/outbox foundations exist; production delivery center and connectors are incomplete |
| R1-006 | Customers | Accounts, ageing, credit policy and collections | LIVE | Live API snapshots, receivable reconciliation and controlled policy/collection commands |
| R1-007 | Customers | Customer master create/profile/document workflow | PARTIAL | Customer facts exist; full live master create and duplicate/document workflow is not exposed as a dedicated Control Center flow |
| R1-008 | Suppliers | Supplier master and sourcing | PARTIAL | Backend governed supplier revisions and sourcing are live; `/suppliers` is still served by the generic mock module route |
| R1-009 | Sales | Quotation, order, reservation, collection and quick sale | LIVE | Live operations and sales workspaces with atomic posting and idempotency |
| R1-010 | Sales | Returns, credit notes and linked reversal | LIVE | Immutable linked reversal and commercial return effects implemented |
| R1-011 | Purchases | Request, RFQ, comparison, award and purchase order | LIVE | Governed sourcing plus procure-to-pay operations |
| R1-012 | Purchases | Receipt, match, invoice, payment and return | LIVE | Atomic stock, AP, journal, audit and outbox effects |
| R1-013 | Inventory | Stock, transfer, count, adjustment and reservation | LIVE | Authoritative movement ledger and governed operations |
| R1-014 | Inventory | Planning, lots, FEFO, expiry and costing | LIVE | Governed inventory-control workspace; specialized costing beyond approved Itemba policy remains subject to scope confirmation |
| R1-015 | Finance | Journals, reversals, chart, mappings and periods | LIVE | Governed finance APIs and Control Center |
| R1-016 | Finance | Banking and reconciliation | PARTIAL | Canonical import/match/approval is live; bank-specific adapters are external work |
| R1-017 | Finance | Budgets, fixed assets and purchase-to-asset clearing | LIVE | Maker-checker and ledger-derived workflows implemented |
| R1-018 | Finance | Treasury, intercompany and consolidation | LIVE | Governed facilities and dual-company posting implemented |
| R1-019 | Finance | Financial statements | LIVE | Trial balance, GL, P&L, balance sheet, cash flow and comparisons derive from immutable journals |
| R1-020 | Reports | Governed CSV, PDF and XLSX exports | LIVE | Retained, checksum-bound artifacts implemented |
| R1-021 | Reports | Scheduled delivery | PLANNED | Required by screen catalog; production delivery workflow and provider integration incomplete |
| R1-022 | HR | Employee, attendance, leave, shifts, documents and loans | LIVE | Scoped governed workflows implemented |
| R1-023 | HR | Payroll preparation, posting, payslips and exports | PARTIAL | Executable payroll exists; statutory calculations/configuration and parallel-run approval remain external gates |
| R1-024 | Settings | Effective-dated policy and numbering | LIVE | Maker-checker configuration, secret references and atomic numbering implemented |
| R1-025 | Settings | Users, roles, sessions and production identity | PARTIAL | Scope authorization exists; production IdP and session lifecycle are not selected/integrated |
| R1-026 | Mobile | Online cash sales | LIVE | Live transport, server-authoritative rules and encrypted local state |
| R1-027 | Mobile | Online credit sales | PARTIAL | Server contract accepts governed online credit; Flutter credit-entry/customer-account UX remains incomplete |
| R1-028 | Mobile | Controlled offline cash and synchronization | LIVE | Encrypted cache, leases, allocations, idempotency and reconciliation cases implemented |
| R1-029 | Mobile | Receipt printing and governed reprint | PARTIAL | Truthful internal/fiscal state exists; production printer models and integration remain unselected |
| R1-030 | Mobile | Return/cancellation request | PARTIAL | Server corrections exist; attendant request flow requires final Release 1 confirmation/implementation |
| R1-031 | Integrations | TRA EFD/VFD | EXTERNAL | Official portal identified; specification/certification access and connector are not complete |
| R1-032 | Integrations | Mobile money and payment channels | EXTERNAL | Provider selection, contracts, callbacks and settlement reconciliation are not complete |
| R1-033 | Integrations | Bank-specific imports/exports | EXTERNAL | Canonical banking engine exists; approved banks and formats are not confirmed |
| R1-034 | Integrations | Email and WhatsApp | EXTERNAL | Provider, templates, consent, purpose and delivery controls are not confirmed |
| R1-035 | Migration | Source migration and opening balances | EXTERNAL | Runbook exists; actual source files, owners and two trials are not complete |
| R1-036 | Platform | CI, containers, IaC and telemetry foundation | PARTIAL | Local/CI evidence passes; production hosting, promotion, secrets, backup and DR are not complete |
| R1-037 | Security | Threat controls and production assurance | PARTIAL | OIDC validation, RLS, permission tests and vulnerability scan exist; IdP, DPIA and penetration evidence remain |
| R1-038 | Rollout | Training, pilot and hypercare | EXTERNAL | Sequence is approved in principle; named sites, users, dates and evidence are not provided |

## Control Center truth-state register

| Route group | Status | Production action |
| --- | --- | --- |
| `/customers`, `/customers/[customerId]` | LIVE | Retain and complete master-data UX |
| `/sales`, `/sales/new`, `/sales/[recordId]`, `/sales/lifecycle` | LIVE | Apply frontend audit and end-to-end UAT |
| `/purchases` | LIVE | Split dense workbench as required by task testing |
| `/inventory` | LIVE | Complete responsive/accessibility testing |
| `/finance` | LIVE | Complete provider adapters and dense-workbench UX testing |
| `/human-resources` | LIVE | Add professionally validated statutory configuration evidence |
| `/reports` | LIVE | Add scheduled delivery |
| `/settings` | LIVE | Add production identity/provider configuration operations |
| `/devices`, `/reconciliation`, `/reconciliation/[caseId]` | LIVE | Complete operational runbooks and UAT |
| `/` | DEMONSTRATION | Replace with governed dashboard API before production |
| `/search` | DEMONSTRATION | Replace with authorized live search before production |
| `/[module]` | DEMONSTRATION | Remove generic mock production fallback; supplier route is affected |
| `/[module]/new` | DEMONSTRATION | Remove or replace with module-specific live create workflows |
| `/[module]/[recordId]` | DEMONSTRATION | Remove or replace with live detail workflows |

## Explicit Release 1 exclusions

The following remain post-launch unless a signed scope change is approved:

- governed read-only AI;
- manufacturing;
- fleet management;
- ecommerce;
- external customer and supplier portals;
- Kafka, Kubernetes, dedicated analytics stores and multi-region writes without measured operational need.

## Scope-control conclusion

Release 1 is now classified at repository level. Demonstration screens are explicitly identified and cannot earn readiness credit. The open scope boundary is no longer hidden, but production scope cannot be frozen until named business owners approve the capability register and decide the remaining customer/supplier master, return-request, costing and provider details.
