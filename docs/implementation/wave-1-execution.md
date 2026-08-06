# Wave 1 execution record

Updated: 2026-08-06

Wave 1 closes remaining executable product gaps without altering the locked
Wave 0 readiness baseline. Completion here means repository implementation and
verification only; it does not grant production launch authorization.

## Slice 1 — Governed executive dashboard

Status: repository implementation complete

Implemented evidence:

- `GET /v1/dashboard` is an implemented OpenAPI 3.1 operation with generated
  TypeScript and Dart bindings.
- The service derives month-to-date revenue, net profit, cash position and
  total assets from the same immutable journal read models used by financial
  statements. Monetary arithmetic stays in integer minor units until the
  decimal API boundary.
- `dashboard.read` and `reports.financial.read` are both required. Optional
  legal-company and branch assertions are rejected if they differ from the
  verified actor scope.
- Pending approvals count submitted operation and finance documents only in
  the authenticated branch and warehouse. It is not presented as a global
  master-data or workforce approval queue.
- The Next.js dashboard is force-dynamic, loads through the server-only live
  repository, exposes permission-shaped actions and renders a truthful
  unavailable state on authentication, configuration or upstream failure.
- The prior demonstration revenue curve, sales mix, branch rankings, activity
  feed and close-readiness score are no longer on the executable dashboard
  route. No mock data is substituted.
- Go service, scope-boundary, repository and full-suite tests pass. Control
  Center API-client and rendering tests, typecheck, lint and production build
  are release checks for this slice.

Known boundary:

- Monetary metrics are legal-company financial-reporting values because
  immutable journals currently own legal-company, not branch, attribution.
  The transactional approval count is branch/warehouse scoped. The UI states
  that distinction explicitly.
- The alerts array is intentionally empty until a governed exception
  projection with stable source identifiers exists; the UI says no alert was
  emitted and does not imply a simulated all-clear.

## Remaining Wave 1 work

- Scheduled report delivery and immutable delivery/retry evidence.
- Removal or replacement of the remaining generic demonstration routes.
- Authoritative cross-module approval inbox, notifications, global search,
  saved filters and audit drawers.
- Cross-module Release 1 acceptance packs and reconciliation of core coverage.
- Control Center responsive/accessibility/bilingual acceptance closure.
- Android POS online credit, printing, return/support flows and signed release
  pipeline.
