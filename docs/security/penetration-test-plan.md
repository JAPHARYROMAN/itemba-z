# Independent penetration-test scope and closure standard

The assessor must be organizationally independent from the implementers and
receive written authorization, production-like targets and test-data rules.
Repository tests and automated DAST are preparation evidence, not an
independent penetration test.

Minimum scope includes OIDC/PKCE/session lifecycle, MFA bypass, token replay,
CSRF, open redirect, injection, request smuggling, authorization and object
reference abuse, cross-tenant/company/branch/warehouse access, payroll and
sensitive-finance access, maker-checker bypass, idempotency/race behavior,
mobile storage/API/offline abuse, device suspension, connector/webhook trust,
file/document handling, secrets/cloud/IaC, SSRF/egress, logging leakage,
rate/resource exhaustion and dependency/container attack surface.

Findings require reproducible evidence, affected component/data, likelihood,
impact, severity, remediation owner and retest result. Critical/high findings
block release. Closure requires assessor retest—not an engineering assertion.
Accepted lower findings require owner, due date and approved compensating
control. The final report, rules of engagement, test dates, version/commit,
scope exceptions and signed closure letter are retained in the restricted
evidence repository and referenced from the readiness register.
