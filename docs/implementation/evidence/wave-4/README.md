# Wave 4 evidence

The repository-controlled Wave 4 implementation is complete, while the
production exit is explicitly blocked. `control-set.json` separates executable
version-controlled controls from evidence that can exist only after provider
selection, protected environment configuration, production-like exercises and
accountable operational approval.

Run `node scripts/validate-wave4-controls.mjs` to validate the environment
order, fail-closed Terraform contract, promotion input policy, telemetry and
redaction code, SLO/alert coverage, runbook mapping, recovery/rollback/load
exercises, provider-adapter contract and external-blocked claims.
