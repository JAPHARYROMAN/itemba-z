# Wave 0 evidence index

- Baseline date: 2026-08-06
- Source commit: `a42e1fb5937454a1a0b7d12764d9319028747aad`
- Production-readiness baseline: 46.5/100
- Launch status: NOT AUTHORIZED

## Evidence set

| Artifact | Purpose |
| --- | --- |
| `repository-inventory.json` | Machine-generated API, web, BFF, module and migration inventory |
| `verification-report.md` | Local full-stack checks and latest GitHub CI evidence |
| `release-1-scope-register.md` | Live, partial, demonstration, planned and external scope classification |
| `production-readiness-register.md` | Owners, reviewers, dates, status and evidence for readiness domains and gates |
| `production-readiness-baseline.md` | First weighted readiness score and launch blockers |
| `ownership-and-signoff-register.md` | Required accountable and professional appointments |
| `provider-access-register.md` | Identity, hosting, TRA, bank, payment, messaging and printer dependencies |
| `source-data-register.md` | Required production migration sources, classifications and control totals |
| `organization-and-rollout-register.md` | Production organization facts, development boundary and rollout sequence |
| `decision-log.md` | Accepted Wave 0 decisions and open authorized decisions |
| `authorized-input-form.md` | Single controlled intake for the appointments and decisions required to close Wave 0 |
| `wave-0-completion-report.md` | Deliverable/exit-gate disposition and next required inputs |

## Regeneration

Regenerate the repository inventory deterministically from the repository root:

```text
node scripts/generate-wave0-inventory.mjs --as-of 2026-08-06 --source-commit a42e1fb5937454a1a0b7d12764d9319028747aad --output docs/implementation/evidence/wave-0/repository-inventory.json
```

Change the evidence date only when creating an approved new baseline. A new baseline must retain the prior evidence rather than silently rewriting history.
