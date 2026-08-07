# Privacy and data governance control set

This is an engineering control record, not legal advice. The appointed
Tanzanian privacy/legal owner must confirm controller/processor roles, lawful
bases, notices, PDPC obligations, transfers and retention before production.

## Data inventory and classification

| Data set | Examples | Class | Subjects | Purpose | Proposed lawful basis (owner to approve) | System / transfer | Retention trigger |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Identity and access | name, email, IdP subject, scope, MFA/access events | Restricted | staff, contractors | secure system access and audit | contract / legitimate interest / legal duty | IdP → Control Center/API; hosting region pending | relationship end plus approved security period |
| HR and payroll | personnel, attendance, leave, loans, pay, bank/tax identifiers | Highly restricted | employees | employment, payroll and statutory duties | contract / legal obligation | ERP, bank/payroll exports; providers pending | statutory/employment schedule |
| Customers and credit | contacts, addresses, invoices, limits, ageing, collections | Restricted | customers/contacts | sales, delivery, credit and accounting | contract / legitimate interest / legal obligation | ERP and approved communications providers | transaction/legal schedule |
| Suppliers | contacts, bank/payment and tax details | Restricted | suppliers/contacts | procurement, payment and tax | contract / legal obligation | ERP and approved banks | transaction/legal schedule |
| Sales/POS and devices | sale lines, attendant/device IDs, location scope, sync evidence | Restricted | customers, staff | fulfilment, fraud control, accounting | contract / legitimate interest / legal obligation | Android device → API | transaction/security schedule |
| Finance/tax/audit | journals, bank lines, fiscal data, immutable evidence | Highly restricted | customers, suppliers, staff | accounting, tax, assurance | legal obligation | ERP, banks, TRA/provider pending | statutory record schedule/legal hold |
| Communications | destination, template, consent/opt-out and delivery state | Restricted | customers, staff, suppliers | operational notices | consent / contract / legitimate interest as approved | email/WhatsApp provider pending | purpose/consent expiry plus evidence period |
| Support/security telemetry | correlation, actor hash, device, error/security events | Restricted | users | reliability, incident response and fraud defense | legitimate interest / legal duty | approved monitoring provider pending | approved operational/security period |
| Documents and backups | attachments, exports, payslips, receipts, backup copies | inherits source class | all applicable | evidence, recovery and authorized service | same as source / legal obligation | object store/backup region pending | source schedule plus backup expiry |

Direct special/sensitive data not required by an approved purpose must not be
collected. Free-text fields are not a license to store passwords, health data
or identity-document images. Production data is prohibited in a hosting or
support region until the transfer assessment and safeguards are approved.

## Transparency and rights

Notices must identify the Itemba legal entity, purposes, lawful bases,
categories, recipients/processors, transfers, retention criteria, rights,
complaint route, DPO/privacy contact and automated decision use. English and
Swahili versions are versioned; acceptance/availability evidence retains notice
version and effective date without coercive consent bundling.

Data-subject requests (access, correction, objection/restriction, portability
where applicable and deletion) receive a case ID, verified requester identity,
scope, legal deadline, systems searched, exemptions, reviewer, response and
delivery evidence. Search uses authoritative source IDs and includes documents,
exports, support systems and processors. Corrections never rewrite posted
financial/audit history; a linked correction or explanatory record preserves
the legal record. Identity evidence collected for the request is minimized and
deleted on its own schedule.

## Retention, legal hold and disposal

The privacy/legal, finance, tax, payroll, HR and security owners approve exact
periods. Until then no automated production deletion schedule may activate.
Each rule names data set, jurisdiction/authority, trigger, active/archive
period, legal-hold behavior, disposal method, evidence and owner.

Legal hold is append-only, reasoned, approved, scope-bounded and time-reviewed.
It suspends matching disposal across primary data, documents, exports and
recoverable backups. Releasing a hold requires separate approval. Disposal is
blocked for posted/audit records where law or accounting integrity requires
retention; eligible data is cryptographically erased or provider-verified and
the manifest records counts/hashes without retaining the deleted content.

## Processor register and transfer assessment

For every IdP, host, database, object store, monitoring, support, bank, payment,
TRA/fiscal, messaging and backup provider record: legal entity, role,
subprocessors, data classes, purposes, regions, remote-support locations,
security measures, retention/deletion, breach duty, audit rights, contract/DPA,
transfer mechanism, owner and exit/export plan. `NOT SELECTED` is a blocker,
not an implicit approval.

## DPIA decision and minimum assessment

The DPIA covers multi-company finance, employee/payroll processing, customer
credit, device/offline monitoring, immutable audit, provider integrations,
cross-border hosting/support, vulnerable users, scale, access misuse, data loss
and rights limitations caused by statutory retention. It records necessity,
proportionality, threat likelihood/impact, controls, residual risk, DPO advice,
consultation need, owner acceptance and review triggers. Engineering cannot
accept high residual privacy risk for the owner.
