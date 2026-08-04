# ITEMBA-Z Control Center

The bilingual Next.js App Router workspace for Itemba Group operations. Its UI reads through a typed in-memory repository, keeping page contracts stable when the versioned OpenAPI client replaces the mock adapter.

## Local development

Node.js 20.9 or newer is required.

```bash
npm ci
npm run dev
```

Open `http://localhost:3000`.

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

The multi-stage image runs as an unprivileged user, copies standalone and static assets, and safely creates an empty `public/` directory when the app has no public assets. Container health checks use `GET /api/health`.

## Verification

```bash
npm run lint
npm run typecheck
npm test
npm run build
```
