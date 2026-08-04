# Migration runbook

## Controlled pipeline

1. Register every source spreadsheet, manual ledger, owner, cutoff date, confidentiality class, and authoritative field.
2. Copy sources into a restricted staging area and assign immutable source-file hashes and import batch IDs.
3. Map, normalize, deduplicate, and validate organization, parties, products, units, prices, employees, chart of accounts, and opening balances.
4. Produce row-level errors and control totals; business owners correct source data rather than editing posted destination records.
5. Run at least two complete trial migrations in a production-like environment.
6. Obtain approval for master records and opening positions.
7. Freeze changes, perform a signed physical stock count, migrate, reconcile, and retain source/batch lineage.
8. Keep legacy records read-only for historical lookup and execute rollback if a mandatory control total fails.

## Non-negotiable reconciliations

- Opening stock equals the signed physical count by product, unit, warehouse, and legal owner.
- Customer and supplier subledgers equal their general-ledger control accounts.
- Cash, bank, and mobile-money opening balances equal approved statements or counts.
- Trial-balance debits equal credits.
- Employee loans, advances, leave, and payroll opening values equal approved schedules.

No import may create unbalanced journals, negative opening stock without approved exception, duplicate parties without review, or posted records without batch, actor, timestamp, and approval references.
