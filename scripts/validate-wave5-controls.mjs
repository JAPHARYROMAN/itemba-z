import { createHash } from "node:crypto";
import { existsSync, readFileSync, statSync } from "node:fs";

const read = (path) => readFileSync(path, "utf8");
const json = (path) => JSON.parse(read(path));
const fail = (message) => { throw new Error(`Wave 5 evidence invalid: ${message}`); };
const sha256 = (path) => createHash("sha256").update(readFileSync(path)).digest("hex");

const controls = json("docs/implementation/evidence/wave-5/control-set.json");
if (controls.repository_status !== "COMPLETE" || controls.release_exit_status !== "BLOCKED_EXTERNAL") {
  fail("status must distinguish repository completion from production acceptance");
}
if (!Array.isArray(controls.repository_controls) || controls.repository_controls.length < 9) fail("repository controls are incomplete");
for (const control of controls.repository_controls) {
  if (control.status !== "IMPLEMENTED") fail(`${control.id} is not implemented`);
  for (const path of control.evidence ?? []) if (!existsSync(path)) fail(`${control.id} evidence is missing: ${path}`);
}
if (!Array.isArray(controls.external_gates) || controls.external_gates.length < 9) fail("external gates are incomplete");
for (const gate of controls.external_gates) {
  if (gate.status !== "BLOCKED_EXTERNAL" || gate.required !== true || !gate.owner || !gate.evidence_required) fail(`${gate.id} must remain owned and external-blocked`);
}

const matrix = json("docs/implementation/evidence/wave-5/qualification-matrix.json");
if (!Array.isArray(matrix.suites) || matrix.suites.length !== 10) fail("qualification matrix must contain all ten suites");
const suiteIds = new Set();
for (const suite of matrix.suites) {
  if (suiteIds.has(suite.id)) fail(`duplicate suite ${suite.id}`);
  suiteIds.add(suite.id);
  if (!suite.repository_status || suite.release_status !== "EXTERNAL_EXECUTION_REQUIRED" || !suite.required_external_evidence) fail(`${suite.id} overstates release evidence`);
  for (const path of suite.repository_evidence ?? []) if (!existsSync(path)) fail(`${suite.id} repository evidence is missing: ${path}`);
}

const migration = read("services/core-api/migrations/000037_migration_assurance.up.sql");
for (const marker of [
  "migration_batches", "migration_source_files", "migration_stage_rows", "migration_control_totals",
  "migration_reconciliations", "migration_batch_transitions", "append-only", "migration batches cannot be deleted",
  "synthetic rehearsal cannot satisfy production migration gates", "ten passing reconciliations",
  "reconciled production trials 1 and 2", "REVOKE ALL ON TABLE"
]) if (!migration.includes(marker)) fail(`migration assurance schema is missing ${marker}`);
if (/GRANT\s+(?:SELECT|INSERT|UPDATE|DELETE|ALL)[\s\S]*migration_(?:batches|source_files|stage_rows)/i.test(migration)) fail("runtime data privilege was granted on restricted migration tables");
const searchPathFix = read("services/core-api/migrations/000038_migration_assurance_search_path.up.sql");
if (!searchPathFix.includes("SET search_path TO itembaz, pg_temp")) fail("migration lifecycle function search path is not pinned");
const sourceKeyScope = read("services/core-api/migrations/000039_migration_source_key_scope.up.sql");
if (!sourceKeyScope.includes("UNIQUE (batch_id, source_key_hash)")) fail("duplicate source keys are not rejected across same-category files");
const reconciliationRuns = read("services/core-api/migrations/000040_migration_reconciliation_runs.up.sql");
for (const marker of ["migration_reconciliation_runs", "file_sha256", "append_only", "REVOKE ALL"]) if (!reconciliationRuns.includes(marker)) fail(`reconciliation replay control is missing ${marker}`);

const manifestCode = read("services/core-api/internal/platform/migrationassurance/manifest.go");
for (const marker of ["DisallowUnknownFields", "io.TeeReader", "maxRowBytes", "maxRowsPerFile", "source_key", "signed manifest", "requiredProductionCategories"]) {
  if (!manifestCode.includes(marker)) fail(`manifest validator is missing ${marker}`);
}
const stageCode = read("services/core-api/internal/platform/migrationassurance/stage.go");
for (const marker of ["pgx.Serializable", "ON CONFLICT (id) DO NOTHING", "idempotent migration staging replay", "validateSource", "normalized_record", "transition_migration_batch"]) if (!stageCode.includes(marker)) fail(`staging transaction is missing ${marker}`);
const lifecycle = read("services/core-api/internal/platform/migrationassurance/lifecycle.go");
for (const marker of ["big.Rat", "requiredReconciliationDomains", "idempotent reconciliation replay", "migration_reconciliation_runs", "APPLIED", "RECONCILED", "CaptureBatchEvidence"]) if (!lifecycle.includes(marker)) fail(`lifecycle assurance is missing ${marker}`);

for (const trial of [1, 2]) {
  const root = `testdata/migration/wave-5/trial-${trial}`;
  const manifest = json(`${root}/manifest.json`);
  if (manifest.mode !== "SYNTHETIC_REHEARSAL" || manifest.trial_number !== trial) fail(`trial ${trial} fixture must remain synthetic`);
  if (!manifest.actor_reference.includes("qualification")) fail(`trial ${trial} fixture has no qualification actor`);
  for (const source of manifest.sources) {
    const path = `${root}/${source.file}`;
    if (!existsSync(path) || source.sha256 !== sha256(path) || source.byte_count !== statSync(path).size) fail(`trial ${trial} source does not match its checksum-bound manifest`);
    const rows = read(path).trimEnd().split("\n");
    if (rows.length !== source.expected_rows) fail(`trial ${trial} source row count differs`);
    for (const row of rows) {
      const parsed = JSON.parse(row);
      if (!parsed.source_key || !parsed.tenant_key || !parsed.company_key || typeof parsed.record !== "object") fail(`trial ${trial} has a non-canonical row`);
    }
  }
}

const workflow = read(".github/workflows/wave5-qualification.yml");
for (const marker of [
  "SYNTHETIC_REHEARSAL", "migrationctl\" validate", "migrationctl\" stage", "migrationctl\" evidence",
  "Synthetic rehearsal crossed a production migration gate", "has_table_privilege", "retention-days: 90"
]) if (!workflow.includes(marker)) fail(`qualification workflow is missing ${marker}`);
const release = read(".github/workflows/release-supply-chain.yml");
if (!release.includes("itemba-migrationctl")) fail("release bundle omits the controlled migration tool");
const ci = read(".github/workflows/ci.yml");
if (!ci.includes("validate-wave5-controls.mjs")) fail("CI omits the Wave 5 evidence validator");

const runbook = read("docs/operations/wave-5-qualification-runbook.md");
for (const marker of ["restricted encrypted source volume", "two distinct", "Abort", "Never edit a posted destination record"]) if (!runbook.includes(marker)) fail(`runbook is missing ${marker}`);
const acceptance = read("docs/operations/wave-5-acceptance-checklist.md");
for (const marker of ["[ ]", "Migration lead", "Payroll", "Security", "Executive", "release_exit_status=BLOCKED_EXTERNAL"]) if (!acceptance.includes(marker)) fail(`acceptance checklist is missing ${marker}`);

console.log(`Wave 5 repository controls verified: ${controls.repository_controls.length} implemented controls, ${matrix.suites.length} qualification suites; ${controls.external_gates.length} production gates remain external-blocked.`);
