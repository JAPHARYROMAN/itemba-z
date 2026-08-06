# ITEMBA-Z Control Center

The bilingual Next.js App Router workspace for Itemba Group operations. The golden-sale workspace uses a server-only BFF and a generated OpenAPI client against the live Go core API. Unimplemented modules remain clearly labelled planning previews and are never used as transactional fallbacks.

## Local development

Node.js 20.9 or newer is required.

```bash
npm ci
npm run dev
```

Open `http://localhost:3000`.

Copy `.env.example` to `.env.local` and replace the UUIDs with identities seeded by the local core API. Development identity headers are enabled only when both `ITEMBA_ENV=development` and `ITEMBA_DEV_IDENTITY_ENABLED=true`; they are never accepted in staging or production.

Production browser authentication uses provider-neutral OpenID Connect authorization code flow with PKCE, state and nonce validation. The callback validates the ID token and issues a short-lived HttpOnly access-token cookie plus an authenticated-encryption session cookie. The access token is forwarded only by the server BFF and never enters browser JavaScript. Active sessions renew through rotating refresh tokens; inactivity and absolute lifetime are enforced independently. Logout clears local credentials, requests access/refresh revocation when the provider exposes RFC 7009, and invokes provider logout when available.

Production requires the OIDC, origin and session variables documented in `.env.example`. Generate a 32-byte base64url encryption key with Node.js, then place it in the deployment secret manager rather than an environment file committed to source:

```bash
node -e "console.log(require('crypto').randomBytes(32).toString('base64url'))"
```

`ITEMBA_SESSION_ENCRYPTION_KEYS` uses `key-id:base64url-key` entries. Add a new key first, retain the old key after it during the maximum session lifetime, and then remove the old key. In production, use `__Host-` cookie names where the hosting platform supports them. OIDC discovery, client authentication and refresh/revocation calls fail closed; a refresh token too large for a safely bounded encrypted cookie is also rejected rather than truncated.

Live sales fail closed when authentication, configuration, or the core API is unavailable. The interface explicitly reports that state and never substitutes demonstration sales.

Before a sale or reversal command is sent, its payload fingerprint and idempotency key are preserved in tab-scoped session storage. Each marker is namespaced by authenticated actor, tenant, company, branch, and warehouse; reversal markers also include the source sale ID. A lost-response retry restores the exact command and key only in that same authorized scope, and the marker is cleared only after a confirmed successful response. Records older than the review threshold remain recoverable and are flagged for reconciliation rather than silently discarded.

## OpenAPI contract types

Generated TypeScript types are checked in at `src/generated/itemba-z.v1.ts`. Regenerate them after an approved contract change from this directory:

```bash
npm run generate:api
```

The command uses `openapi-typescript` against `../../contracts/openapi/itemba-z.v1.yaml`. Application code wraps the generated schemas in `src/live-api/types.ts`; backend credentials and identity resolution remain in the server-only repository.

## Production build

```bash
npm ci
npm run build
```

The app uses Next.js standalone output. Run the production artifact from the app directory:

```bash
HOSTNAME=0.0.0.0 PORT=3000 node .next/standalone/server.js
```

In PowerShell, set `$env:HOSTNAME = "0.0.0.0"` and `$env:PORT = "3000"` before the `node` command.

## Container build and run

```bash
docker build -t itemba-z-control-center .
docker run --rm --name itemba-z-control-center -p 3000:3000 itemba-z-control-center
```

The image defaults to `ITEMBA_ENV=production`. A local container may opt into development identity only by explicitly setting `ITEMBA_ENV=development` plus all `ITEMBA_DEV_*` variables; `NODE_ENV` remains `production` for the optimized Next.js runtime.

The multi-stage image runs as an unprivileged user, copies standalone and static assets, and safely creates an empty `public/` directory when the app has no public assets. Container health checks use `GET /api/health`.

## Verification

```bash
npm run lint
npm run typecheck
npm test
npm run build
```
