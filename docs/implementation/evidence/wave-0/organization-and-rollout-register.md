# Wave 0 organization and rollout register

- Baseline date: 2026-08-06
- Status: Release 1 model confirmed; production instances unconfirmed

## Organization model

The implemented ownership hierarchy is:

```text
tenant/group -> legal company -> branch -> warehouse
```

Each legal company owns separate books and inventory. Branch and warehouse scope further restrict authorization and operations.

## Production organization facts

| ID | Fact | Current production value | Status | Required approval/evidence |
| --- | --- | --- | --- | --- |
| ORG-001 | First tenant/group | Itemba Group, per approved blueprint assumption | ASSUMED | Executive confirmation and legal naming |
| ORG-002 | Production legal companies | NOT PROVIDED | BLOCKED | Registered names, identifiers, TINs, base currency and books |
| ORG-003 | Production branches | NOT PROVIDED | BLOCKED | Names, addresses, legal-company ownership and rollout priority |
| ORG-004 | Production warehouses | NOT PROVIDED | BLOCKED | Names, branches, ownership, stock responsibility and physical-count owners |
| ORG-005 | Departments/cost centres | NOT PROVIDED | BLOCKED | Approved hierarchy and effective dates |
| ORG-006 | Base currency | TZS for Release 1 | ASSUMED | Finance-controller approval per legal company |
| ORG-007 | Business timezone | Africa/Dar_es_Salaam candidate | ASSUMED | Finance/payroll approval and boundary test cases |
| ORG-008 | Fiscal year and periods | NOT PROVIDED | BLOCKED | Approved calendar, opening periods and close policy |
| ORG-009 | Migration cutoff/opening date | NOT PROVIDED | BLOCKED | Signed cutoff plan and rollback window |
| ORG-010 | Pilot legal company | NOT PROVIDED | BLOCKED | Executive/operations selection |
| ORG-011 | Pilot branch | NOT PROVIDED | BLOCKED | Operations selection and readiness assessment |
| ORG-012 | Pilot warehouses/devices | NOT PROVIDED | BLOCKED | Inventory/device list and provisioning plan |
| ORG-013 | Additional branch order | NOT PROVIDED | BLOCKED | Dependency and capacity-based rollout sequence |
| ORG-014 | Additional legal-company order | NOT PROVIDED | BLOCKED | Opening-balance and support-capacity sequence |

## Development fixture boundary

The following values are deterministic development fixtures only and must never be promoted into production configuration:

| Fixture | Development value |
| --- | --- |
| Tenant | Itemba Group DEV |
| Legal company | Itemba Trading DEV |
| Branch | Dar es Salaam DEV |
| Warehouse | Main Warehouse DEV |
| Tax | `DEV_ZERO`, zero-rated test rule |
| Fiscal period | 2020-01-01 through 2100-01-01, open |
| User | `operator@itemba.invalid` |
| Products/customers/supplier | Fictional DEV masters |

The seed command already refuses non-development/test environments. Production deployment must additionally prove that development seed execution and header-based identity are impossible.

## Controlled rollout sequence

1. Configuration environment.
2. Test legal company with non-production data.
3. Selected full-suite pilot branch.
4. Additional branches only after the pilot stabilization gate.
5. Additional legal companies only after signed company-specific openings and support approval.

## Expansion gate

Expansion pauses for any critical defect, failed mandatory integration, unexplained stock/cash/payroll/tax/subledger/GL variance, security incident, failed recovery objective or user inability to complete a critical workflow without implementation-team help.
