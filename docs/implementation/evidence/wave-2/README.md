# Wave 2 evidence index

Repository evidence is reproducible; external approvals remain explicitly
blocked until supplied by authorized owners.

| Evidence | Repository status | External closure required |
| --- | --- | --- |
| OIDC/session/MFA assurance | Implemented and automated | Selected IdP, claims/MFA configuration and owner test sign-off |
| JML/delegation/break glass/access review | Schema, controlled command and revocation acceptance implemented | Named role owners, assignment register and completed owner review |
| Secret/key/certificate custody | Terraform reference contract and procedure implemented | Selected managed service, resource IDs, access/rotation logs and owner approval |
| SAST/dependency/secret/IaC/container/DAST | Blocking workflows and retained-report jobs implemented | Protected branch activation and first successful retained production-like run |
| SBOM/provenance | API, worker, web and Android release jobs implemented | Protected release invocation and verified attestation bundle |
| Privacy/data governance | Inventory, classification, rights, retention, processor, transfer and DPIA control set drafted | DPO/legal decisions, provider register, approved notices/schedules/DPIA |
| Incident response | Procedure and exercise scenarios defined | Named responders and completed tabletop evidence |
| Penetration testing | Scope and closure standard defined | Independent test and critical/high closure letter |

`control-set.json` is validated in CI by
`node scripts/validate-wave2-controls.mjs`. It prevents missing evidence or a
repository-only control from being mislabeled as externally approved.
