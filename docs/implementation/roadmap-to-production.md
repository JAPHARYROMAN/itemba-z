# ITEMBA-Z roadmap to 100% production readiness

Updated: 2026-08-06

## Purpose

This roadmap converts the Release 1 blueprint and the current executable system into an evidence-based path to production. It distinguishes functional coverage from production readiness. A workflow is not production-ready merely because its screen, API, database tables, and happy-path tests exist.

ITEMBA-Z may claim 100% production readiness only when every mandatory release gate has objective evidence, a named approver, and no unresolved critical or high-severity defect.

## Starting position

The repository now reports 100% of the Wave 1 approved repository core ERP functional scope after provider/statutory work was assigned to Wave 3. The Go modular monolith, PostgreSQL transaction boundary, Control Center, Android POS foundations, immutable ledgers, governed finance, commercial, inventory, payroll, reporting, offline synchronization, audit, and outbox capabilities are substantial.

The remaining path is not 5.9% of the total launch effort. Production identity, statutory calculations, TRA and provider integrations, bank adapters, deployment controls, secrets, recovery evidence, source-data migration, professional validation, pilot operation, and signed reconciliations remain outside the core functional-coverage score.

The initial production-readiness baseline must be established in Wave 0 from evidence. Until then, the defensible status is:

- approved repository core ERP functional coverage: 100%;
- implemented OpenAPI operations: 111;
- planned OpenAPI operations: 0; future work must add reviewed contracts before implementation;
- production release gates: open;
- production launch authorization: not granted.

## What 100% means

The production-readiness score has ten domains. A domain earns points only for completed, reviewed evidence. Partial code without its required verification does not receive full credit.

| Domain | Weight | Completion evidence |
| --- | ---: | --- |
| Product, process, accounting, payroll and permission governance | 5% | Signed specifications, process maps, policies, terminology, approval thresholds and UAT cases |
| Core ERP transactions and accounting integrity | 20% | Complete executable workflows, balanced journals, reconciled subledgers, reversals, idempotency and concurrency evidence |
| Control Center and bilingual experience | 8% | All Release 1 screens live, accessible, responsive, permission-shaped and free of misleading demonstration data |
| Android POS, devices and offline operation | 7% | Online cash and credit, controlled offline cash, encrypted storage, printing, device governance and duplicate-safe synchronization |
| Identity, security and privacy | 10% | Production identity, least privilege, secure sessions, secret custody, threat controls, privacy obligations and penetration-test closure |
| Fiscal, payment, banking, payroll and notification integrations | 15% | Approved provider contracts, sandbox/certification evidence, reconciliation, retry, outage and support procedures |
| Data migration and opening reconciliation | 10% | Source register, cleansing evidence, two trial migrations, physical stock verification and signed opening balances |
| Infrastructure, observability, backup and recovery | 10% | Production IaC, promotion controls, SLOs, alerts, PITR, restore and disaster-recovery evidence |
| Quality, performance, compliance and release assurance | 10% | Full test matrix, load and recovery results, professional sign-offs and zero critical/high defects |
| Rollout, training, support and hypercare | 5% | Trained users, support readiness, pilot success, daily reconciliations and approved expansion |
| **Total** | **100%** | **Every mandatory gate closed** |

### Scoring rules

1. Every score entry must link to a durable artifact such as a test report, reconciliation, approval, provider response, restore log, signed checklist or release record.
2. A mandatory blocker caps the total score below 100% even if weighted arithmetic would otherwise round upward.
3. A critical or high-severity defect in security, accounting, stock, payroll, fiscalization, migration or recovery sets the affected domain to blocked.
4. Demonstration data and planned contracts receive no production-readiness credit.
5. Sign-off belongs to the accountable business or professional owner; engineering cannot self-approve tax, payroll, legal, finance or security obligations.
6. The score is recalculated at the weekly release-readiness review and stored with the evidence snapshot used to calculate it.

## Delivery strategy

The work is organized into seven overlapping waves. With the assumed multi-workstream team and timely provider access, the planning range is approximately 20–28 weeks. This is a capacity range, not a committed deadline. Wave 0 must establish staffing, integration access, data condition, and accountable owners before a dated baseline is approved.

## Wave 0 — Lock scope and establish the evidence baseline

Planning range: 1–2 weeks

