# Wave 4 evidence

The first Wave 4 repository slice is implemented, while the production exit is
explicitly blocked. `control-set.json` separates version-controlled controls
from evidence that can exist only after provider selection, protected
environment configuration and production-like exercises.

Run `node scripts/validate-wave4-controls.mjs` to validate the environment
order, fail-closed Terraform contract, promotion input policy, SLO and alert
coverage, runbook mapping, recovery plan and external-blocked claims.
