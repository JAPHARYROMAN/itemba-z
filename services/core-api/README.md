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

Start the API with PostgreSQL:

```text
go run ./cmd/api
```

Production additionally requires `ITEMBA_ENV=production`, `OIDC_ISSUER_URL`,
and `OIDC_AUDIENCE`. In non-production environments only, omitting OIDC selects
the conspicuously named development header authenticator. Production ID tokens
must include a UUID-valued `user_id` claim plus tenant, company, branch, and
warehouse UUID claims; opaque OIDC `sub` values are retained as provenance and
are never used as database user IDs.

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
`--entrypoint /app/migrate`, and the outbox deployment uses
`--entrypoint /app/worker` after a production `outbox.Publisher` is supplied.
