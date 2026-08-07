# ITEMBA-Z Core API

The core API is a Go modular monolith. PostgreSQL is authoritative in deployed
environments; the in-memory adapter exists for deterministic tests and explicit
local development only.

## Database and migrations

Set `DATABASE_URL` to a PostgreSQL connection string, then run migrations from
this directory:

```text
go run ./cmd/migrate
```

The migration runner applies every `migrations/*.up.sql` file in lexical order,
records its SHA-256 checksum in `public.itembaz_schema_migrations`, and refuses
to run if an already-applied migration was changed. Set
`ITEMBA_MIGRATIONS_DIR` only when invoking the command from another directory.

Migrations and seed data use an administrative identity, but application
processes must not. Provision distinct least-privilege logins with
`ITEMBA_RUNTIME_CAPABILITY=api` and `ITEMBA_RUNTIME_CAPABILITY=worker`; the API
login cannot enumerate tenants or lease/mark outbox deliveries, while the
worker can discover only tenants with claimable events and cannot read business
tables. `cmd/api` and `cmd/worker` reject the other process's capability role.

After applying migrations with the administrative URL, provision both runtime
logins with the same administrative `DATABASE_URL`:

```text
ITEMBA_RUNTIME_USER=itemba_z_api_runtime ITEMBA_RUNTIME_PASSWORD=<strong-password> ITEMBA_RUNTIME_CAPABILITY=api go run ./cmd/devdbrole
ITEMBA_RUNTIME_USER=itemba_z_worker_runtime ITEMBA_RUNTIME_PASSWORD=<strong-password> ITEMBA_RUNTIME_CAPABILITY=worker go run ./cmd/devdbrole
```

For the local golden-sale flow, migrate first and then install the idempotent
development fixture:

```text
ITEMBA_ENV=development DATABASE_URL=<postgres-url> go run ./cmd/devseed
```

Then start each process with its own restricted connection URL:

```text
ITEMBA_ENV=development DATABASE_URL=<api-runtime-url> go run ./cmd/api
ITEMBA_ENV=development DATABASE_URL=<worker-runtime-url> go run ./cmd/worker
```

The API exposes Prometheus metrics only on the separate
`ITEMBA_METRICS_ADDRESS` listener. This address is mandatory outside explicit
development and must remain on the private workload network. HTTP labels use
route templates rather than raw URLs and never include tenant, actor, document
or payload data. Production runtimes use the redacting structured logger; the
managed telemetry destination is configured through the production collector
contract in `infra/observability`.

`devseed` accepts only `development`, `dev`, `local`, or `test` and refuses all
other environments. It uses the fictional zero-rated `DEV_ZERO` tax rule; it
is not statutory configuration. Stable fixture IDs are:

| Record | UUID |
| --- | --- |
| Tenant | `00000000-0000-4000-8000-000000000001` |
| Legal company | `00000000-0000-4000-8000-000000000002` |
| Branch | `00000000-0000-4000-8000-000000000003` |
| Warehouse | `00000000-0000-4000-8000-000000000004` |
| Operator | `00000000-0000-4000-8000-000000000005` |
| Role | `00000000-0000-4000-8000-000000000006` |
| General Customer | `00000000-0000-4000-8000-000000000007` |
| Credit customer | `00000000-0000-4000-8000-000000000008` |
| Product 1 | `00000000-0000-4000-8000-000000000009` |
| Product 2 | `00000000-0000-4000-8000-000000000010` |

The fixture's fictional posting policy maps every canonical settlement method
to a distinct account: `CASH` to `cash-on-hand`, `MOBILE_MONEY` to
`mobile-money-clearing`, `BANK_CARD` to `bank-card-clearing`, and
`BANK_TRANSFER` to `bank-current`. Unknown methods, or canonical methods absent
from a company's posting configuration, return a 422 business-rule response.

Use `00000000-0000-4000-8000-000000000011` as the stable smoke-test device
ID. Development HTTP authentication uses the tenant, company, branch,
warehouse, and operator IDs above in `X-Tenant-ID`, `X-Company-ID`,
`X-Branch-ID`, `X-Warehouse-ID`, and `X-Actor-ID` respectively.

The fixture device is intentionally `ACTIVE` with fictional, development-only
offline cash policy: TZS 20,000,000 minor units per transaction, TZS 50,000,000
minor units per local business day, and 25 units allocated to each seeded
product. Re-running `devseed` restores those configured policy and allocation
values without duplicating ledger stock or previously synchronized sales.

Exactly 100 stored minor units equal TZS 1. Public integer amounts, quantities,
counts, and versions are capped at JavaScript's exact integer maximum
(`9007199254740991`); out-of-range commands fail with 422 and no committed
effects. PostgreSQL remains bigint-backed.