Execution evidence: [`evidence/wave-0`](evidence/wave-0/README.md). Repository inventory, verification, scope classification and the first 46.5/100 baseline are complete. The Wave 0 exit gate remains blocked by authorized owner appointments, production organization facts, provider access/selections, real source-data inventory and business/professional approval.

### Deliverables

- Freeze Release 1 scope and classify every screen and API as live, planned, demonstration-only, or post-launch.
- Create the production-readiness register with one owner, reviewer, due date, status and evidence link for every weighted domain and release gate.
- Reconcile the screen catalog, OpenAPI status, backend routes, database migrations, Control Center routes and Flutter flows.
- Confirm the legal companies, branches, warehouses, currencies, business timezone, fiscal periods, opening date and rollout sequence.
- Name the finance, tax, payroll, legal/privacy, operations, security, migration, training, support and executive signatories.
- Confirm the selected production identity, hosting, TRA, bank, mobile-money, email, WhatsApp and printing providers.
- Obtain sandbox credentials, specifications, certificates, test accounts and escalation contacts.
- Inventory every spreadsheet, manual ledger, bank statement, payroll file, product list, customer/supplier list and opening-balance source.
- Convert the frontend Figma audit into a prioritized UX backlog and mark every non-live surface unmistakably.

### Exit gate

- No Release 1 requirement lacks an owner, acceptance test or evidence definition.
- The dated delivery baseline and provider/data dependencies are approved.
- The first evidence-based production-readiness score is published.

## Wave 1 — Close the remaining executable product gaps

Planning range: 3–5 weeks, parallel with Waves 2 and 3

Execution record: [`wave-1-execution.md`](wave-1-execution.md). Repository Wave
1 is complete. Provider-backed printing and outbound communications, statutory
payroll policy and certification remain Wave 3 gates; human UAT remains Wave 5.

### Core ERP and contracts

- Implement the governed executive dashboard endpoint and replace illustrative dashboard metrics with reconciled live projections.
- Complete scheduled report delivery with permission checks, immutable delivery history, retry, failure disclosure and audit correlation.
- Complete professionally specified inventory-costing requirements beyond the existing standard and moving-average evidence only where Itemba policy requires them.
- Remove generic demonstration workflows from production navigation or replace them with authoritative APIs.
- Complete cross-module acceptance tests for order-to-cash, procure-to-pay, inventory, record-to-report and hire-to-retire.
- Reconcile the core coverage document against executable evidence and close or explicitly defer every remaining item.

### Control Center

- Apply the frontend audit: fix clipped mobile controls, horizontal context overflow, undersized text, weak confirmation summaries and misleading truth states.
- Make the approval inbox, notification center and global search authoritative; provide a reusable immutable record-audit API and drawer. Persisted personal saved filters were removed from Release 1 during scope reconciliation and remain post-launch UX work.
- Complete English and Swahili coverage for content, validation, API errors, accessible names, documents and financial terminology.
- Verify keyboard navigation, focus management, contrast, zoom, screen-reader semantics and responsive layouts.
- Add explicit LIVE, PREVIEW and UNAVAILABLE states to every metric-bearing surface.

### Android POS

- Complete online credit-sale entry and customer-account cache UX while retaining server-authoritative credit decisions.
- Complete receipt-printer integration, reprint controls and visible internal-versus-fiscal receipt state.
- Complete return/cancellation request and support/escalation flows.
- Verify device suspension, credential rotation, cache expiry, lease expiry, clock skew, storage corruption, restart and synchronization recovery.
- Produce a signed release APK/AAB pipeline with versioning, provenance and controlled distribution.

### Exit gate

- Core ERP functional coverage is 100% for the approved Release 1 scope, or every deliberate deferral is removed from Release 1.
- No demonstration data can be mistaken for live production data.
- Web and POS critical journeys pass bilingual, accessibility and responsive acceptance tests.

## Wave 2 — Production identity, security and privacy controls

Planning range: 3–5 weeks, parallel with Waves 1 and 3

Execution record: [`wave-2-execution.md`](wave-2-execution.md). The
provider-neutral repository implementation is complete. The Wave 2 release
exit remains blocked by IdP/secret-provider selection, named owner approvals,
privacy/DPIA/transfer decisions, protected-run evidence and an independent
penetration-test closure letter.

### Deliverables

