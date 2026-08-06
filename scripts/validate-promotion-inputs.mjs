import { readFileSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const root = new URL("../", import.meta.url);
const environmentContract = JSON.parse(readFileSync(new URL("infra/environments/control-set.json", root), "utf8"));
const fail = (message) => { throw new Error(`Promotion preflight rejected: ${message}`); };

function validateIdentifier(name, value) {
  if (!/^[A-Za-z0-9][A-Za-z0-9._:/#-]{5,159}$/.test(value ?? "")) fail(`${name} must be a durable, non-secret evidence identifier`);
}

function validate(input) {
  const target = environmentContract.environments.find((candidate) => candidate.name === input.targetEnvironment);
  if (!target) fail("target environment is not governed");
  const expectedSource = target.predecessor ?? "none";
  if (input.sourceEnvironment !== expectedSource) fail(`${input.targetEnvironment} must be promoted from ${expectedSource}`);
  if (!/^[0-9a-f]{40}$/.test(input.releaseCommit ?? "")) fail("release commit must be a full lowercase Git SHA");
  if (input.checkedOutCommit && input.checkedOutCommit !== input.releaseCommit) fail("checked-out commit does not match the authorized release commit");
  for (const [name, digest] of [["release bundle", input.releaseDigest], ["migration bundle", input.migrationDigest]]) {
    if (!/^[0-9a-f]{64}$/.test(digest ?? "")) fail(`${name} digest must be a lowercase SHA-256`);
  }
  validateIdentifier("predecessor evidence", input.predecessorEvidence);
  validateIdentifier("change ticket", input.changeTicket);
  validateIdentifier("rollback plan", input.rollbackPlan);
  return target;
}

if (process.argv.includes("--self-test")) {
  const sha = "a".repeat(40);
  const digest = "b".repeat(64);
  validate({ targetEnvironment: "staging", sourceEnvironment: "test", releaseCommit: sha, checkedOutCommit: sha, releaseDigest: digest, migrationDigest: digest, predecessorEvidence: "TEST-RUN-100", changeTicket: "CHANGE-100", rollbackPlan: "ROLLBACK-100" });
  try {
    validate({ targetEnvironment: "production", sourceEnvironment: "staging", releaseCommit: sha, checkedOutCommit: sha, releaseDigest: digest, migrationDigest: digest, predecessorEvidence: "TEST-RUN-100", changeTicket: "CHANGE-100", rollbackPlan: "ROLLBACK-100" });
    fail("self-test accepted a skipped pilot environment");
  } catch (error) {
    if (!String(error).includes("must be promoted from pilot")) throw error;
  }
  console.log("Promotion preflight validation self-test passed.");
  process.exit(0);
}

const input = {
  targetEnvironment: process.env.TARGET_ENVIRONMENT,
  sourceEnvironment: process.env.SOURCE_ENVIRONMENT,
  releaseCommit: process.env.RELEASE_COMMIT,
  checkedOutCommit: execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
  releaseDigest: process.env.RELEASE_BUNDLE_SHA256,
  migrationDigest: process.env.MIGRATION_BUNDLE_SHA256,
  predecessorEvidence: process.env.PREDECESSOR_EVIDENCE,
  changeTicket: process.env.CHANGE_TICKET,
  rollbackPlan: process.env.ROLLBACK_PLAN
};
validate(input);

if (!process.env.PROMOTION_RECORD_PATH) fail("PROMOTION_RECORD_PATH is required");
const record = {
  schema_version: 1,
  status: "PROTECTED_ENVIRONMENT_AUTHORIZATION_PENDING",
  target_environment: input.targetEnvironment,
  source_environment: input.sourceEnvironment,
  release_commit: input.releaseCommit,
  release_bundle_sha256: input.releaseDigest,
  migration_bundle_sha256: input.migrationDigest,
  predecessor_evidence: input.predecessorEvidence,
  change_ticket: input.changeTicket,
  rollback_plan: input.rollbackPlan,
  workflow_run_id: process.env.WORKFLOW_RUN_ID,
  workflow_actor: process.env.WORKFLOW_ACTOR,
  recorded_at: new Date().toISOString(),
  provider_deployment_status: "BLOCKED_UNTIL_APPROVED_ADAPTER_IS_CONFIGURED"
};
writeFileSync(process.env.PROMOTION_RECORD_PATH, `${JSON.stringify(record, null, 2)}\n`, { flag: "wx", mode: 0o600 });
console.log(`Promotion preflight accepted for ${input.targetEnvironment}; provider deployment remains fail-closed.`);
