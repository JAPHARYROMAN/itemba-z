# Wave 0 production-readiness register

- Baseline date: 2026-08-06
- Review cadence: weekly until launch
- Rule: `UNASSIGNED` owner/reviewer or `UNSCHEDULED` due date prevents gate closure

## Weighted domains

| ID | Domain | Weight | Accountable role | Independent reviewer | Named appointments | Target date | Status | Current evidence |
| --- | --- | ---: | --- | --- | --- | --- | --- | --- |
| RD-001 | Product and domain governance | 5% | Product lead | Executive sponsor | UNASSIGNED | UNSCHEDULED | AMBER | Product specs, process maps, screen catalog, permission matrix |
| RD-002 | Core ERP and accounting integrity | 20% | Backend/domain lead | Finance controller | UNASSIGNED | UNSCHEDULED | 104 implemented operations, migrations and passing full backend checks |
| RD-003 | Control Center and bilingual UX | 8% | Web/product-design lead | Product and bilingual reviewers | UNASSIGNED | UNSCHEDULED | 15 live routes, 47 tests, production build and Figma audit |
| RD-004 | Android POS and offline operation | 7% | Mobile lead | Sales/operations lead | UNASSIGNED | UNSCHEDULED | 59 tests, encrypted offline state, sync controls and debug APK |
| RD-005 | Identity, security and privacy | 10% | Security/privacy leads | Independent security assessor | UNASSIGNED | UNSCHEDULED | Threat model, OIDC verifier, RLS, audit and vulnerability scan |
| RD-006 | External/statutory integrations | 15% | Integration lead | Tax/treasury/payroll owners | UNASSIGNED | UNSCHEDULED | Integration register and official authority references only |
| RD-007 | Migration and opening balances | 10% | Migration lead | Finance/inventory/HR owners | UNASSIGNED | UNSCHEDULED | Migration runbook and source category register only |
| RD-008 | Infrastructure and recovery | 10% | Platform/SRE lead | Security/operations | UNASSIGNED | UNSCHEDULED | CI, Docker, Terraform and telemetry foundations |
| RD-009 | Quality/compliance assurance | 10% | QA/release lead | Executive release board | UNASSIGNED | UNSCHEDULED | Full local checks and latest CI pass; external qualification open |
| RD-010 | Rollout, training and support | 5% | Implementation lead | Operations/executive | UNASSIGNED | UNSCHEDULED | Rollout sequence only |

## Formal release gates

| ID | Gate | Accountable role | Required reviewers | Named appointments | Target date | Status | Closure evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| GATE-001 | Product and domain readiness | Product lead | Finance, tax, payroll, operations, executive | UNASSIGNED | UNSCHEDULED | OPEN | Signed scope, policies, terminology, thresholds, UAT and provider prerequisites |
| GATE-002 | Technical readiness | Architecture/platform lead | Security, QA, operations | UNASSIGNED | UNSCHEDULED | OPEN | Full invariant matrix, SLOs, correlation, load, restore, PITR and DR |
| GATE-003 | Business and compliance readiness | Executive sponsor | Finance, tax, payroll, privacy, migration | UNASSIGNED | UNSCHEDULED | OPEN | Two migrations, parallel payroll, fiscal certification, privacy and openings |
| GATE-004 | Pilot and rollout readiness | Implementation lead | Operations, support, finance, executive | UNASSIGNED | UNSCHEDULED | OPEN | Trained users, pilot operation, daily reconciliations and handover |

## Evidence update protocol

Every update records the evidence URI/path, artifact checksum where applicable, producing environment, completion date, accountable owner, independent reviewer, defects/exceptions and expiry/revalidation date. Status may move to closed only after the required reviewer accepts the evidence.