- Select and integrate the production OIDC provider, MFA policy, login, logout, token renewal, revocation, inactivity timeout and forced reauthentication.
- Implement joiner, mover, leaver, delegation, emergency access and periodic access-review procedures.
- Validate role, scope, amount, maker-checker and segregation-of-duties assignments with business owners.
- Provision production secret management, certificate/key custody, rotation, access logging and break-glass procedures.
- Add dependency, container, infrastructure, secret, SAST and DAST scanning with blocking severity policies and retained reports.
- Produce an SBOM and signed build provenance for API, worker, web and Android artifacts.
- Complete the data inventory, classification, lawful basis, notices, retention schedule, data-subject rights, incident response, processor register and deletion/legal-hold procedures.
- Complete the DPIA and cross-border hosting/support assessment where applicable.
- Run independent penetration testing and close all critical and high findings.

### Exit gate

- Production authentication and authorization pass cross-tenant, cross-company, payroll, finance, session and device tests.
- No production secret exists in code, CI logs, images or unmanaged files.
- Privacy, security and legal owners approve the control set.

## Wave 3 — External integrations and statutory configuration

Planning range: 6–12 weeks and the likely critical path

Execution record: [`wave-3-execution.md`](wave-3-execution.md). The repository-
controlled delivery spine, governed operations APIs, replay/reconciliation and
bilingual workspace are complete. Approved provider/statutory adapters,
protected sandbox evidence and professional/business sign-off remain external
blockers, so the Wave 3 release exit is not closed.

Each connector must satisfy the integration register: owner, versioned contract, authentication, idempotency, timeout/retry budget, circuit breaking, dead-letter handling, manual replay, reconciliation, monitoring, data protection and escalation.

### TRA EFD/VFD

- Confirm the applicable current specification and certification route with the authorized Tanzanian tax professional and TRA/provider contacts.
- Implement fiscalization as an asynchronous, durable integration that never corrupts the committed ERP transaction.
- Retain request, response, fiscal reference, receipt, status, retry and reconciliation evidence without exposing protected credentials.
- Implement cancellation/return, duplicate, timeout, outage, rejected receipt and recovery procedures.
- Complete approved sandbox/certification tests and professional sign-off.

### Payments and mobile money

- Confirm approved channels, merchant accounts, settlement rules, fees, reversals and webhook/file contracts.
- Implement signed/certified inbound callbacks, duplicate protection, settlement matching and exception queues.
- Reconcile provider transactions, ERP payments, fees, cash/bank movements and settlement statements.

### Banks

- Implement provider-specific statement/file adapters behind the governed canonical import boundary.
- Add format/version detection, signed source-file hashes, duplicate protection, line-level errors and reconciliation evidence.
- Validate payment and payroll export formats with each approved bank where used.

### Payroll and statutory outputs

- Configure effective-dated, professionally approved PAYE, NSSF, WCF and other applicable calculations without hardcoding statutory values.
- Retain calculation snapshots, source authority, effective date, approval and superseded version.
- Validate filing/remittance outputs and liability-account reconciliations.

### Communications and printing

- Integrate approved email and WhatsApp providers only for governed templates and consented purposes.
- Add delivery state, retry limits, opt-out/consent controls, redaction and support procedures.
- Validate supported receipt-printer models, paper formats, character rendering and offline behavior.

### Exit gate

- Every mandatory connector passes normal, duplicate, delayed, malformed, unavailable, rejected and recovery scenarios.
- Fiscal, settlement and payroll/statutory reconciliations are signed by their professional and business owners.

## Wave 4 — Production platform and operational resilience

Planning range: 4–8 weeks, parallel with Waves 2 and 3

Execution record: [`wave-4-execution.md`](wave-4-execution.md). The first
repository control-plane slice is implemented. Provider/region selection,
production infrastructure modules, protected environment configuration,
telemetry and paging binding, automated backup/PITR, production-like exercises
and Operations acceptance remain open, so the Wave 4 exit gate is not closed.

### Environments and delivery

- Provision isolated configuration, test, staging, pilot and production environments through reviewed infrastructure-as-code.
- Use managed PostgreSQL, managed TLS, private networking, encrypted object storage, Redis only for non-authoritative coordination, and least-privilege runtime identities.
- Add protected build, deployment, approval, promotion, rollback and database-migration workflows.
- Prohibit development seeds, header identity and unsafe debug modes outside local development.
- Exercise forward and backward-compatible deployment and rollback procedures.

