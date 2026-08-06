import { existsSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const registerPath = "docs/implementation/evidence/wave-2/control-set.json";
const register = JSON.parse(readFileSync(registerPath, "utf8"));
const requiredFamilies = new Set(["IDENTITY", "ACCESS", "SECRET_CUSTODY", "SECURE_SDLC", "PRIVACY", "INCIDENT", "PENETRATION_TEST"]);
const allowedStatuses = new Set(["REPOSITORY_IMPLEMENTED_EXTERNAL_OPEN", "EXTERNAL_BLOCKED"]);
const ids = new Set();

if (register.version !== 1 || !Array.isArray(register.controls) || register.controls.length < requiredFamilies.size) {
  throw new Error("Wave 2 control register has an invalid shape");
}
for (const control of register.controls) {
  if (!/^W2-[A-Z]{2}-\d{2}$/.test(control.id) || ids.has(control.id)) throw new Error(`Invalid or duplicate control ID: ${control.id}`);
  ids.add(control.id);
  requiredFamilies.delete(control.family);
  if (!allowedStatuses.has(control.status)) throw new Error(`${control.id} uses an unapproved status`);
  if (!control.external_owner || !String(control.external_owner).trim()) throw new Error(`${control.id} has no external owner state`);
  if (!Array.isArray(control.evidence) || control.evidence.length === 0) throw new Error(`${control.id} has no evidence`);
  for (const path of control.evidence) if (!existsSync(path)) throw new Error(`${control.id} evidence is missing: ${path}`);
}
if (requiredFamilies.size) throw new Error(`Wave 2 control families are missing: ${[...requiredFamilies].join(", ")}`);

const securityWorkflow = readFileSync(".github/workflows/security.yml", "utf8");
for (const marker of ["dependency-review-action", "codeql-action", "gitleaks", "trivy-action", "sbom-action", "action-api-scan", "exit-code: \"1\""]) {
  if (!securityWorkflow.includes(marker)) throw new Error(`Security workflow is missing blocking control: ${marker}`);
}
const releaseWorkflow = readFileSync(".github/workflows/release-supply-chain.yml", "utf8");
for (const marker of ["actions/attest@v4", "itemba-api", "itemba-worker", "itemba-control-center", "itemba-accessctl", "spdx-json"]) {
  if (!releaseWorkflow.includes(marker)) throw new Error(`Release workflow is missing supply-chain evidence: ${marker}`);
}

const tracked = execFileSync("git", ["ls-files"], { encoding: "utf8" }).split(/\r?\n/).filter(Boolean);
for (const path of tracked) {
  if (/(^|\/)\.env($|\.)/.test(path) && !path.endsWith(".env.example")) throw new Error(`Tracked environment file is prohibited: ${path}`);
  if (/\.(pem|p12|pfx|jks|keystore)$/i.test(path)) throw new Error(`Tracked key material is prohibited: ${path}`);
}

const accessMigration = readFileSync("services/core-api/migrations/000031_access_governance.up.sql", "utf8");
for (const marker of ["valid_until", "revoked_at", "BREAK_GLASS", "ACCESS_REVIEW_ATTESTED", "actor_id <> approver_id"]) {
  if (!accessMigration.includes(marker)) throw new Error(`Access governance migration is missing: ${marker}`);
}
const privacyMigration = readFileSync("services/core-api/migrations/000032_privacy_governance.up.sql", "utf8");
for (const marker of ["privacy_retention_policies", "privacy_legal_holds", "privacy_cases", "privacy_disposal_manifests", "privacy_processor_versions"]) {
  if (!privacyMigration.includes(marker)) throw new Error(`Privacy governance migration is missing: ${marker}`);
}
const apiAuth = readFileSync("services/core-api/internal/httpapi/auth.go", "utf8");
for (const marker of ["RequiredACR", "RequiredAMR", "AuthenticationTime", "MaxAuthAge"]) {
  if (!apiAuth.includes(marker)) throw new Error(`API assurance policy is missing: ${marker}`);
}

console.log(`Wave 2 repository control set valid: ${ids.size} controls, external approvals remain explicit.`);
