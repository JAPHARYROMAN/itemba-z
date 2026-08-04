# Security threat model

## Assets and trust boundaries

Restricted assets include payroll and identity data, supplier bank details, customer credit information, tax records, API credentials, audit records, and ledger entries. Trust boundaries exist between users and clients, clients and the API, API modules, worker and external connectors, database/object storage, and each tenant/legal company.

## Principal threats and controls

| Threat | Required controls |
| --- | --- |
| Cross-tenant or cross-company access | Tenant and company keys in owned tables and constraints; backend policy checks; PostgreSQL RLS as defense in depth; synthetic two-tenant tests |
| Privilege escalation | OIDC, short-lived tokens, MFA for sensitive roles, contextual RBAC, separation of duties, reauthentication, immutable permission-change audit |
| Duplicate or replayed transactions | Idempotency keys, mobile device/transaction uniqueness, request hashing, original-response replay, database uniqueness constraints |
| Silent financial or stock tampering | Append-only ledgers, immutable posted documents, linked reversals, balanced-journal constraints, actor/reason/approval audit |
| Lost or stolen mobile device | Device approval and suspension, encrypted local data, scoped cache, token rotation, remote invalidation, offline value and stock limits |
| Connector compromise or failure | Per-connector credentials, outbound allowlists, signed requests where supported, rate limits, retry budgets, dead-letter review, manual replay, no rollback of posted core transactions |
| Sensitive-data leakage in telemetry | Structured allowlisted fields, authorization-header removal, user-ID hashing, no payload bodies, restricted access and retention |
| Data loss or ransomware | Encrypted PITR backups, immutable secondary copy, least-privilege operators, quarterly restore rehearsal, documented recovery authority |

## Security gates

Threat-model review, dependency and secret scanning, static analysis, permission tests, API abuse tests, penetration testing, backup restoration, incident simulation, and privacy impact assessment are release requirements. Unresolved critical or high findings block production.