### Observability and supportability

- Define SLOs and error budgets for critical sales, synchronization, posting, reporting and integration paths.
- Correlate request, actor, scope, device, source document, business transaction, journal, stock movement, audit and outbox identifiers.
- Add dashboards and alerts for availability, latency, error rate, database saturation, outbox lag, fiscal/payment failures, sync failures, reconciliation exceptions and backup status.
- Add redaction and restricted-data logging tests.
- Create runbooks for identity outage, database pressure, stuck migrations, outbox lag, connector outage, fiscal backlog, duplicate callbacks, device compromise and suspected data breach.

### Backup and disaster recovery

- Approve RTO and RPO per service and data class.
- Automate encrypted backups, point-in-time recovery, object-version retention and restore verification.
- Execute production-like restore, regional/service failure and disaster-recovery exercises.
- Prove that restored ledgers, stock, audit, outbox and documents reconcile at the selected recovery point.

### Exit gate

- A release candidate can be promoted and rolled back without unmanaged production access.
- Load, soak, failover, backup restore and disaster-recovery objectives pass with retained evidence.
- Operations accepts dashboards, alerts, runbooks and escalation paths.

## Wave 5 — Migration, UAT and formal release qualification

Planning range: 6–8 weeks; Trial 1 can begin once Wave 0 mappings are stable

### Migration rehearsals

- Build a repeatable, checksum-bound, batch-audited staging and import pipeline.
- Cleanse and approve organizations, users, parties, products, units, prices, tax mappings, accounts, employees and historical/open documents.
- Run Trial Migration 1 to expose mapping and data-quality failures.
- Correct source data and mappings; never repair posted destination balances manually.
- Run Trial Migration 2 using the complete production procedure and timed cutoff plan.
- Reconcile stock, AR, AP, cash, bank, mobile money, employee loans, leave, payroll and trial balance.

### Full qualification matrix

- End-to-end business workflow and maker-checker UAT.
- Balanced-journal, stock-conservation, subledger-control and reversal tests.
- Tenant, company, branch, warehouse, payroll and sensitive-finance isolation tests.
- Idempotency, duplicate sync, concurrency, ambiguous response and retry tests.
- Closed-period, effective-date, exchange/currency and timezone boundary tests.
- Integration outage, replay, dead-letter and reconciliation tests.
- Accessibility, bilingual, printing and Android-device-matrix tests.
- Load, soak, penetration, dependency, restore and disaster-recovery tests.
- Payroll parallel runs and statutory/accounting reconciliation.

### Exit gate

- Two trial migrations reconcile and are signed.
- Payroll parallel runs are approved.
- Every release gate has linked evidence.
- No critical or high defect remains; accepted lower-severity defects have owners and approved workarounds.
- Finance, tax, payroll, operations, migration, security and executive owners approve the pilot candidate.

## Wave 6 — Pilot, hypercare and controlled production rollout

Planning range: 4–6 weeks before wider rollout

### Pilot sequence

1. Configuration environment.
2. Test legal company.
3. Full-suite pilot branch using production processes and devices.
4. Additional branches after the pilot gate.
5. Additional legal companies after branch stability and company-opening reconciliation.

### Pilot controls

- Train role-based users in English and Swahili and assess proficiency on real workflows.
- Run a dress rehearsal for cutoff, stock count, opening balances, device provisioning and support escalation.
- Freeze legacy changes, complete signed physical stock verification, execute final migration and reconcile openings.
- Operate structured command-center support with severity, ownership, response and escalation targets.
- Reconcile sales, fiscal receipts, stock, cash, banks, mobile money, AR, AP, payroll, tax and general ledger daily.
- Pause expansion for any critical defect, unexplained variance, unrecoverable queue, security event or failed mandatory integration.

### Exit gate

- Pilot users complete critical workflows without implementation-team assistance.
- The pilot operates through the agreed stabilization period with no unexplained financial, stock, payroll or fiscal variance.
- Support, operations and business owners sign the production handover.
- Executive go-live approval is recorded before expansion.

## Cross-cutting workstreams and ownership

