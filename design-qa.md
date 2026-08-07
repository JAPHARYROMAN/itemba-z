# ITEMBA-Z Calm Operations — Design QA

## Scope

- Verified experience: Control Center shell and Sales reference route tree.
- Live URL: `http://localhost:3000/sales`
- Reference: `C:\Users\user\.codex\generated_images\019fcb0e-dc88-7372-8ee4-379e29e8f003\exec-1298865f-1a23-42ee-aac1-a3b28168b203.png`
- Desktop implementation: `C:\Users\user\OneDrive\Documents\itemba-z\.qa\calm-sales-desktop-release.png`
- Mobile implementation: `C:\Users\user\OneDrive\Documents\itemba-z\.qa\calm-sales-mobile-release.png`
- Combined desktop comparison: `C:\Users\user\OneDrive\Documents\itemba-z\.qa\calm-sales-desktop-release-comparison.png`

## Comparison setup

- Reference pixels: 1487 × 1058.
- Desktop browser viewport: 1487 × 1058 at DPR 1; captured content: 1472 × 1048 after browser scrollbar/chrome.
- Mobile browser viewport: 390 × 844 at DPR 1; captured content: 375 × 811 after browser scrollbar/chrome.
- Desktop reference and implementation were joined into a single 2974 × 1058 comparison input and inspected together at original detail.
- No mobile source frame exists. The mobile capture was evaluated independently for responsive integrity, content hierarchy, readable wrapping, and working navigation.

## State alignment

- The reference contains illustrative ready-order counts and five mock transaction types. The implementation deliberately renders authoritative local ERP state: two posted sales, no ready orders, and two unresolved fiscal-configuration exceptions.
- The implementation uses the complete permission-shaped application navigation required by the approved information architecture, while the visual reference used a smaller Sales-only sidebar.
- Accounting period and Help are absent because the current API does not provide an authoritative period or Help destination. No operational values or routes were fabricated.

## Iteration findings and resolutions

### Iteration 1

- P1: raw UUID receipt references and system-oriented status language were visible. Resolved with compact business references, named entities, formatted TZS values, and business-readable fiscal states.
- P1: the desktop navigation participated in a focus trap. Resolved so trapping, body scroll lock, Escape dismissal, and focus restoration apply only to the open mobile drawer.
- P1: Sales quick actions pointed at page anchors instead of dedicated tasks. Resolved with Transactions, Quotes & orders, Payments, and Returns routes.
- P1: fiscal `NOT_CONFIGURED` appeared as a normal state. Resolved as a visible warning and attention exception.
- P1: the payment flow exposed implementation details and did not preserve recovery intent. Resolved with customer and open-invoice selection, normal TZS entry, authoritative server loading, and a persistent idempotency key across ambiguous retries.
- P2: four independent quick-action cards diverged from the approved direction. Resolved as a three-segment desktop action surface that becomes stacked touch-friendly cards on narrow screens.
- P2: company, branch, and warehouse values truncated on mobile. Resolved with wrapped context values and verified no horizontal page overflow.
- P2: technical task titles such as “Sales lifecycle” and generic “Transaction register” remained on task pages. Resolved with “Quotes & orders” and “Returns & corrections” in both English and Swahili.

### Iteration 2

- Desktop source/implementation comparison found no remaining P0, P1, or P2 visual defect in the verified scope.
- Mobile inspection found no clipped controls, page-level horizontal overflow, unreadable context value, or inaccessible primary action.
- Intentional state and information-architecture differences are documented above and do not represent visual regressions.

### Iteration 3 — release-safety audit

- P0: backend fulfilment accepted an approved non-order source document. Resolved by locking and requiring an exact `SALES_ORDER` before any stock, sale, reservation, or status effect; a PostgreSQL regression test proves an approved quotation is rejected without side effects.
- P1: quote/order create and transition commands created a fresh retry identity after a lost response. Resolved with persisted command identities, an atomic in-flight guard, and authoritative-success clearing.
- P1: quote/order price entry used floating-point conversion. Resolved with exact string and `BigInt` conversion to safe minor units.
- P1: Sales document reads silently stopped after the first page. Resolved with bounded, duplicate-safe, cursor-safe pagination.
- P1: least-privilege navigation could expose Sales tasks whose loaders required more permissions. Resolved with one tested route-permission matrix used by navigation, task links, and loaders.
- P1: resizing an open mobile drawer into desktop mode could leave the application inert. Resolved with a breakpoint listener and regression coverage.
- P1: mobile approvals, notifications, and currency could disappear. Resolved with compact persistent actions and a two-column context layout.
- P1: primary navigation labelling, small-text contrast, hidden Sales-tab overflow, and technical retry copy were corrected. Verified contrast ratios are at least 4.64:1 for the adjusted small-text combinations.
- P2: a fractional profile-control boundary caused one pixel of page overflow at 320 px. The non-critical profile control is now omitted only below 341 px; approvals, notifications, language, and currency remain available. Reflow is clean at 320, 360, 768, 1024, and 1487 px.