The offline milestone is deliberately narrower than the future statutory POS:
only physical `CASH` sales whose authoritative effective tax rate is zero may
synchronize as offline. Non-cash or nonzero-tax offline commands fail with 422.
`DEV_ZERO` exists only to exercise this path and is not Tanzanian statutory
configuration. Governed effective-time tax snapshots and TRA fiscalization are
still required before statutory offline sales can be claimed.

Device enrollment uses an install acknowledgement. A new device reports
installed `master_data_version` and `price_version` as zero plus separate
`available_*` versions. After downloading and atomically persisting the cache,
the client repeats enrollment with both `installed_*` fields. The server
advances installed state only when both equal the current company versions;
stale acknowledgements fail with 422, and retrying after a lost response is
idempotent. For an existing device, `app_version` advances in that same
acknowledgement only; an unacknowledged application upgrade cannot split the
response from durable server state. Every state-changing acknowledgement adds
one immutable audit record and transactional outbox event with the final device
state; an exact retry under the still-live lease is a pure read.

An offline-enabled device receives an exclusive server-persisted
`offline_sales_valid_until` lease only after successful acknowledgement. A
lease is bound to exact scope, device, application, master-data version, and
price version, lasts at most four hours, and stops at the next configured tax
transition. Lease history is append-only, so acknowledging a newer cache
preserves the authorization proof for a transaction created under an older
valid lease. That does not guarantee later posting: governed product, price, or
tax drift can still fail closed and require reconciliation until immutable
as-of snapshots exist. Offline daily limits use the lease-validated client
creation timestamp in the legal company timezone, rather than the later
synchronization timestamp. Tax-rule writes are serialized with acknowledgement
and cannot overlap an outstanding lease; product master/price drift beyond the
leased cache fails closed.

This lease is still not a production statutory snapshot. Product-list download
and acknowledgement do not yet share an immutable content token/fingerprint,
so a download can straddle a scheduled rule's activation while retaining the
same company version. Production offline enablement therefore remains blocked
on an as-of snapshot/token (or activation-time version publication), governed
historical cache data, and professional TRA validation.

## Credit safety boundary

The server blocks General Customer credit and evaluates registered customers
against an append-only, effective-dated policy containing limit, payment terms,
overdue tolerance, and risk state. Invoice-level receivable items and allocations
must reconcile to the append-only customer ledger. Credit completion creates a
due-dated invoice in the same transaction; reversal creates and allocates a linked
credit note. Online mobile synchronization uses this same gate. Offline credit is
always rejected, and clients must never infer that a missing overdue amount is zero.

Only the exact `ITEMBA_ENV=development` value permits the in-memory repository
or development header authenticator. Unset, misspelled, staging, production,
and every other environment require `DATABASE_URL`, `OIDC_ISSUER_URL`,
`OIDC_AUDIENCE`, `OIDC_REQUIRED_ACR`, `OIDC_REQUIRED_AMR`, and
`OIDC_MAX_AUTH_AGE_SECONDS` and fail closed when any are absent or invalid.
Production access tokens must include the configured assurance class, every
required authentication-method reference, a recent `auth_time`, a UUID-valued
`user_id` claim plus tenant, company, branch, and warehouse UUID claims. Opaque
OIDC `sub` values are retained as provenance and are never database user IDs.

Approved identity lifecycle changes run through `go run ./cmd/accessctl` in a
protected administrative job. It requires a managed
`ITEMBA_ADMIN_DATABASE_URL`, separate actor and approver IDs, a reason, and a
ticket reference. Standard grants, time-bounded delegation, two-hour maximum
break-glass access, revocation, user disablement and access-review attestations
write immutable evidence. The API runtime identity cannot execute this tool's
administrative SQL.

## Outbox worker

Run `go run ./cmd/worker` with `DATABASE_URL` configured. The local worker uses
an at-least-once logging publisher and safe leases (`FOR UPDATE SKIP LOCKED`). It
refuses production mode until a durable broker publisher is provided through
the `outbox.Publisher` boundary. Consumers must remain idempotent because a
process crash after publish and before acknowledgement can redeliver an event.

## Verification

```text
go vet ./...
go test ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
```

PostgreSQL integration tests create and remove a uniquely named schema and skip
cleanly unless `TEST_DATABASE_URL` points to a disposable test database.

## Container image

Build the non-root production image with `docker build -t itemba-z-core-api .`.
The default entrypoint runs `/app/api`; deployment jobs run migrations with
`--entrypoint /app/migrate`, local fixture setup uses `--entrypoint /app/devseed`, and the outbox deployment uses
`--entrypoint /app/worker` after a production `outbox.Publisher` is supplied.
