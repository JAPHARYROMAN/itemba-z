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

The seed matrix is a starting policy. Named users, value thresholds, substitutes, delegations, and emergency access require signed business-owner approval before production.

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
