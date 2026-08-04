# System context

## Runtime surfaces

| Surface | Users | Responsibility |
| --- | --- | --- |
| Control Center | Administrators and authorized business staff | Complete ERP operations, approvals, configuration, audit, and reporting |
| Sales Mobile | Sales attendants | Controlled cash and credit sales, receipts, own sales, return requests, and synchronization |
| Core API | Both clients and approved integrations | Authentication context, business rules, atomic posting, queries, and idempotency |
| Worker | Internal | Outbox delivery, report projections, documents, notifications, imports, and connector retries |

## Constitutional layer mapping

| NEXT layer | ITEMBA-Z implementation |
| --- | --- |
| Experience | Next.js Control Center and Flutter POS |
| Application | Go bounded contexts and background worker |
| Intelligence | Governed semantic read APIs and future AI gateway |
| Social and economic | Identity, approvals, payments, company ledgers, and payroll |
| Data and event | PostgreSQL ledgers, transactional outbox, and reporting projections |
| Infrastructure | Managed containers, Terraform, CI/CD, OpenTelemetry, Redis, and object storage |
| Edge | CDN for static assets and later regional delivery where measured need justifies it |

## Bounded contexts

Organization, Identity and Access, Parties, Catalog and Pricing, Sales, Purchasing, Inventory, Finance, Human Resources and Payroll, Approvals, Audit, Documents, Reporting, Notifications, Imports, and Integrations are separate logical modules. Transaction coordinators compose them through typed interfaces without transferring ownership of their data.
