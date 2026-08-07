# Wave 0 repository verification report

- Evidence date: 2026-08-06
- Git commit verified: `a42e1fb5937454a1a0b7d12764d9319028747aad`
- Branch: `codex/live-golden-sale`
- Result: PASS
- Scope: local full-stack repository verification plus latest GitHub CI result

## Backend

Commands:

```text
go test ./...
go vet ./...
govulncheck ./...
```

Result: PASS. All Go package tests and vet checks passed. `govulncheck` found no reachable vulnerabilities in ITEMBA-Z code. It reported one vulnerability in a required module that the code does not call.

## Control Center

Commands:

```text
npm audit --audit-level=high
npm run lint
npm run typecheck
npm test
npm run build
```

Result: PASS.

- npm high-severity audit: 0 vulnerabilities
- test files: 22 passed
- tests: 47 passed
- ESLint: passed
- TypeScript: passed
- Next.js 16.3.0 production build: passed
- production route compilation: passed

## Android Sales POS

Commands:

```text
dart run tool/generate_openapi_subset.dart --check
flutter analyze
flutter test
flutter build apk --debug
```

Result: PASS.

- generated contract drift check: passed
- Flutter analysis: no issues found
- tests: 59 passed
- debug APK: built successfully

The debug APK is development evidence only. A signed, provenance-controlled release artifact and managed distribution path remain production requirements.

## Contracts and infrastructure

Commands:

```text
npx --yes @redocly/cli@2.9.0 lint contracts/openapi/itemba-z.v1.yaml
node -e "JSON.parse(require('fs').readFileSync('contracts/events/event-envelope.v1.schema.json','utf8'))"
terraform fmt -check -recursive
terraform -chdir=infra/terraform init -backend=false -input=false
terraform -chdir=infra/terraform validate
docker compose --env-file infra/docker/.env.example -f infra/docker/compose.yaml config --quiet
```

Result: PASS.

- OpenAPI 3.1 contract: valid
- event envelope JSON: valid
- Terraform formatting and validation: passed
- local Docker topology: valid

## Remote CI

The latest CI run for the verified commit completed successfully:

- workflow: `CI`
- run: [GitHub Actions 31073408349](https://github.com/JAPHARYROMAN/itemba-z/actions/runs/31073408349)
- commit: `a42e1fb5937454a1a0b7d12764d9319028747aad`
- conclusion: success

## Interpretation

This report proves that the checked repository compiles and passes its current automated verification boundaries. It does not prove production readiness. Production identity, real provider integrations, production data, trial migrations, performance, penetration, restore, disaster recovery, statutory validation, UAT, pilot operation and professional sign-off remain separate release evidence.
