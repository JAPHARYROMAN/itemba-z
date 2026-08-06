# Core ERP functional coverage

Updated: 2026-08-05

ITEMBA-Z approved repository core ERP functional coverage is **100%** after
Wave 1 scope reconciliation. This is an implementation score for executable
Release 1 business surfaces, not a production-readiness score and not
permission to launch.

| Core surface | Weight | Implemented | Weighted result |
| --- | ---: | ---: | ---: |
| Platform, security, audit, organization and governed settings | 10% | 100% | 10.0% |
| Customers, suppliers, sales, purchasing and subledgers | 24% | 100% | 24.0% |
| Inventory ownership, movement, transfer, count and approved valuation controls | 12% | 100% | 12.0% |
| Finance, banking, assets, treasury, intercompany and consolidation | 24% | 100% | 24.0% |
| Employees, attendance, leave, loans, payroll and payslips | 14% | 100% | 14.0% |
| Financial reporting, drill-down and governed retained export | 8% | 100% | 8.0% |
| Android POS, device governance, offline sync and reconciliation | 8% | 100% | 8.0% |
| **Total** | **100%** |  | **100.0%** |

Implemented means the workflow has an authoritative backend boundary, scope and permission enforcement, persistence, business validation, idempotency where commands mutate state, and integration tests proportional to its accounting risk. Demonstration-only screens receive no credit.

The scope reconciliation moves provider-backed outbound report delivery and
receipt printing, plus professionally approved statutory payroll outputs, to
Wave 3. Costing beyond governed standard and moving average is not admitted
without an approved Itemba accounting policy. Online mobile credit entry,
signed Android delivery, live suppliers/search/notifications and immutable
audit retrieval are executable Wave 1 evidence.

External TRA, payment, notification, identity-provider, migration, deployment,
backup, security, recovery, accessibility/device UAT and professional sign-off
gates remain tracked separately as production readiness.
