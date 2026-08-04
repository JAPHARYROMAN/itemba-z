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
   current catalog and price cache and acknowledges both available versions.
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
| Control Center credit sale | General Customer is rejected; an eligible customer creates a receivable within its limit. Mobile credit remains fail-closed. |
| Duplicate web command | Repeating the same idempotency key and body returns the original sale; a changed body conflicts. |
| Duplicate mobile synchronization | Repeating `device_id + client_transaction_id` returns the original server sale and cannot post twice. |
| Unauthorized device | An unbound, disabled, actor-mismatched, or scope-mismatched device cannot synchronize. |
| Offline cash sale | The encrypted queue survives restart and synchronizes exactly once when the authoritative product, master, price, and tax facts remain compatible; this milestone accepts physical CASH and authoritative zero-rated lines only, while detected drift fails closed for governed review and reconciliation. |
| Cache acknowledgement | Installed and available versions are distinct; stale acknowledgements fail, while retry after a lost response is idempotent. |
| Offline cache lease | A successful acknowledgement issues an exact app/master/price lease for no more than four hours and never across a known tax transition; historical rows preserve the authorization proof after renewal, while later governed-data drift still fails closed. |
| Late tax scheduling | Tax-rule mutations that overlap an outstanding lease are rejected under the same serialization lock used by acknowledgement. |
| Queued business day | Offline daily limits are charged to the lease-validated client creation day in the legal-company timezone, even when synchronization occurs after midnight. |
| Exact integers | TZS minor-unit amounts and whole-unit quantities remain within the JSON safe-integer contract or fail with 422 and no effects. |
| Offline credit sale | Completion is blocked until an authoritative online credit check is available. |
| Insufficient stock | The complete transaction rolls back with no sale, ledger, audit, or outbox residue. |
| Reversal | A linked reversal restores stock and financial effects and cannot be repeated. |
| Scope isolation | Cross-tenant, cross-company, cross-branch, and cross-warehouse reads and commands are denied. |

## Explicitly deferred

An identity-provider selection and production login user experience remain a
deployment decision. TRA fiscal receipt submission, statutory receipt numbers,
payment-provider confirmation, and notification delivery remain integration
workstreams; the current receipt state must not be represented as a TRA fiscal
receipt until those integrations receive professional validation. Mobile credit
sale entry also remains fail-closed until the authoritative AR-aging and credit
policy contract supplies overdue amount, due date, approval state, and the
expected post-sale balance. The client must never infer a zero overdue balance.
Offline nonzero-tax sale completion also remains fail-closed until posting owns
a governed historical tax/clock snapshot; the live product tax rate is exposed
for exact review totals but is not sufficient statutory evidence.
The bounded lease is an operational safety boundary, not that snapshot:
catalog download and acknowledgement still need an immutable as-of token or
content fingerprint to eliminate a download/activation race before production
offline sales can be approved.
Production continuity across catalog or price changes additionally requires
versioned historical product/price facts, a consistent paginated snapshot, and
an operator workflow that reconciles collected cash when a queued transaction
fails closed. The current milestone demonstrates conflict detection and durable
review state; it does not claim unattended recovery across configuration drift.
Release 1 also needs a governed distinction between offline document time and
server posting time, including clock-skew limits, fiscal-period policy, and a
reconciliation path for sales synchronized after a period boundary. The
current slice evaluates fiscal-period openness and records posting time when
the server accepts the command, so it makes no historical-period posting claim.

This boundary follows the modular, event-driven, observable, secure, and
testable engineering doctrine in NEXT Constitution sections 3, 5, and 6 while
retaining the stack deviations recorded in ADR-0001.
