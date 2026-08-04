# Release 1 screen catalog

Each screen inherits working context, authorization, global search, approvals, notifications, bilingual labels, saved filters, export controls, and an audit-aware record drawer. Detail pages must link derived values to their source documents and ledger effects.

| Area | Screen | Primary action | Completion evidence |
| --- | --- | --- | --- |
| Shell | Dashboard | Investigate an alert or metric | Every value drills to governed records |
| Shell | Global search | Open an authorized result | Scope filtering and correlation tested |
| Shell | Approval inbox | Approve, reject, or return | Authority, separation of duties, reason, and audit recorded |
| Customers | Customer list and profile | Create customer account | Duplicate check, company terms, credit profile, and documents validated |
| Customers | Customer statement and ageing | Receive payment or investigate balance | Subledger reconciles to control account |
| Suppliers | Supplier list and profile | Create supplier account | Terms, tax, bank-detail protection, and duplicate check validated |
| Suppliers | Supplier statement and performance | Review liability or reliability | Subledger and governed metrics reconcile |
| Sales | Sales dashboard | Open exception or create sale | Metrics reconcile to posted sales |
| Sales | Quotations and orders | Submit order | Price, customer, stock reservation, and approval rules pass |
| Sales | Quick sale and invoice | Complete sale | Stock, receivable/payment, tax, journal, audit, receipt, and outbox commit once |
| Sales | Payments | Receive payment | Allocation and cash/bank/mobile-money movement reconcile |
| Sales | Returns and credit notes | Post approved correction | Original remains visible and reversal links reconcile |
| Purchases | Requests and approval queue | Submit or decide request | Threshold, budget, and separation-of-duties controls pass |
| Purchases | RFQ and comparison | Select supplier | Comparable terms and decision audit are retained |
| Purchases | Purchase orders | Approve order | Supplier, price, terms, and authorization pass |
| Purchases | Goods receipt | Receive goods | Warehouse, quantities, batches, ownership, and stock movement reconcile |
| Purchases | Supplier invoice and match | Post invoice | PO, receipt, invoice, tax, and tolerance checks pass |
| Purchases | Supplier payment and returns | Pay or correct | Payables, cash/bank, stock, and journal effects reconcile |
| Inventory | Product catalog | Create or revise product | Units, conversions, price/cost mapping, tax, and history validated |
| Inventory | Stock by location and movement ledger | Trace quantity | Balance derives from immutable movements |
| Inventory | Transfers | Dispatch or receive | In-transit ownership and both locations reconcile |
| Inventory | Counts and adjustments | Submit variance | Count evidence, reason, approval, movement, and journal reconcile |
| Inventory | Reorder, expiry, and valuation | Act on exception | Governed quantity and moving-average cost explain every value |
| Finance | Finance dashboard | Investigate exception | Metrics trace to ledger and reconciliations |
| Finance | Chart of accounts and posting mappings | Activate mapping | Effective date, approval, compatibility, and audit validated |
| Finance | Journals and general ledger | Post or reverse journal | Debits equal credits; closed-period policy enforced |
| Finance | Cash, bank, and mobile money | Reconcile account | Statement, system balance, matched items, and exceptions reconcile |
| Finance | AR and AP | Investigate exposure | Subledgers equal control accounts |
| Finance | Budgets, assets, loans, and facilities | Approve schedule | Ownership, effective dates, calculations, and journals validated |
| Finance | Period close and consolidation | Close period | Checklist, eliminations, retained earnings, and reopen authority tested |
| HR | Employee directory and profile | Create employment | Company, branch, position, sensitive data, and documents validated |
| HR | Attendance, shifts, and leave | Approve exception | Policy, balance, supervisor scope, and audit validated |
| HR | Loans and salary advances | Approve facility | Schedule, limits, payroll deduction, and ledger mapping validated |
| HR | Payroll preparation and review | Submit payroll | Effective-dated earnings/deductions and anomalies reviewed |
| HR | Payroll approval, posting, and payslips | Post payroll | Separation of duties, liabilities, payment schedule, and journals reconcile |
| Reports | Report library and builder | Run governed report | Metric definition, scope, filters, and drill-down remain consistent |
| Reports | Scheduled delivery and exports | Schedule or export | Permission, filters, actor, format, and delivery audit recorded |
| Settings | Organization and financial years | Configure structure | Tenant/company/branch/warehouse ownership remains valid |
| Settings | Users, roles, devices, and sessions | Grant or revoke access | Least privilege, scope, reauthentication, and audit pass |
| Settings | Sales, purchase, inventory, finance, and HR policy | Activate version | Effective date, approval, affected workflows, and rollback documented |
| Settings | Numbering, templates, notifications, imports, and integrations | Activate configuration | Uniqueness, secrets, retry, health, and data mapping tested |
| Mobile Sales | Sales home | Start a sale | Device, attendant, branch, warehouse, cache, and sync status visible |
| Mobile Sales | Customer selection | Select eligible customer | General Customer cash-only and credit eligibility enforced by UI and API |
| Mobile Sales | Product search and cart | Add product | Authorized price, unit, quantity, stock, and offline allocation checked |
| Mobile Sales | Payment or credit review | Complete sale | Cash settles fully; credit is online and server-authorized |
| Mobile Sales | Receipt and My Sales | Issue or reprint | Immutable server result and sync state visible |
| Mobile Sales | Return/cancellation request | Submit correction request | Original, reason, lines, quantities, and manager approval captured |
| Mobile Sales | Sync status and profile | Retry or request support | No duplicate posting; rejected items retain reason and local evidence |
