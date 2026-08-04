# ITEMBA-Z

ITEMBA-Z is a bilingual, tenant-ready Super ERP for multi-company and multi-branch businesses. It combines a web Control Center, an Android-first Sales POS, and one governed business platform built around immutable ledgers, strict scope controls, and traceable transactions.

## Repository map

```text
apps/
  control-center/       Next.js administrator ERP
  sales-mobile/         Flutter sales-attendant POS
services/
  core-api/             Go modular business platform and outbox worker
contracts/
  openapi/              Versioned REST contract
  events/               Versioned domain-event envelope
docs/                   Architecture, product, security, migration, and release controls
infra/                  Local dependencies, observability, and deployment foundations
```

## Architectural invariants

- A tenant/group contains legal companies; each legal company owns separate books and inventory.
- Stock, customer, supplier, and general-ledger balances are derived from append-only entries.
- Posted transactions are corrected by linked reversal, never silent mutation.
- A completed business transaction commits its operational, inventory, financial, audit, and outbox effects atomically.
- Every request is authorized by tenant, legal company, branch, warehouse, role, and contextual limits.
- Mobile retries are idempotent using `device_id + client_transaction_id`.
- AI and external integrations may fail without blocking the core transaction engine.

## Initial developer workflow

1. Copy each checked-in `.env.example` to its local `.env` equivalent.
2. Start PostgreSQL, Redis, object storage, and telemetry from `infra/docker`.
3. Run the Go API, then the Next.js Control Center and Flutter POS using their local READMEs.
4. Run the backend, web, mobile, contract, and infrastructure checks before opening a pull request.

The first production release is intentionally stage-gated. A working screen is not complete until permissions, audit, posting, reconciliation, failure handling, documentation, and acceptance tests pass.

## Current milestone

This repository currently implements the executable platform foundation and the golden sales transaction slice: a bilingual Control Center prototype, an offline-capable Android POS foundation, the Go sales posting/reversal core, PostgreSQL persistence, verified-scope authentication, transactional outbox processing, contracts, infrastructure, and automated checks.

It is not yet the complete production Release 1 ERP. Purchasing, supplier operations, full inventory workflows, complete finance, HR/payroll, statutory integrations, production migrations, and the formal release gates remain tracked in [the implementation status](docs/implementation/current-status.md).
