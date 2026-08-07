# People and payroll

The people module owns company-scoped employee records, append-only attendance, effective-dated bilingual leave configuration, leave approvals, employee loans, payroll runs, and payslip lines. Attendance corrections use linked negative reversals; approved leave cannot overlap; leave, loan, and payroll approvals prohibit maker self-approval.

An approved employee loan posts receivable against bank. Posted payroll debits salary expense and credits net payroll payable, configured deduction liabilities, and employee-loan receivables. The payroll transaction reduces the loan subledger in the same database transaction, and every journal must balance before commit. Statutory rates are deliberately absent: Tanzania payroll values must enter through professionally approved, effective-dated configuration rather than source-code constants.

All commands require idempotency keys. PostgreSQL row-level security, immutable history tables, audit events, and the transactional outbox preserve scope and evidence. The Control Center `/human-resources` workspace is bilingual and permission-aware.