## Interaction and accessibility evidence

- Mobile drawer: unique trigger; focus moves to Close navigation; body scroll locks; Escape closes; focus returns to Open navigation.
- Language: EN → SW changes the Sales heading to “Mauzo” and the active language state to SW; EN restores the English catalog.
- Task routes: `/sales/transactions`, `/sales/documents`, `/sales/payments`, and `/sales/returns` each render one main landmark, one task-level H1, no visible raw UUID, and no page-level horizontal overflow.
- Desktop and mobile Sales home each render one main landmark and no visible raw UUID.
- Desktop dimensions: document scroll width equals client width (1472 px).
- Mobile dimensions: document scroll width equals client width (375 px).
- Extreme narrow dimensions: document scroll width equals client width (305 px at a requested 320 px viewport).
- Drawer breakpoint: opening at 390 px sets focus, scroll lock, and inert state; expanding to 1200 px closes the drawer and restores interactive desktop content.
- Console: zero browser warnings or errors across the verified routes.

## Automated gates

- ESLint: passed.
- TypeScript: passed.
- Vitest: 34 files and 109 tests passed.
- Next.js production build: passed.
- `npm audit --audit-level=high`: zero vulnerabilities.
- Core API: complete `go test ./...` suite passed, including the PostgreSQL source-document regression.
- Docker Core API and Control Center images: rebuilt, healthy, and running.

## Commercial workspace extension

- Verified live routes: `/customers`, `/customers/[customerId]`, `/suppliers`, and `/suppliers/[supplierId]`.
- Customer directory presents readable buying terms, authoritative TZS exposure, collection access, search, filters, and mobile record cards without exposing internal identifiers.
- Supplier directory presents approved partners, payment terms, sourcing activity, and a permission-shaped approval queue; supplier profiles progressively disclose the technical record identifier under System details.
- Supplier approval projections are fetched only when both `masterdata.read` and `purchases.sourcing.read` are granted; normal supplier reads do not fail because approval access is absent.
- English and Swahili directory and supplier-profile states were inspected in the live browser.
- Desktop at 1440 × 1000 and mobile at 390 × 844 have no page-level horizontal overflow; mobile directory links now retain a 44 px target.
- Customer credit-limit entry now uses exact string-to-minor-unit conversion, and risk states are rendered as business language rather than raw enums.
- Automated gates after the extension: ESLint passed, TypeScript passed, 36 Vitest files / 111 tests passed, production build passed, and npm audit reported zero vulnerabilities.

## Purchasing workspace extension

- Verified live routes: `/purchases`, `/purchases/requests`, `/purchases/sourcing`, `/purchases/orders`, `/purchases/receipts`, `/purchases/bills`, `/purchases/payments`, `/purchases/returns`, and `/purchases/[documentId]`.
- Desktop implementation: `C:\Users\user\OneDrive\Documents\itemba-z\.qa\purchases\purchases-desktop-final.png`.
- Mobile implementation: `C:\Users\user\OneDrive\Documents\itemba-z\.qa\purchases\purchases-mobile-final.png`.
- The Purchasing home now provides task-first entry points and exception-oriented attention states across the complete procure-to-pay chain.
- Each operational task has a dedicated composer and register. Downstream tasks select approved business document numbers; supplier and line evidence remain linked to the authoritative source instead of requiring pasted identifiers.
- Competitive sourcing now focuses on RFQs, comparable supplier quotes, independent approval, and traceable award decisions. Supplier master-data governance remains in the Supplier workspace.
- Create, transition, and sourcing mutations preserve one idempotency key across ambiguous retries. Currency entry uses exact major-to-minor conversion and total validation rejects unsafe integer values.
- Purchase document details follow summary, business evidence, recursive source chain, and progressively disclosed System details.
- Permission tests cover route visibility, least-privilege landing, and exact manage capabilities. Source and transition tests cover approved/posted eligibility and append-only status decisions.
- English and Swahili route states were inspected in the live browser. Desktop at 1440 × 1000 and mobile at 390 × 844 render one main landmark and one H1, expose no raw UUID, and have no page-level horizontal overflow.
- A mobile input clipping defect found during visual QA was resolved with zero-minimum grid columns, constrained controls, and responsive card padding; final measured product, quantity, and price fields stay within the viewport.
- The rebuilt Control Center container is healthy and serves the final experience at `http://localhost:3000/purchases`.
- Final automated gates: OpenAPI generation passed, ESLint passed, TypeScript passed, 39 Vitest files / 117 tests passed, Next.js production build passed, and the production-runtime dependency audit reported zero vulnerabilities.
- The full development audit reports the current high-severity `js-yaml` advisory only through `openapi-typescript` → `@redocly/openapi-core` 1.x. The patched Redocly 2.x release is incompatible with `openapi-typescript` 7.13.0 and was verified to break generation; the affected development-only packages are not copied into the production runner image.

final result: passed
