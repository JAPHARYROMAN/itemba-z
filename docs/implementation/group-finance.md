# Intercompany accounting and consolidation

ITEMBA-Z keeps each legal company's books separate while coordinating group transactions through a dual-company workflow. A source-company maker creates and submits an immutable transaction, a different source-company actor approves it, and a distinct counterparty-company actor confirms the final posting.

Final confirmation atomically creates one balanced journal in each company. Supported Release 1 patterns are cash transfers and shared-cost allocations. Both legal companies must use the same base currency, have open fiscal periods, and reference active accounts of the required type.

The consolidation report derives normal balances directly from every tenant legal company's immutable journals. It separately reports and eliminates posted intercompany receivable/payable balances. Cost allocations additionally eliminate internal recovery revenue and allocated expense. The report proves the consolidated accounting equation after eliminations and retains company-level drill-down totals.

Permissions are separated across read, manage, approve, and consolidation access. PostgreSQL tenant RLS, immutable transaction/transition guards, idempotency, audit events, and transactional outbox evidence apply throughout. The bilingual Control Center shows the acting company's permitted queue and never permits source approval from the counterparty context or final posting from the source context.
