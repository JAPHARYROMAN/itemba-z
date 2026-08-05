# Live Golden Sale milestone

## Purpose

This milestone connects the Control Center and Android Sales POS to the
authoritative Go API and PostgreSQL database for one complete sales lifecycle.
It is an integration checkpoint, not the Release 1 production boundary.

## Transaction boundary

For every accepted sale, one database transaction must commit all of the
following exactly once:

- immutable sale header and lines using server-owned price, tax, and cost;
- stock issue movements in the allocated warehouse;
- cash payment or customer receivable entry;
- balanced general-ledger journal;
- immutable audit event and transactional outbox event;
- the idempotency result returned to subsequent identical retries.

A reversal is a new linked record. It restores the original stock and financial
effects while preserving the original transaction and its audit history.

## Client flow

1. An authenticated operator loads their server-derived working context.
2. The client loads scoped customers, products, current prices, and warehouse
   availability; no client may select a different organizational scope.
3. Android devices enroll against the authenticated actor and allocated scope.
   A new device remains cache-unacknowledged until it atomically installs the
   current catalog and price cache under one immutable snapshot token and
   acknowledges that token with both available versions.
4. The client submits product IDs and whole-unit quantities. The API recomputes
   all monetary values and credit eligibility.
5. Online web sales use `Idempotency-Key`. Mobile sales additionally use the
   unique `device_id + client_transaction_id` identity.
6. A retry returns the original sale and receipt state. A different request
   reusing the same identity is rejected as a conflict.
7. Authorized users may create a reasoned linked reversal; posted records are
   never edited or deleted.

## Authentication boundary

- Production uses OIDC bearer tokens whose verified claims provide the actor,
  tenant, legal company, branch, and warehouse scope.
- Browser credentials remain on the Next.js server boundary and are never
  exposed through public environment variables or client bundles.
- Android bearer credentials and device allocation are stored only through the
  encrypted application persistence and Android Keystore boundary.
- Development header identity is allowed only when the API and clients are
  explicitly configured for local development. Every production process fails
  closed when OIDC configuration or credentials are absent.

## Acceptance scenarios

| Scenario | Required evidence |
| --- | --- |
| Cash sale | Payment, stock issue, balanced journal, audit, outbox, and receipt state share one sale ID and correlation ID. |
| Online credit sale | General Customer is rejected; an eligible customer creates a due-dated receivable only after effective policy, risk, overdue, reconciliation, and post-sale exposure checks. |
| Duplicate web command | Repeating the same idempotency key and body returns the original sale; a changed body conflicts. |
| Duplicate mobile synchronization | Repeating `device_id + client_transaction_id` returns the original server sale and cannot post twice. |
| Unauthorized device | An unbound, disabled, actor-mismatched, or scope-mismatched device cannot synchronize. |
| Device suspension | A separately authorized, reasoned, idempotent command changes ACTIVE to SUSPENDED, ends current offline authorization, and appends audit/outbox/change evidence. A repeated command cannot duplicate evidence. |
| Device reactivation | SUSPENDED can return to ACTIVE with separate evidence, but offline sales remain unavailable until the device successfully acknowledges its installation and receives a fresh lease. REVOKED is never reactivated. |
| Offline allocation change | An exact-scope operator sets an absolute product allocation with a reason. The command rejects quantities below synchronized consumption and total remaining reservations above live warehouse stock. |
| Offline cash sale | The encrypted queue survives restart and synchronizes exactly once from the acknowledged immutable publication, including after later customer, product, price, cost, posting-account, or tax configuration drift. This milestone accepts physical CASH and publication-owned zero-rated lines only. |
| Catalog snapshot download | Every customer and product page echoes the requested immutable token and identical version metadata; token drift aborts the download before installation. |
| Cache acknowledgement | Installed and available token/version triples are distinct; stale acknowledgements fail, while retry after a lost response is idempotent. |
| Offline cache lease | A successful acknowledgement issues an exact app/token/master/price lease for no more than four hours and never across a known tax transition; historical lease and catalog-publication rows preserve both authorization and posting facts after renewal. |
| Missing publication evidence | The API returns `offline_reconciliation_required` with no business effects, registers one append-only scoped case with the exact command, and emits audit/outbox evidence. The POS retains the command as Reconciliation Required across restart and does not automatically retry it. |
| Offline accounting time | The retained client timestamp is `document_at`; authoritative server receipt is both `received_at` and `accounting_at` under the effective company policy. Stock, payment, customer, and journal effects use accounting time. |
| Device clock skew | A document beyond the configured future-skew ceiling registers `offline_clock_reconciliation_required` with the exact command and no business effects. |
| Fiscal-period crossing | Document and receipt must belong to the same fiscal period. A boundary crossing or unavailable current period registers `offline_fiscal_period_reconciliation_required` and does not backdate or post implicitly. |
| Reconciliation resolution | A separately authorized operator appends exactly one idempotent `CASH_REFUNDED`, `POSTED_EXTERNALLY`, or `DUPLICATE_CONFIRMED` disposition. Resolution creates audit/outbox evidence but never stock, cash, sale, or journal effects. |
| Late tax scheduling | Tax-rule mutations that overlap an outstanding lease are rejected under the same serialization lock used by acknowledgement. |
| Queued business day | Offline daily limits are charged to the lease-validated client creation day in the legal-company timezone, even when synchronization occurs after midnight. |
| Exact integers | TZS minor-unit amounts and whole-unit quantities remain within the JSON safe-integer contract or fail with 422 and no effects. |
| Offline credit sale | Completion is prohibited even when a device previously cached an eligible account. |
| Insufficient stock | The complete transaction rolls back with no sale, ledger, audit, or outbox residue. |
| Reversal | A linked reversal restores stock and financial effects and cannot be repeated. |
| Scope isolation | Cross-tenant, cross-company, cross-branch, and cross-warehouse reads and commands are denied. |

## Explicitly deferred

An identity-provider selection and production login user experience remain a
deployment decision. TRA fiscal receipt submission, statutory receipt numbers,
payment-provider confirmation, and notification delivery remain integration
workstreams; the current receipt state must not be represented as a TRA fiscal
receipt until those integrations receive professional validation. Flutter credit
sale entry remains deferred, although online mobile synchronization now reaches
the authoritative AR-ageing and effective-policy gate. Offline credit remains
prohibited and no client may infer a zero overdue balance.
Offline nonzero-tax sale completion remains fail-closed pending professional
validation of document-time tax and fiscalization policy. Posting now owns the
effective tax rules in each immutable publication, but the current release
slice deliberately accepts only zero-rated offline lines. The server now
registers and resolves offline exceptions through a permission-separated,
exact-scope API and bilingual Control Center evidence screen. Business approval
assignment remains to be implemented; the mobile client continues to preserve
and pause the exact command without authority to alter or repost it. Offline
document time is retained as evidence while server receipt time governs
accounting. The effective-dated company policy rejects excess future skew and
routes fiscal-period crossings to reconciliation, so the system makes no
implicit historical-period posting claim.

This boundary follows the modular, event-driven, observable, secure, and
testable engineering doctrine in NEXT Constitution sections 3, 5, and 6 while
retaining the stack deviations recorded in ADR-0001.
