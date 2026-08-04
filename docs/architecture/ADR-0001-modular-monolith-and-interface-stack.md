# ADR-0001: Modular monolith and interface stack

- **Status:** Accepted
- **Date:** 2026-08-04
- **Owners:** ITEMBA-Z Architecture Council

## Context

ITEMBA-Z must keep sales, stock movements, customer or supplier ledger effects, balanced journals, payment records, audit events, and outbox events consistent. Splitting these effects across independently committed services at launch would introduce distributed-transaction failure modes into the system's golden invariants.

The NEXT Master Build Constitution requires modular, event-driven, observable, resilient, secure, and extensible systems (Sections 3, 5, and 6), while its preferred stack names React Native, GraphQL Federation, gRPC, Kafka, and Kubernetes (Section 10). The approved ITEMBA-Z blueprint instead selects Flutter, REST/OpenAPI, a Go modular monolith, and managed containers for the first release.

## Decision

1. Deploy the transactional core as a Go modular monolith with explicit bounded contexts and a separately runnable worker from the same repository.
2. Modules own their packages and database schema areas. A module may call another module only through typed application interfaces; it may not mutate another module's tables.
3. Coordinate critical cross-module effects inside one PostgreSQL transaction and write domain events to a transactional outbox before commit.
4. Expose REST/JSON through OpenAPI 3.1. Generate TypeScript and Dart clients from the same contract. Reserve gRPC for a future extracted service and GraphQL for an optional read-only composition layer.
5. Use Flutter for the Android-first POS because offline storage, background synchronization, receipt printing, and device integration outweigh source-code sharing with the web application.
6. Begin on managed containers, managed PostgreSQL, Redis, and S3-compatible storage. Introduce Kubernetes, Kafka, or dedicated analytics/search systems only against measured scale or operational needs.

## Consequences

- Critical ERP invariants are easier to prove and recover.
- Clear module contracts and outbox events preserve an extraction path without paying distributed-system costs immediately.
- The Flutter and REST decisions intentionally deviate from Constitution Section 10 while retaining its system qualities.
- First extraction candidates are notifications, fiscal/payment integrations, documents, reporting, and AI because their failure semantics are asynchronous.
