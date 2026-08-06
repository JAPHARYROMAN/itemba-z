# Wave 2 execution record

Updated: 2026-08-06

Wave 2 establishes production identity, security and privacy controls. Repository
completion is not production authorization: provider selection, owner approvals,
MFA enforcement and independent assurance require controlled external evidence.

Repository status: **COMPLETE**

Wave 2 exit gate: **BLOCKED BY EXTERNAL EVIDENCE**

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

## Slice 2 — Access lifecycle and segregation of duties

Status: repository implementation complete; owner assignment review open

- Migration `000031_access_governance` adds validity, expiry, revocation,
  delegation, emergency-access and review evidence to scoped role assignments.
- API authorization rejects inactive users and assignments that are not yet
  valid, expired or revoked. The PostgreSQL acceptance suite proves an
  assignment loses access immediately after governed revocation.
- Governed changes require different actor and approver identities, 8–500
  character reason, ticket reference and immutable correlation evidence.
- Delegation is capped at 30 days and break-glass access at two hours by both
  command validation and database constraints.
- `cmd/accessctl` provides protected joiner, mover/leaver, delegation,
  break-glass, revocation and review operations through a separate
  administrative database secret; it is not part of the API runtime.
- The access-governance procedure defines JML, periodic review, emergency
  response and business-process SoD expectations.

External gate retained:

- Named production users, roles, scopes, amount thresholds, exceptions,
  reviewers and emergency members still require business/security owner
  approval. Seed roles are not production evidence.

## Slice 3 — Secret, certificate and key custody contract

Status: provider-neutral repository contract complete; provider resources open

- Terraform requires non-secret secret-manager, KMS, certificate-manager,
  immutable audit destination, rotation owner and emergency-group identifiers.
- Runtime inputs must be secret-reference URIs and cannot embed credential
  values. Workload identity resolves references in the selected provider
  module; Terraform variables/state are not secret stores.
- The custody procedure covers creation, dual approval, access logging,
  overlap rotation, revocation, session key rings, Android signing and
  break-glass evidence.

External gate retained:

- Hosting and secret provider selection, real resource IDs, access/rotation
  logs, certificate custody and owner sign-off remain unavailable.

## Slice 4 — Secure software supply chain

Status: repository workflows complete; protected-run evidence open

- The security workflow blocks high/critical dependency, repository, IaC and
  container findings; scans full Git history for secrets; runs extended CodeQL
  and an isolated OpenAPI DAST boundary scan; and retains redacted SARIF.
- Dependabot covers Go, npm, Dart and GitHub Actions dependencies.
- API/worker/web release jobs generate hashes, SPDX JSON SBOM, controlled
  bundles and GitHub/Sigstore-backed provenance and SBOM attestations.
- Android release jobs use protected signing material and attest signed APK/AAB
  artifacts plus their SBOM.
- `actionlint`, Terraform validation and the machine-readable Wave 2 control
  register are reproducible gates.

External gate retained:

- Protected-branch activation and the first retained scheduled/release run must
  be reviewed. A workflow definition is not a completed production release.

## Slice 5 — Privacy, incident and independent assurance controls

Status: repository control set complete; legal/owner/assessor approvals open

- The data inventory classifies identity, HR/payroll, customer/credit,
  supplier, POS/device, finance/tax/audit, communications, telemetry,
  documents and backups by subject, purpose, proposed lawful basis, transfer
  and retention trigger.
- Migration `000032_privacy_governance` provides dual-approved,
  effective-dated retention rules, legal holds, rights cases/events, disposal
  manifests and processor versions without enabling unapproved deletion.
- Procedures cover bilingual notice content, data-subject rights, correction
  without ledger rewriting, legal hold/release, verified disposal, processor
  and transfer assessment, DPIA, personal-data incident response and evidence.
- The independent penetration-test plan defines target scope, authorization,
  findings, retest and zero-critical/high closure requirements.

External gate retained:

- DPO/privacy/legal owners are unassigned; lawful bases, exact retention,
  notices, processor agreements, hosting/support transfers and DPIA are not
  approved. No independent penetration test or closure letter exists.

## Wave 2 repository exit

- All provider-neutral controls that can be implemented safely before owner and
  provider selection are executable and machine-validated.
- The repository never labels external approvals, provider resources or an
  independent assessment complete.
- Production authentication/security/privacy exit remains blocked until the
  external evidence in `evidence/wave-2/README.md` is supplied and approved.
