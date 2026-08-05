# Treasury facilities

ITEMBA-Z models legal-company borrowing as governed facility masters plus an append-only transaction subledger. Facility facts cannot be edited after creation; corrections require a new governed transaction or a rejected/replaced draft.

The lifecycle is `DRAFT → SUBMITTED → ACTIVE | REJECTED`, with maker-checker enforcement on the decision. An active facility can close only after both outstanding principal and accrued interest reconcile to zero.

Supported postings are:

- Drawdown: debit bank, credit principal liability.
- Principal repayment: debit principal liability, credit bank.
- Interest accrual: debit interest expense, credit accrued-interest liability.
- Interest payment: debit accrued-interest liability, credit bank.

Every command is scope-authorized and idempotent. Postings require an active facility, company base currency, an open fiscal period, governed account types, and sufficient facility/balance capacity. The journal, treasury transaction, audit event, and outbox event commit atomically. PostgreSQL row-level security and mutation guards protect tenant isolation and historical evidence.

The Control Center supplies English and Swahili creation, approval, posting, balance, and settlement controls. Interest rates are captured as facility configuration in basis points; the system does not infer or hardcode statutory or bank-specific rates.
