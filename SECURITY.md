# Security policy

ITEMBA-Z is pre-release software. No version is approved for production use until the security, recovery, compliance, migration, and reconciliation gates in `docs/implementation/release-gates.md` have passed.

## Reporting a vulnerability

Do not disclose suspected vulnerabilities, credentials, personal data, or production records in a public issue. Use the repository's private vulnerability-reporting flow under the GitHub **Security** tab. Repository administrators must enable that channel before granting external access.

Include the affected component and version, reproduction steps, likely impact, and any suggested mitigation. Maintainers should acknowledge the report privately, preserve evidence, assign severity and ownership, remediate on a protected branch, rotate exposed secrets, and disclose only after a coordinated fix.

## Repository controls

- Never commit secrets, signing keys, customer exports, payroll records, fiscal credentials, or real production data.
- Treat tenant, finance, payroll, identity, receipt, and integration logs as sensitive.
- Keep dependency and toolchain vulnerability checks green; do not suppress findings without a dated, owned risk acceptance.
- Posted business records remain immutable and corrections use linked reversals.
- Production access must use verified OIDC claims, least privilege, scoped database roles, encrypted transport and storage, auditable administration, and protected deployment credentials.
