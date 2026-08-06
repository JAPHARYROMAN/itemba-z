# Secrets, certificate and key custody

## Rules

Production secrets exist only in the approved managed secret service. Git,
GitHub variables, Terraform state/variables, container layers, `.env` files,
support tickets, chat and user workstations are not secret stores. Workloads
retrieve secret references through least-privilege workload identity; routine
operators cannot read secret values.

The custody register covers database credentials, OIDC client credentials,
session-encryption key rings, object-storage keys, connector credentials,
Android signing keys, TLS/private certificates, webhook verification keys and
backup encryption keys. Each entry requires an owner, approver, purpose,
environment, consuming workload, vault reference, creation/rotation date,
rotation interval, recovery authority and revocation procedure.

## Rotation and access evidence

1. A different approver authorizes creation or rotation.
2. The managed service generates key material where supported; exportable
   private material is prohibited unless the provider requires it.
3. A workload-identity policy grants only the named workload and version.
4. Rotation uses overlap where the protocol permits it, validates the new key,
   then revokes the old key within the approved window.
5. Read, write, rotation, policy and emergency-access events flow to an
   immutable security log with approved retention and alerting.
6. Failed rotations roll back references, not committed ERP transactions.

Session key rings add the new write key first, retain decrypt-only predecessors
for no longer than the maximum session lifetime, and then remove them. Android
signing material is exposed only to the protected release job. CI log masking
is defense in depth and never authorizes echoing a secret.

## Break glass

Emergency retrieval requires the incident ticket, two named people, fresh MFA,
a time-bound emergency group and immediate alerting. Evidence records who,
what reference, why, when, approval, actions taken and revocation. The actual
secret value is never copied into evidence. Use triggers a mandatory rotation
unless the security owner records why rotation would increase risk.

The Terraform root accepts only secret-manager identifiers and reference URIs.
The selected provider module and owner-approved resource IDs remain external
Wave 2 evidence; placeholder identifiers are not production custody.
