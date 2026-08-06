# Permission and segregation-of-duties baseline

Permissions combine action, module, tenant, legal-company, branch, warehouse, ownership, amount, document state, and approval level. Backend authorization is authoritative.

| Role | Core permissions | Explicit restrictions |
| --- | --- | --- |
| Super Administrator | Group configuration and authorized operations across all companies | Cannot erase audit or silently alter posted records; break-glass actions require fresh auth and reason |
| Company Administrator | Assigned company operations, configuration, and reports | No access to another legal company without explicit group scope |
| Branch Manager | Assigned branch operations and approvals | Cannot approve own high-value transaction |
| Sales Attendant | Mobile sale create, own sale view, receipt reprint, return request | No customer/product creation, cost visibility, price change, stock adjustment, override approval, or sale deletion |
| Storekeeper | Receipt, issue, transfer, count, and adjustment creation | Cannot approve own material variance |
| Purchasing Officer | Requests, quotations, orders, and supplier coordination | Cannot approve above configured authority |
| Accountant | Journals, AR/AP, cash, bank, reconciliation, and finance reports | Cannot reopen periods or approve own restricted payment without authority |
| Cashier | Collections, payments, register, and cash close | No payroll or unrestricted bank access |
| HR Officer | Employee, attendance, leave, and payroll preparation | No general banking data; cannot approve own payroll |
| Finance Manager | Finance approvals, periods, closing, and statements | Reopening and reversal actions require reason and audit |
| Auditor | Scoped read-only data and audit reports | No mutations or approvals |

`audit.read` permits immutable timeline retrieval, including the retained
evidence payload, only in the actor's verified tenant and legal-company scope.
It does not authorize the underlying business-record endpoint, mutations or
approvals. Because evidence may contain sensitive financial detail, this grant
is restricted to approved audit/compliance roles and is tested independently
from ordinary module-read permissions.

The seed matrix is a starting policy. Named users, value thresholds, substitutes, delegations, and emergency access require signed business-owner approval before production.

Security governance separates `security.access.read`,
`security.access.manage`, `security.access.review`, and
`security.emergency.activate`. An assignment authorizes only while its user is
active, its exact organizational scope matches, its validity window is open,
and it has not been revoked. Governed grants require different maker and
approver identities, reason and ticket evidence. Delegation expires within 30
days and break-glass access within two hours; neither can erase immutable
evidence or bypass business maker-checker rules.

Executive overview access uses `dashboard.read` in addition to
`reports.financial.read`. The combination permits legal-company ledger metrics
and an exact branch/warehouse count of submitted transactional operation and
finance documents. It grants no mutation, approval, payroll detail, audit-log
access, or cross-scope query authority; optional company and branch query
assertions must match the verified identity scope.

Offline-sale exception access is split into `mobile.reconciliation.read` and
`mobile.reconciliation.resolve`. Production role assignment must preserve this
separation where policy requires investigation and disposition by different
people; neither permission grants sale posting or ledger mutation authority.

Device operations are separately split into `mobile.devices.read` and
`mobile.devices.manage`. Management permits reasoned suspension/reactivation
and allocation commands only inside the operator's exact assigned scope. It
does not permit device enrollment, mobile sale synchronization, stock-ledger
adjustment, or mutation of append-only change evidence.

Customer receivables are separately split into `customers.accounts.read` and
`customers.credit.manage`. Account read exposes scoped invoice ageing and the
effective policy. Credit management only appends a reasoned, future-effective
policy; it cannot edit history, alter ledger balances, enable General Customer
credit, or bypass overdue, risk-hold, and reconciliation controls.

Advanced finance separates budget read/manage/approve and fixed-asset
read/manage/approve/depreciate/dispose permissions. Budget and capitalization
approval reject the originating maker. Depreciation and disposal remain
separate ledger-posting capabilities and must be assigned only to finance roles
authorized for the relevant legal company and operating scope.

Treasury separates `finance.treasury.read`, `manage`, `approve`, and
`transact`. Facility activation rejects the originating maker; closure requires
fully reconciled principal and accrued interest. Transaction authority posts
only the four governed borrowing movements and does not permit facility-master
mutation, approval, fiscal-period override, or account remapping.

Group finance separates `finance.intercompany.read`, `manage`, `approve`, and
`finance.consolidation.read`. Source approval and counterparty confirmation are
bound to the actor's assigned legal-company scope. The maker cannot approve,
the source approver cannot confirm the counterparty posting, and consolidation
access does not grant transaction mutation.

External integration operations separate `integrations.read`,
`integrations.manage`, and `integrations.replay`. Read access exposes scoped,
redacted delivery and attempt evidence. Management governs new immutable route
versions but cannot approve the maker's route. Replay is reserved for reviewed
dead letters and cannot edit a prior request, response or attempt; it also does
not grant permission to post or reverse the underlying ERP transaction.
