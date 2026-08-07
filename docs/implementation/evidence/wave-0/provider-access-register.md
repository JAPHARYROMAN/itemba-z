# Wave 0 provider and access register

- Baseline date: 2026-08-06
- Rule: no connector build commitment without provider owner, approved contract/specification, sandbox access and failure/reconciliation procedure

## Register

| ID | Capability | Candidate/authority | Criticality | Business owner | Contract/specification | Sandbox/credentials | Status | Required next evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| PRV-001 | TRA EFD/VFD fiscal receipts | [TRA EFD Receipt API](https://virtual.tra.go.tz/efdmsRctApi/) | MANDATORY | UNASSIGNED | Official portal identified; taxpayer-specific requirements unconfirmed | NOT PROVIDED | BLOCKED | Appoint tax owner; obtain current specification, certificate/key process, sandbox and certification route |
| PRV-002 | Personal-data registration and transfers | [PDPC registration](https://www.pdpc.go.tz/en/registration-data-controller-processor/) | MANDATORY | UNASSIGNED | Official registration guidance identified | NOT PROVIDED | BLOCKED | Appoint DPO/privacy lead; confirm controller/processor role, registration and transfer position |
| PRV-003 | Production OIDC identity | NOT SELECTED | MANDATORY | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Approve IdP, tenant, MFA, lifecycle, support, SLA and test credentials |
| PRV-004 | Production hosting/database/object storage/Redis | NOT SELECTED | MANDATORY | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Approve host, region, subprocessors, residency/transfer position, SLA and recovery architecture |
| PRV-005 | Primary operating bank adapter | NOT SELECTED | MANDATORY | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Name bank/accounts, statement/import format, payment/export format, test files and support contact |
| PRV-006 | Additional bank adapters | NOT SELECTED | CONDITIONAL | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Inventory every Release 1 account and decide adapter priority |
| PRV-007 | Mobile-money/payment provider | NOT SELECTED | MANDATORY IF ACCEPTED | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Select only an approved provider; obtain callback/signing, settlement, reversal and sandbox details |
| PRV-008 | Receipt printers | NOT SELECTED | MANDATORY FOR POS | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Approve device models, transport, paper, character set, driver/SDK and physical test units |
| PRV-009 | Email delivery | NOT SELECTED | MANDATORY IF EMAIL ENABLED | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Approve provider, sender domains, templates, data region, retention and sandbox |
| PRV-010 | WhatsApp delivery | NOT SELECTED | CONDITIONAL | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Confirm business purpose, consent, provider, templates and data-processing terms |
| PRV-011 | Payroll bank file | NOT SELECTED | MANDATORY FOR BANK PAYROLL | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Approve paying bank and exact file/API format with test account |
| PRV-012 | Statutory remittance/file channels | NOT CONFIRMED | MANDATORY AS APPLICABLE | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Qualified professional confirms applicable authorities, forms, channels and effective dates |
| PRV-013 | Production support/monitoring alert destination | NOT SELECTED | MANDATORY | UNASSIGNED | NOT PROVIDED | NOT PROVIDED | BLOCKED | Approve paging/ticket channel, responders, retention and escalation |

## Regulatory selection control

Bank and payment providers must be checked against current Bank of Tanzania licensing/consumer guidance before contracting. The [Bank of Tanzania](https://www.bot.go.tz/BankSupervision/Mandate) is recorded as the authoritative starting point; the project must retain the exact provider-list evidence reviewed on the approval date.

## Access-handling rule

Credentials, certificates, private keys, merchant identifiers, taxpayer secrets, production account numbers and personal contacts must never be committed here. The register stores only secret-manager references and approval metadata after a provider is selected.
