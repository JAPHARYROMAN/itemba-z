# Release 1 gates

## Product and domain readiness

- Screen specifications, business processes, accounting and inventory policies, permission matrix, approval thresholds, governed metrics, bilingual terminology, and UAT cases are signed.
- Required TRA, bank, mobile-money, printer, messaging, payroll, and privacy dependencies have owners, specifications, sandbox credentials, and failure procedures.

## Technical readiness

- All module definition-of-done checks pass: rules, permissions, audit, ledger effects, reports, failure handling, tests, migration, documentation, recovery, and security.
- Golden invariants pass under normal, concurrent, retried, reversed, and dependency-failure conditions.
- Common pages meet the two-second target under the agreed load; critical API operations meet the service-level budget.
- Observability correlates request, actor, source document, transaction, journal, stock movement, audit event, and outbox event without logging restricted payloads.
- A production-like backup restores within the approved RTO and meets the approved RPO.

## Business and compliance readiness

- Two trial migrations and the final opening reconciliation are signed.
- Payroll completes approved parallel runs.
- Fiscal receipts and external reconciliation complete in an approved sandbox and certification path.
- Privacy registration, impact assessment, DPO responsibilities, retention, processor agreements, and cross-border transfer approval are complete where applicable.
- Finance, tax, payroll, operations, security, and executive owners sign go-live.

## Rollout

Use configuration environment, test company, full-suite pilot branch, then branch-by-branch and legal-company-by-legal-company expansion. Hypercare reconciles sales, stock, cash, receivables, payables, payroll, tax, and the general ledger daily. Critical or unexplained variance pauses expansion.
