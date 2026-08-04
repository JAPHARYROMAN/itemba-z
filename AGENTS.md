# ITEMBA-Z repository guidance

## Governing sources

The ITEMBA-Z Master Product, Business and Engineering Blueprint is the domain source of truth. The NEXT Master Build Constitution governs system quality, modularity, eventing, observability, resilience, security, extensibility, and calm accessible UX. ADR-0001 records intentional first-release stack deviations.

## Non-negotiable invariants

- Never update balances directly; append source-linked ledger or stock movements.
- Never edit or delete a posted transaction; create a reasoned, approved, linked reversal.
- Never accept a credit sale for General Customer or a manually entered customer.
- Never trust tenant, company, branch, warehouse, price, credit, or permission data supplied only by a client.
- Never use binary floating-point values for money or business quantities.
- Never commit a business document without all required stock, ledger, payment, audit, and outbox effects in the same transaction.
- Never make core posting depend on AI, notification, reporting, or external-connector availability.
- Every retryable command must have an idempotency strategy and mismatch behavior.

## Module boundaries

The Go core is a modular monolith. Modules communicate through typed application ports and do not mutate another module's tables. PostgreSQL is authoritative. Redis and projections are disposable derivatives.

The Control Center and Sales POS consume the versioned OpenAPI contract. Business rules remain authoritative in the Go API even when clients duplicate checks for usability or offline policy.

## Verification

Run the narrowest relevant tests during development and the full repository checks before handoff:

- Go: `go test ./...`, `go vet ./...`, and `govulncheck ./...` from `services/core-api`.
- Web: `npm audit --audit-level=high`, `npm run lint`, `npm run typecheck`, `npm test`, and `npm run build` from `apps/control-center`.
- Mobile: `flutter analyze`, `flutter test`, and `flutter build apk --debug` from `apps/sales-mobile`.
- Contracts: Redocly lint for OpenAPI and JSON parsing/schema checks for events.
- Infrastructure: `terraform fmt -check -recursive`, `terraform validate`, and `docker compose config`.

Tests must cover tenant isolation, permissions, balanced journals, stock conservation, credit controls, idempotency, reversal, closed periods, audit/outbox correlation, and failure rollback whenever affected.