| Workstream | Accountable owner | Required partners |
| --- | --- | --- |
| Product scope and UAT | Product lead | Operations, finance, HR, sales, inventory |
| Accounting and posting | Finance controller | ERP domain, backend, reporting, audit |
| Tax and fiscalization | Tanzanian tax professional | TRA/provider, finance, integration engineering |
| Payroll and statutory | Payroll owner and Tanzanian payroll professional | HR, finance, backend, bank/provider |
| Web experience | Web lead | Product design, accessibility, QA, domain owners |
| Android POS | Mobile lead | Sales operations, device support, printer/provider |
| Core platform | Backend lead | Architecture, database, security, QA |
| Integrations | Integration lead | Provider owners, finance, operations, security |
| Data migration | Migration lead | Business data owners, finance, HR, inventory |
| Infrastructure and SRE | Platform lead | Security, database, support, engineering |
| Security and privacy | Security/privacy lead | Legal/DPO, platform, product, vendors |
| Release assurance | QA/release lead | All workstreams and executive sponsor |
| Training and rollout | Implementation lead | Support, branch champions, business owners |

## Critical path and dependency rules

1. Provider selection, specifications, sandbox credentials and professional policy decisions must precede final connector and statutory implementation.
2. Production identity and secret management must precede production-like security and penetration testing.
3. Stable master-data mappings and approved accounting policies must precede meaningful trial migration.
4. Provider adapters and statutory configuration must be stable before payroll parallel runs, fiscal certification and full end-to-end UAT.
5. Production-like infrastructure must exist before load, restore, disaster-recovery and penetration-test evidence can close.
6. Trial Migration 2, payroll parallel runs, integration certification and recovery evidence must close before pilot approval.
7. Pilot reconciliation and independent user operation must close before branch or legal-company expansion.

## Operating cadence

- Daily: workstream blocker review during active delivery and hypercare.
- Weekly: evidence-based readiness score, dependency review, defect ageing and decision log.
- Fortnightly: integrated demo using production-like data and failure scenarios, not isolated happy paths.
- Per release candidate: complete automated checks, contract drift check, migration rehearsal, security scan, release notes and rollback review.
- Monthly: architecture, privacy, threat-model, vendor and disaster-recovery risk review until launch.

## Mandatory launch blockers

Production launch remains prohibited while any of the following is true:

- a journal, subledger, stock, cash, bank, payroll, tax or opening balance does not reconcile;
- General Customer can purchase on credit or a protected price can be overridden without authority;
- a duplicate request or mobile synchronization can duplicate a business transaction;
- a reversal edits or hides the original transaction or does not restore its authorized effects;
- cross-tenant, cross-company, payroll or sensitive-finance access controls fail;
- mandatory fiscal, payment, bank, payroll or receipt workflows lack approved failure and reconciliation evidence;
- production authentication, secret custody, backup restore, PITR, load, penetration or audit-correlation tests have not passed;
- either trial migration or a payroll parallel run is unsigned;
- a mandatory Tanzanian professional, business, security or executive approval is absent;
- any critical or high-severity defect remains open;
- pilot users cannot complete critical workflows without implementation-team intervention.

## First execution backlog

The next implementation cycle should start these items in order:

1. Create the production-readiness evidence register and reconcile current claims against executable tests.
2. Select production identity, hosting, TRA, bank, payment, messaging and printer providers and obtain specifications and sandbox access.
3. Implement the live governed dashboard and remove or relabel demonstration surfaces.
4. Complete Flutter online credit sales and receipt-printer workflow.
5. Implement production identity/session lifecycle and secret management.
6. Build the TRA fiscalization connector framework and the first approved bank/payment adapters.
7. Provision production-like staging with promotion, observability, backup and restore automation.
8. Build the repeatable migration pipeline and execute Trial Migration 1.
9. Apply the frontend audit and complete bilingual/accessibility verification.
10. Run the integrated qualification matrix, Trial Migration 2, payroll parallel runs and pilot readiness review.

## Architecture alignment

This roadmap retains the NEXT Constitution requirements for modularity, events, observability, resilience, security, extensibility and complete engineering evidence in Sections 3, 5 and 6. ITEMBA-Z intentionally follows ADR-0001 for the first release: a Go modular monolith, transactional outbox, REST/OpenAPI, Flutter and managed containers preserve atomic ERP transactions while deferring Kafka, Kubernetes and independently deployed services until measured operational need justifies them.
