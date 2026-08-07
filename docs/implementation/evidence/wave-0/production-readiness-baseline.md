# Wave 0 production-readiness baseline

- Baseline: v1
- Evidence date: 2026-08-06
- Source commit: `a42e1fb5937454a1a0b7d12764d9319028747aad`
- Overall result: **46.5 / 100 — NOT AUTHORIZED FOR PRODUCTION**
- Core ERP functional coverage tracked separately: **94.1%**

## Method

The score follows the ten weighted domains in `roadmap-to-production.md`. Points are earned only for repository or external evidence available on the baseline date. Missing named approval, provider, production environment, real source data or professional validation is scored as incomplete even when supporting architecture exists.

This is a readiness baseline, not a forecast and not a measure of engineering effort already invested.

## Weighted baseline

| Domain | Weight | Earned | Status | Evidence supporting the score | Evidence still required |
| --- | ---: | ---: | --- | --- | --- |
| Product, process, accounting, payroll and permission governance | 5.0 | 3.0 | AMBER | Product specification, process maps, screen catalog, permission matrix, integration/compliance registers | Named owners, signed policies, thresholds, terminology and executable UAT approval |
| Core ERP transactions and accounting integrity | 20.0 | 18.0 | AMBER | 104 implemented OpenAPI operations, 28 migrations, passing Go tests/vet/vulnerability scan, broad immutable posting workflows | Full-suite concurrency/failure matrix, final policy sign-off and closure of remaining Release 1 flows |
| Control Center and bilingual experience | 8.0 | 5.5 | AMBER | Production build passes; 47 tests pass; 15 of 20 page routes use live boundaries | Governed dashboard/search, removal of generic mock routes, accessibility, responsive and professional bilingual sign-off |
| Android POS, devices and offline operation | 7.0 | 5.0 | AMBER | 59 tests, encrypted SQLCipher, Keystore credentials, offline leases/allocations/sync, APK build | Online credit UX, printer integration, signed release pipeline, device matrix and operational acceptance |
| Identity, security and privacy | 10.0 | 4.5 | RED | OIDC/JWT verification code, contextual permissions, RLS, threat model, vulnerability checks | Selected IdP, MFA/session lifecycle, production secrets, DPIA, PDPC evidence, penetration test and access review |
| Fiscal, payment, banking, payroll and notification integrations | 15.0 | 1.5 | RED | Canonical boundaries, outbox, integration register, official TRA/PDPC/BOT references identified | Selected providers, credentials, connectors, certification, retry/replay, reconciliation and professional approval |
| Data migration and opening reconciliation | 10.0 | 0.5 | RED | Controlled migration runbook and required reconciliation definitions | Actual source register, data owners, mappings, cleansing, two trial migrations, physical stock and signed openings |
| Infrastructure, observability, backup and recovery | 10.0 | 3.0 | RED | Docker topology, Terraform foundation, OpenTelemetry config and CI validation pass | Selected host/region, production IaC, secret manager, promotion, SLOs, alerts, PITR, restore and DR evidence |
| Quality, performance, compliance and release assurance | 10.0 | 5.0 | AMBER | Local full-stack verification and latest GitHub CI pass | E2E UAT, load/soak, penetration, recovery, integration certification, payroll parallel runs and professional sign-off |
| Rollout, training, support and hypercare | 5.0 | 0.5 | RED | Rollout sequence and release blockers documented | Named pilot, trained users, support model, rehearsals, daily reconciliations and pilot sign-off |
| **Total** | **100.0** | **46.5** | **RED** |  |  |

## Mandatory blocker register

| ID | Blocker | Owner role | Status | Closure evidence |
| --- | --- | --- | --- | --- |
| BLK-001 | Production legal companies, branches, warehouses and opening date are unconfirmed | Executive sponsor / finance controller | OPEN | Signed organization and rollout register |
| BLK-002 | Named accountable owners and signatories are not assigned | Executive sponsor | OPEN | Ownership register with names and acceptance |
| BLK-003 | Production identity and hosting providers are not selected | Architecture / security / platform | OPEN | Approved decision records and contracts |
| BLK-004 | TRA specification/certification access and tax owner are not provided | Tax lead | OPEN | Sandbox credentials, test plan and professional appointment |
| BLK-005 | Banks, payment/mobile-money channels and formats are not confirmed | Treasury lead | OPEN | Approved provider list, account scopes and specifications |
| BLK-006 | Payroll statutory formulas and professional validator are not approved | Payroll lead | OPEN | Effective-dated policy sources and signed test cases |
| BLK-007 | Actual migration sources and data owners are not provided | Migration lead | OPEN | Completed source-data register and immutable source hashes |
| BLK-008 | Production privacy role, registration and cross-border position are unconfirmed | Privacy/DPO/legal lead | OPEN | PDPC/DPIA/data-flow evidence and approvals |
| BLK-009 | Production backups, restore, PITR and DR have not been exercised | Platform/SRE lead | OPEN | Approved RTO/RPO and successful exercises |
| BLK-010 | Pilot branch, users, training and support model are not selected | Implementation lead | OPEN | Approved pilot charter and readiness evidence |

## Confidence and change control

- Confidence: medium for repository facts; low for business, provider, regulatory and migration facts because no authoritative Itemba production evidence was supplied.
- The score must not increase from a statement of intent alone.
- Evidence added after this date requires a reviewer and a baseline revision.
- Any critical/high defect or failed mandatory reconciliation blocks production regardless of arithmetic.

## Next score target

The first target is **60% with no unknown owner roles**. It requires closure of provider/hosting/identity decisions, real source-data inventory, named signatories, approved Release 1 scope, and a production-like environment plan. Wave 1 feature work should not be used to mask these Wave 0 governance blockers.
