# Wave 5 migration and qualification runbook

## Security boundary

Run `itemba-migrationctl` only from an approved migration runner using the
database-owner migration credential and a restricted encrypted source volume.
The API and worker roles have no access to staging tables. Never place real
source files, manifests containing personal data, database URLs or approval
evidence in Git, build logs or general CI artifacts.

## Canonical source envelope

Convert approved source extracts to UTF-8 NDJSON. Each line contains only:

```json
{"source_key":"opaque-source-id","tenant_key":"approved-tenant-key","company_key":"approved-company-key","record":{}}
```

The signed manifest records every file UUID, domain category, exact SHA-256,
byte count, row count, classification, owner, mapping version and cutoff. A
production trial must include every required category; synthetic owners and
incomplete category sets are rejected.

## Trial procedure

1. Freeze the approved extracts in restricted storage and independently record their hashes.
2. Run `itemba-migrationctl validate --manifest <path> --source-root <restricted-directory>`.
3. Review the non-secret validation report and sign its manifest/evidence hashes.
4. Run `itemba-migrationctl stage` with the same paths. The tool re-reads and re-hashes every source inside a serializable transaction.
5. Execute the mapping-version-specific importer and destination validations. Source-specific adapters cannot be approved before real source assessment.
6. Retain validation evidence, then transition `STAGED` to `VALIDATED` using the signed evidence hash.
7. Record maker-checker approval and transition `VALIDATED` to `APPROVED`.
8. Apply through the approved importer and transition `APPROVED` to `APPLIED` with retained execution evidence.
9. Produce the ten-domain reconciliation JSON and run `itemba-migrationctl reconcile --file <path>`.
10. If any value differs, the batch remains applied and cannot be repaired in place. Correct the source/mapping and create a new batch.

Final cutover cannot validate until two distinct production batches—Trial 1 and Trial 2—using
the same mapping version have reached `RECONCILED`.

## Qualification procedure

Use `qualification-matrix.json` as the minimum suite register. Link each run to
the exact release commit, environment, configuration version, provider sandbox,
device/printer, tester identity, start/end time, result, defect IDs and immutable
artifact hashes. Engineering automation is baseline evidence only; it does not
replace independent UAT, penetration testing, professional payroll/tax review or
accountable release approval.

## Abort conditions

Abort and preserve evidence for a checksum mismatch, undeclared file, unexpected
row, invalid scope, control-total variance, unbalanced trial balance, subledger
variance, stock variance, missing approver, critical/high defect or lost audit
correlation. Never edit a posted destination record to make reconciliation pass.
