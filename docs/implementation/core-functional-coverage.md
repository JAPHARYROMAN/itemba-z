# Core ERP functional coverage

Updated: 2026-08-05

ITEMBA-Z core ERP functional coverage is **90.7%**. This is a weighted implementation score for executable Release 1 business surfaces, not a production-readiness score and not permission to launch.

| Core surface | Weight | Implemented | Weighted result |
| --- | ---: | ---: | ---: |
| Platform, security, audit, organization and governed settings | 10% | 92% | 9.2% |
| Customers, suppliers, sales, purchasing and subledgers | 24% | 93% | 22.3% |
| Inventory ownership, movement, transfer, count and valuation controls | 12% | 90% | 10.8% |
| Finance, banking, assets, treasury, intercompany and consolidation | 24% | 94% | 22.6% |
| Employees, attendance, leave, loans, payroll and payslips | 14% | 86% | 12.0% |
| Financial reporting, drill-down and governed export | 8% | 85% | 6.8% |
| Android POS, device governance, offline sync and reconciliation | 8% | 87% | 7.0% |
| **Total** | **100%** |  | **90.7%** |

Implemented means the workflow has an authoritative backend boundary, scope and permission enforcement, persistence, business validation, idempotency where commands mutate state, and integration tests proportional to its accounting risk. Demonstration-only screens receive no credit.

The remaining 9.3% is chiefly RFQ comparison, richer product/supplier maintenance, reorder/expiry and alternative costing, shift scheduling and employee documents, statutory payroll/payment files, comparative and scheduled report packs, and completion of the online mobile credit-entry UX. External TRA, payment, notification, identity-provider, migration, deployment, backup, security, recovery, and professional sign-off gates are tracked separately as production readiness.
