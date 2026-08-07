import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const required = (name) => {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
};

const evidencePath = required("EVIDENCE_PATH");
const runnerTemp = required("RUNNER_TEMP");
const source = readFileSync(join(runnerTemp, "source-manifest.json"));
const restored = readFileSync(join(runnerTemp, "restored-manifest.json"));
if (!source.equals(restored)) throw new Error("restored manifest does not match the source recovery point");
const record = {
  schema_version: 1,
  exercise_id: required("EXERCISE_ID"),
  release_commit: required("RELEASE_COMMIT"),
  status: "PASSED_ISOLATED_REPOSITORY_DRILL",
  source_manifest_sha256: createHash("sha256").update(source).digest("hex"),
  restored_manifest_sha256: createHash("sha256").update(restored).digest("hex"),
  authoritative_scope: ["journals", "stock", "subledgers", "audit", "outbox", "integrations", "document_metadata"],
  limitations: ["ephemeral_ci_database", "no_managed_provider_pitr", "no_object_blob_restore", "no_approved_rto_rpo"],
  recorded_at: new Date().toISOString()
};
writeFileSync(evidencePath, `${JSON.stringify(record, null, 2)}\n`, { flag: "wx", mode: 0o600 });
console.log("Isolated recovery evidence recorded; managed-provider recovery remains external-blocked.");
