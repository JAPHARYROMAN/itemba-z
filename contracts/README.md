# ITEMBA-Z contracts

Contracts are reviewed artifacts. Breaking changes require a new API or event version and a migration note.

- `openapi/itemba-z.v1.yaml` is the source for web, mobile, and connector clients.
- `events/event-envelope.v1.schema.json` defines the durable event envelope written through the transactional outbox.

Rules:

1. Posted transactional money is represented as integer minor units. Read models that expose `Money` and fractional quantities use decimal strings; clients must never use binary floating-point arithmetic for business values.
2. Entity identifiers are UUIDs. Identity-provider subjects are opaque strings, and human document numbers are separate fields.
3. Timestamps are RFC 3339 UTC values.
4. Tenant, legal-company, branch, and warehouse scope comes from verified identity claims at the HTTP boundary; actor, correlation, and causation context remains explicit in persisted records and events.
5. Commands that can be retried require idempotency.
6. Additive changes may remain in a version; renamed or semantically changed fields require a new version.
7. `x-implementation-status` distinguishes executable operations from forward contract work during the staged build.
