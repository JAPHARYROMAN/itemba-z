# Wave 0 decision log

- Baseline date: 2026-08-06

## Recorded decisions

| ID | Decision | Status | Rationale/evidence |
| --- | --- | --- | --- |
| W0-D001 | Functional coverage and production readiness remain separate metrics | ACCEPTED | 94.1% core coverage does not include provider, migration, recovery, professional and pilot evidence |
| W0-D002 | The first evidence-based production-readiness baseline is 46.5/100 | ACCEPTED | Weighted evidence in `production-readiness-baseline.md` |
| W0-D003 | Mock-backed Control Center routes are classified as demonstration and earn no readiness credit | ACCEPTED | Machine inventory identifies 15 live and 5 demonstration page routes |
| W0-D004 | Development organization/tax/fiscal fixtures cannot become production configuration | ACCEPTED | `devseed` values are fictional and environment-restricted |
| W0-D005 | Release 1 retains the Go modular monolith, transactional outbox, REST/OpenAPI, Flutter and managed-container path | ACCEPTED | ADR-0001 preserves atomic ERP posting and NEXT Sections 3, 5 and 6 qualities |
| W0-D006 | Provider-specific connectors are not implemented until owner, specification, sandbox and reconciliation evidence exist | ACCEPTED | Integration register and external failure semantics |
| W0-D007 | Production personal, payroll, bank and commercial source files are stored outside the source repository | ACCEPTED | Data classification and migration controls |
| W0-D008 | No named business/professional owner or provider is inferred from repository ownership | ACCEPTED | Authority and professional qualification require explicit appointment |
| W0-D009 | Kafka, Kubernetes and service extraction remain post-need decisions | ACCEPTED | ADR-0001 and roadmap architecture alignment |

## Open decisions requiring authorized input

| ID | Decision required | Decision owner | Current status | Blocks |
| --- | --- | --- | --- | --- |
| W0-O001 | Name executive sponsor and all domain/professional signatories | Executive sponsor | UNASSIGNED | Scope freeze and Wave 0 exit |
| W0-O002 | Confirm production legal companies, branches, warehouses, fiscal calendar and pilot | Executive/finance/operations | UNASSIGNED | Configuration, migration and rollout |
| W0-O003 | Select production identity provider | Security/platform | UNASSIGNED | Authentication and penetration testing |
| W0-O004 | Select hosting provider, region and managed services | Architecture/privacy/platform | UNASSIGNED | Production IaC, privacy assessment and DR |
| W0-O005 | Confirm TRA route, taxpayer scope and tax professional | Tax lead | UNASSIGNED | Fiscal connector and certification |
| W0-O006 | Confirm banks, accounts, payment/mobile-money providers and payroll bank | Treasury/payroll | UNASSIGNED | Adapters, settlement and payment files |
| W0-O007 | Confirm email, WhatsApp and printer providers/models | Product/operations/privacy | UNASSIGNED | Notifications and POS receipts |
| W0-O008 | Approve statutory payroll applicability, sources and validator | Payroll/finance | UNASSIGNED | Payroll qualification |
| W0-O009 | Provide controlled production-data source inventory | Migration lead/domain owners | UNASSIGNED | Trial Migration 1 |
| W0-O010 | Approve remaining Release 1 boundary decisions | Product/executive | UNASSIGNED | Scope freeze and dated baseline |
