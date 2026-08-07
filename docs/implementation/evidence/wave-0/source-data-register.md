# Wave 0 source-data register

- Baseline date: 2026-08-06
- Status: required sources classified; actual Itemba production files and owners not provided
- Handling: this repository contains metadata only, never production personal, payroll, banking or commercial source files

## Source register

| ID | Domain | Required source | Authoritative owner | Actual source supplied | Classification | Required control total / acceptance |
| --- | --- | --- | --- | --- | --- | --- |
| SRC-001 | Organization | Group, legal-company registrations, TINs, base currency and business timezone | UNASSIGNED | NO | RESTRICTED | Every legal entity has approved identifiers, books and scope |
| SRC-002 | Organization | Branches, departments, cost centres and warehouses | UNASSIGNED | NO | INTERNAL | Complete hierarchy with ownership and active dates |
| SRC-003 | Security | Users, roles, employment status and scoped access | UNASSIGNED | NO | RESTRICTED | Every active user maps to one approved identity and least-privilege assignment |
| SRC-004 | Customers | Customer masters, tax IDs, contacts, terms and credit policies | UNASSIGNED | NO | RESTRICTED | Deduplicated count, General Customer identified, credit approvals retained |
| SRC-005 | Receivables | Open invoices, credit notes, collections and allocations | UNASSIGNED | NO | RESTRICTED | Customer subledger equals AR control account |
| SRC-006 | Suppliers | Supplier masters, tax IDs, terms and protected bank details | UNASSIGNED | NO | HIGHLY RESTRICTED | Deduplicated suppliers; bank details separately verified |
| SRC-007 | Payables | Open supplier invoices, debit notes, payments and allocations | UNASSIGNED | NO | RESTRICTED | Supplier subledger equals AP control account |
| SRC-008 | Products | SKUs, descriptions, units, conversions, categories and active state | UNASSIGNED | NO | INTERNAL | Unique SKUs and approved unit conversion definitions |
| SRC-009 | Pricing/tax | Price lists, discounts, tax codes, effective dates and mappings | UNASSIGNED | NO | RESTRICTED | Approved effective-dated policy and sample calculation cases |
| SRC-010 | Inventory | Quantity by product, unit, legal owner and warehouse | UNASSIGNED | NO | RESTRICTED | Signed physical count equals opening stock |
| SRC-011 | Inventory | Lots/batches, manufacture/expiry dates and serials where applicable | UNASSIGNED | NO | RESTRICTED | Lot totals reconcile to product/location quantity |
| SRC-012 | Inventory valuation | Cost basis, standard/moving-average evidence and inventory GL | UNASSIGNED | NO | RESTRICTED | Opening valuation reconciles to inventory control accounts |
| SRC-013 | Finance | Chart of accounts and posting mappings | UNASSIGNED | NO | RESTRICTED | Active accounts/mappings approved; control accounts protected |
| SRC-014 | Finance | Trial balance and opening journals | UNASSIGNED | NO | HIGHLY RESTRICTED | Debits equal credits; retained earnings/opening policy approved |
| SRC-015 | Cash | Cash tills, floats and signed counts | UNASSIGNED | NO | HIGHLY RESTRICTED | Physical count equals opening cash by location/custodian |
| SRC-016 | Banks/mobile money | Statements, account masters, settlement balances and uncleared items | UNASSIGNED | NO | HIGHLY RESTRICTED | Approved statements equal opening ledger and outstanding items |
| SRC-017 | Fixed assets | Asset masters, acquisition evidence, useful life and accumulated depreciation | UNASSIGNED | NO | HIGHLY RESTRICTED | Asset register cost/depreciation equals GL controls |
| SRC-018 | Treasury | Loans, overdrafts, accrued interest and repayment schedules | UNASSIGNED | NO | HIGHLY RESTRICTED | Facility statements equal principal/interest ledger balances |
| SRC-019 | Budgets | Approved legal-company budgets and versions | UNASSIGNED | NO | RESTRICTED | Approved totals by period/account/cost centre |
| SRC-020 | Employees | Employee master, identity, contract, position and bank data | UNASSIGNED | NO | HIGHLY RESTRICTED | Active/inactive population and bank details independently verified |
| SRC-021 | Attendance/leave | Balances, attendance history, shifts and open requests | UNASSIGNED | NO | HIGHLY RESTRICTED | Approved opening leave and attendance exceptions |
| SRC-022 | Employee loans | Principal, repayments, deductions and balances | UNASSIGNED | NO | HIGHLY RESTRICTED | Employee schedules equal payroll and GL control accounts |
| SRC-023 | Payroll | Earnings, deductions, statutory data, YTD values and liabilities | UNASSIGNED | NO | HIGHLY RESTRICTED | Parallel payroll and liability reconciliation approved |
| SRC-024 | Documents | Required customer, supplier, employee, asset and transaction documents | UNASSIGNED | NO | HIGHLY RESTRICTED | Manifest count/hash, retention and access policy approved |
| SRC-025 | Legacy history | Read-only historical ledgers and documents retained outside opening import | UNASSIGNED | NO | RESTRICTED | Search/retrieval and retention responsibility approved |

## Required registration procedure

For every actual source, record outside this public repository:

- source owner and custodian;
- system/file name and version;
- immutable SHA-256 hash;
- extraction timestamp and cutoff;
- row count and control totals;
- confidentiality and personal-data classification;
- authoritative fields and known defects;
- mapping version and import batch ID;
- approved corrections and rejection counts;
- retention, legal hold and disposal authority.

## Current finding

No production source files were supplied in the workspace. Trial Migration 1 cannot begin until the executive sponsor and migration lead appoint owners and place approved extracts in a restricted staging location. The deterministic `devseed` data is fictional and is prohibited as a migration source.
