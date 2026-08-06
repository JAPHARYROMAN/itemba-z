# Wave 2 execution record

Updated: 2026-08-06

Wave 2 establishes production identity, security and privacy controls. Repository
completion is not production authorization: provider selection, owner approvals,
MFA enforcement and independent assurance require controlled external evidence.

## Slice 1 — Control Center OIDC and session lifecycle

Status: repository implementation complete; provider integration gate open

Implemented evidence:

- Provider-neutral OpenID Connect discovery and confidential or public client
  authentication use a maintained protocol implementation rather than custom
  token parsing.
- Login uses authorization code flow with PKCE S256, state, nonce, exact callback
  registration, bounded transaction lifetime and safe local return paths.
- The callback validates authorization response, issuer/audience/signature, ID
  token nonce, authentication age and Bearer token lifetime before creating a
  session.
- The access token remains in a short-lived HttpOnly cookie. Refresh token and
  minimal display/session metadata are authenticated-encrypted with AES-256-GCM,
  purpose-bound associated data, a bounded cookie envelope and ordered key-ring
  rotation.
- Active browser use renews near-expiry access tokens. Inactive tabs do not keep
  a session alive. Server policy independently enforces inactivity and absolute
  lifetime, and `/api/auth/login?force=1` requests fresh provider authentication.
- Session mutation and logout require the exact configured Origin or same-origin
  Fetch Metadata. Logout clears every local auth cookie, requests token
  revocation when the provider advertises it, and invokes provider end-session
  when available.
- Invalid configuration, missing sessions, altered cookies, failed renewal,
  unsupported token types and unsafe redirects fail closed. Development identity
  remains explicitly separate and cannot activate outside `ITEMBA_ENV=development`.
- The Control Center displays the real session mode, identity and assurance hint;
  it provides working reauthentication/sign-out controls and a truthful secure
  login page instead of the former static OIDC label.

Repository verification:

- OIDC configuration tests cover HTTPS enforcement, client-auth consistency,
  timeout bounds and redirect confinement.
- Session tests cover authenticated encryption, tamper and purpose-replay
  rejection, key rotation, expiry order, refresh leeway and mutation-origin
  checks.
- Control Center lint, strict typecheck, unit/component tests and production build
  are mandatory for this slice.

External gate retained:

- `PRV-003` remains blocked until authorized owners select the production IdP,
  register exact redirect/logout URLs, supply non-production credentials, approve
  claims and MFA policy, confirm refresh/revocation/end-session behavior, execute
  cross-scope tests and sign the evidence. No provider has been inferred from
  repository code.

## Remaining Wave 2 work

- Joiner/mover/leaver, delegation, break-glass and periodic access-review control
  implementation and owner evidence.
- Business-approved role/scope/amount/maker-checker and segregation-of-duties
  assignment review.
- Production secret manager, certificate/key custody, rotation logging and
  emergency access.
- Blocking dependency/container/IaC/secret/SAST/DAST policies with retained
  reports; SBOM and signed provenance for every release artifact.
- Data inventory/classification, lawful basis, notices, retention, data-subject
  rights, processor register, legal hold/deletion, incident response and DPIA.
- Independent penetration test and closure of all critical/high findings.
