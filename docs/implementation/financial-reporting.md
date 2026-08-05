# Financial statements and reporting

Updated: 2026-08-05

This slice implements legal-company, base-currency financial statements directly from the immutable general ledger. A branch or warehouse in the authenticated working context establishes the exact permission assignment; the report itself covers the selected legal company’s books and says so in the Control Center.

## Governed definitions

- **Trial balance:** cumulative debit-minus-credit balance per active or inactive GL account through the selected business date. Total debits must equal total credits.
- **General ledger:** opening balance, chronological journal lines, operational source identity, memo, debits, credits, running balance, and closing balance for one account and a maximum 366-day range.
- **Profit and loss:** period credits-minus-debits for revenue accounts, debits-minus-credits for expense accounts, and net profit. Unclassified account movement remains visible as an exception.
- **Balance sheet:** cumulative asset debit balances and liability/equity credit balances through the selected date. Revenue and expense balances roll into current earnings. The statement is marked balanced only when the accounting equation holds and no unclassified balance remains.
- **Cash flow:** opening cash plus journal-level cash movements equals independently calculated closing cash. Known source types are classified as operating, investing, or financing; unknown cash sources remain explicitly unclassified and require review.

All date boundaries use `legal_companies.business_timezone`; Tanzania development fixtures use `Africa/Dar_es_Salaam`. Reports never use browser or UTC midnight as the accounting-day boundary.

## Security and evidence

- `reports.financial.read` runs statements and account drill-down.
- `reports.financial.export` generates CSV artifacts in addition to requiring read access.
- Exports require an idempotency key, retain their exact base64 content and metadata, and emit audit and transactional-outbox evidence.
- PostgreSQL row-level security and every query retain tenant and legal-company predicates.
- Public monetary values remain integer minor units within JavaScript’s exact integer range.

## Acceptance evidence

Automated memory and PostgreSQL tests prove that the same journals reconcile across trial balance, profit and loss, balance sheet, cash flow, and general-ledger drill-down. Migration verification proves permissions, indexes, RLS-protected retained exports, and development roles on a clean database. OpenAPI lint, generated TypeScript/Dart checks, Control Center tests, production builds, and the existing live HTTP workflow remain release gates.

## Next reporting extensions

Comparative periods, consolidated group statements, budgets/variance, fixed-asset schedules, PDF/XLSX packs, schedules, and governed delivery require their owning finance modules and remain later roadmap work. The current CSV export and statement contracts are versioned foundations for those extensions.
