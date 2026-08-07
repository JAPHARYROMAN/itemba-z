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

## Integrated golden-sale environment

The optional `application` Compose profile runs the executable sales slice with
PostgreSQL migrations, an idempotent development seed, the Go API, local outbox
worker, and the Next.js Control Center:

```text
Copy-Item infra/docker/.env.example infra/docker/.env
docker compose --env-file infra/docker/.env -f infra/docker/compose.yaml --profile application up --build
```

Open `http://localhost:3000/sales`. The development seed uses stable, fictional
master data and a zero-rated `DEV_ZERO` tax rule; it is not statutory Tanzania
configuration and the seed command runs only with `ITEMBA_ENV=development`.
The API is available on `http://localhost:8080` for an Android emulator or an
HTTP smoke test. Production deployments require OIDC and never accept the local
development identity headers.

The transaction boundary and executable acceptance scenarios are documented in
[`docs/implementation/live-golden-sale.md`](docs/implementation/live-golden-sale.md).

The first production release is intentionally stage-gated. A working screen is not complete until permissions, audit, posting, reconciliation, failure handling, documentation, and acceptance tests pass.

## Current milestone

This repository currently implements the executable platform foundation plus live sales, customer receivables, commercial, procure-to-pay, inventory, banking reconciliation, governed finance, budgets, fixed assets, treasury facilities, and legal-company financial reporting slices. Trial balance, general ledger, profit and loss, balance sheet, cash flow, budget-versus-actual, drill-down, and audited CSV export are derived from immutable journals. The bilingual Control Center and encrypted Android POS use versioned live APIs; PostgreSQL persistence, verified-scope authentication, transactional outbox processing, generated bindings, and database-backed checks protect the implemented workflows. Unimplemented modules remain clearly separated demonstrations until their live APIs exist.

Core ERP functional coverage is tracked at **94.1%** in [the coverage assessment](docs/implementation/core-functional-coverage.md). This is not production readiness: statutory integrations, production migrations, infrastructure evidence, professional validation, and the formal release gates remain tracked in [the implementation status](docs/implementation/current-status.md). The evidence-based path from the current system to an authorized Release 1 launch is defined in the [production-readiness roadmap](docs/implementation/roadmap-to-production.md), with the first **46.5/100** readiness baseline and Wave 0 registers retained in the [Wave 0 evidence index](docs/implementation/evidence/wave-0/README.md). Wave 1 execution remains separately tracked in its [execution record](docs/implementation/wave-1-execution.md). Wave 5 repository controls—including checksum-bound migration staging and qualification evidence—are complete, while its real trials, UAT, payroll, security and approval exit remains explicitly external-blocked in the [Wave 5 evidence index](docs/implementation/evidence/wave-5/README.md).
