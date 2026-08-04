# Core process maps

## Order to cash

```text
Quotation -> Sales order -> Reservation -> Delivery -> Invoice -> Payment
                                    \-> Quick sale -> Payment or online credit validation
```

Posting commits sale, stock, customer/payment, tax, balanced journal, audit, and outbox records together. Receipt generation, fiscal delivery, notifications, and reporting projection continue asynchronously and are reconciled without changing the posted sale.

## Procure to pay

```text
Purchase request -> Approval -> RFQ -> Supplier comparison -> Purchase order
  -> Goods receipt -> Supplier invoice -> Three-way match -> Payment
```

Goods receipt owns the incoming stock movement. Supplier invoice owns the payable and expense/inventory accounting. Matching tolerances are effective-dated policy, not client logic.

## Inventory control

```text
Count plan -> Blind count -> Variance -> Review -> Approval -> Adjustment movement -> Journal
Transfer request -> Dispatch -> Stock in transit -> Receipt -> Destination balance
```

No screen edits a balance. Every state change references an append-only movement.

## Record to report

```text
Operational postings -> Subledgers -> General ledger -> Reconciliation
  -> Close checklist -> Period close -> Financial statements -> Consolidation
```

Governed metrics share one definition across dashboards, exports, scheduled reports, and future AI.

## Hire to payroll

```text
Employment -> Attendance/leave/loans -> Payroll inputs -> Calculate -> Review
  -> Independent approval -> Post -> Payment schedule -> Payslips -> Reconciliation
```

Payroll configuration is jurisdiction- and effective-date-aware. A posted payroll is corrected through adjustment or reversal.

## Mobile synchronization

```text
Encrypted draft -> Local policy checks -> Pending sync -> Server authentication/scope
  -> Idempotency claim -> Server business validation -> Atomic post
  -> Synced | Rejected | Requires review
```

Repeated `device_id + client_transaction_id` submissions return the original result. Credit sales never enter the offline completion path.
