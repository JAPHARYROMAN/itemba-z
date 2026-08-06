# Wave 5 evidence

Wave 5 repository controls are complete. This means the repository now has an
executable, checksum-bound migration assurance pipeline, immutable staging and
lifecycle facts, exact reconciliation controls, synthetic control rehearsals,
a qualification matrix and retained-evidence workflows.

It does **not** mean production migration or Release 1 acceptance is complete.
The synthetic fixtures are deliberately prevented from progressing beyond
`STAGED`. The production exit remains `BLOCKED_EXTERNAL` until real source
files, two distinct signed production trials, payroll parallel runs, independent
UAT/security/operational exercises, defect closure and named approvals exist.

Run `node scripts/validate-wave5-controls.mjs` for the repository control gate.
