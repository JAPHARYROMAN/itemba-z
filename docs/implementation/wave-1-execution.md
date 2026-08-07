# Wave 1 execution record

Updated: 2026-08-06

Wave 1 closes remaining executable product gaps without altering the locked
Wave 0 readiness baseline. Completion here means repository implementation and
verification only; it does not grant production launch authorization.

Repository status: **COMPLETE**

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

## Slice 2 — Product truth and Control Center operations

Status: repository implementation complete

- Generic mock route families, their mock repository and illustrative ERP data
  are removed. `/suppliers` is now an explicit live supplier directory.
- Global search queries only sources authorized by the verified working
  context and exposes its live source set. Header notifications use the
  governed dashboard projection; all hard-coded notices are removed.
- `GET /v1/audit/{entity_type}/{entity_id}` provides reusable,
  permission-bound immutable timelines. Posted sales expose a
  keyboard-accessible audit drawer.
- Search supports Command/Ctrl+K and new content, empty and unavailable states
  are bilingual and responsive.
- Persisted personal saved-filter preferences were removed from the approved
  Release 1 scope during reconciliation: they are neither authoritative ERP
  data nor a release-blocking workflow. They remain a post-launch usability
  enhancement and no current screen claims that a filter was saved.
- The audit endpoint and drawer are reusable across entity types; Wave 1 binds
  the drawer to the immutable posted-sale surface where correction evidence is
  required. Additional module placements are presentation enhancements, not
  separate audit stores or Release 1 posting controls.

## Slice 3 — Android POS closure

Status: repository implementation complete

- Online credit entry restricts selection to active, registered,
  credit-enabled customers and retains server-authoritative overdue/limit
  decisions. General Customer and offline credit remain blocked.
- Internal receipt references and fiscal status are visibly distinct. Until an
  approved printer adapter exists, printing is disclosed as unavailable
  instead of being represented by a no-op control.
- Return/cancellation guidance no longer claims to submit an unpersisted
  request. It exposes the immutable references required for a manager to use
  the governed Control Center reversal.
- The protected release job builds signed APK/AAB artifacts, records SHA-256
  hashes, retains controlled artifacts and emits build-provenance attestations.

## Slice 4 — Cross-module acceptance and scope reconciliation

Status: repository implementation complete

- `TestRelease1CrossModuleAcceptanceAndReversal` covers order-to-cash,
  procure-to-pay, inventory, record-to-report and hire-to-retire in one
  reconciled PostgreSQL scenario, including immutable audit retrieval.
- Standard cost and moving average remain the approved executable costing
  methods. No additional method is admitted without an Itemba accounting
  policy and acceptance cases.
- Outbound scheduled email/WhatsApp delivery and physical printer-model
  adapters are Wave 3 provider integrations. Retained governed CSV/PDF/XLSX
  report packs remain in Release 1 core.
- Statutory payroll formulas and filing outputs remain Wave 3 pending
  effective-dated professional approval; payroll lifecycle, posting, loans,
  artifacts and reconciliation remain in core.

## Wave 1 repository exit

- Approved repository core scope is reconciled to 100%; provider-dependent
  work is assigned to Wave 3 instead of counted as partially implemented core.
- No executable Control Center route resolves to demonstration ERP data.
- Automated Go, OpenAPI, Next.js, Flutter and PostgreSQL acceptance gates are
  retained. Human accessibility, device, printer and business UAT evidence
  remains a Wave 5 release gate and is not implied by engineering completion.
