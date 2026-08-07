import { spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const evidenceRoot = join(root, "docs/implementation/evidence/wave-0");
const required = [
  "README.md",
  "repository-inventory.json",
  "verification-report.md",
  "release-1-scope-register.md",
  "production-readiness-register.md",
  "production-readiness-baseline.md",
  "ownership-and-signoff-register.md",
  "provider-access-register.md",
  "source-data-register.md",
  "organization-and-rollout-register.md",
  "decision-log.md",
  "authorized-input-form.md",
  "wave-0-completion-report.md",
];

const errors = [];
for (const name of required) {
  if (!existsSync(join(evidenceRoot, name))) errors.push(`missing Wave 0 evidence: ${name}`);
}

const generated = spawnSync(process.execPath, [
  join(root, "scripts/generate-wave0-inventory.mjs"),
  "--as-of", "2026-08-06",
  "--source-commit", "a42e1fb5937454a1a0b7d12764d9319028747aad",
], { cwd: root, encoding: "utf8" });
if (generated.status !== 0) {
  errors.push(`inventory regeneration failed: ${generated.stderr.trim()}`);
} else {
  const expected = JSON.stringify(JSON.parse(readFileSync(join(evidenceRoot, "repository-inventory.json"), "utf8")));
  const actual = JSON.stringify(JSON.parse(generated.stdout));
  if (actual !== expected) errors.push("repository-inventory.json is stale; regenerate it");
}

const baseline = readFileSync(join(evidenceRoot, "production-readiness-baseline.md"), "utf8");
if (!baseline.includes("**46.5 / 100 — NOT AUTHORIZED FOR PRODUCTION**")) {
  errors.push("baseline score or launch state is missing");
}
if (!baseline.includes("| **Total** | **100.0** | **46.5** |")) {
  errors.push("baseline weighted total is missing or inconsistent");
}

const completion = readFileSync(join(evidenceRoot, "wave-0-completion-report.md"), "utf8");
if (!completion.includes("Wave 0 exit status: BLOCKED BY AUTHORIZED EXTERNAL INPUT")) {
  errors.push("Wave 0 exit blocker is not explicit");
}

const markdownFiles = [
  ...required.filter((name) => name.endsWith(".md")).map((name) => join(evidenceRoot, name)),
  join(root, "README.md"),
  join(root, "docs/implementation/current-status.md"),
  join(root, "docs/implementation/roadmap-to-production.md"),
];
const linkPattern = /\[[^\]]+\]\(([^)]+)\)/g;
for (const file of markdownFiles) {
  const source = readFileSync(file, "utf8");
  for (const match of source.matchAll(linkPattern)) {
    const target = match[1].trim();
    if (/^(https?:|mailto:|#)/.test(target)) continue;
    const withoutAnchor = target.split("#", 1)[0];
    if (!withoutAnchor) continue;
    const resolved = resolve(dirname(file), withoutAnchor);
    if (!existsSync(resolved)) errors.push(`broken relative link in ${file}: ${target}`);
  }
}

if (errors.length > 0) {
  for (const error of errors) console.error(`ERROR: ${error}`);
  process.exit(1);
}

console.log(`Wave 0 evidence valid: ${required.length} artifacts, reproducible inventory, consistent baseline, and working relative links.`);
