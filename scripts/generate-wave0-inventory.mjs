import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

const repositoryRoot = resolve(import.meta.dirname, "..");
const args = new Map();
for (let index = 2; index < process.argv.length; index += 2) {
  args.set(process.argv[index], process.argv[index + 1]);
}
const asOf = args.get("--as-of");
if (!/^\d{4}-\d{2}-\d{2}$/.test(asOf ?? "")) {
  throw new Error("--as-of YYYY-MM-DD is required so the evidence is reproducible");
}
const sourceCommit = args.get("--source-commit")
  ?? execFileSync("git", ["rev-parse", "HEAD"], { cwd: repositoryRoot, encoding: "utf8" }).trim();
if (!/^[0-9a-f]{40}$/.test(sourceCommit)) {
  throw new Error("--source-commit must be a full 40-character Git commit SHA");
}

function git(args) {
  return execFileSync("git", args, { cwd: repositoryRoot, encoding: "utf8" });
}

function filesAtSource(prefix) {
  return git(["ls-tree", "-r", "--name-only", sourceCommit, "--", prefix])
    .split(/\r?\n/)
    .map((path) => path.trim())
    .filter(Boolean);
}

function readAtSource(path) {
  return git(["show", `${sourceCommit}:${path}`]);
}

function appRoute(path, marker) {
  const suffix = path.split(marker)[1];
  const withoutFile = suffix.replace(/\/(page|route)\.tsx$/, "");
  return withoutFile === "" ? "/" : withoutFile;
}

function parseOpenApi() {
  const lines = readAtSource("contracts/openapi/itemba-z.v1.yaml").split(/\r?\n/);
  const operations = [];
  let currentPath = null;
  let currentOperation = null;
  for (const line of lines) {
    const pathMatch = /^  (\/v1\/[^:]+):\s*$/.exec(line);
    if (pathMatch) {
      currentPath = pathMatch[1];
      currentOperation = null;
      continue;
    }
    const methodMatch = /^    (get|post|put|patch|delete):\s*$/.exec(line);
    if (methodMatch && currentPath) {
      currentOperation = { method: methodMatch[1].toUpperCase(), path: currentPath, status: "unclassified" };
      operations.push(currentOperation);
      continue;
    }
    const statusMatch = /^      x-implementation-status: (implemented|planned)\s*$/.exec(line);
    if (statusMatch && currentOperation) currentOperation.status = statusMatch[1];
  }
  return operations;
}

const openApiOperations = parseOpenApi();
const pageRoot = "apps/control-center/src/app";
const controlCenterPages = filesAtSource(pageRoot).filter((path) => path.endsWith("/page.tsx") || path === `${pageRoot}/page.tsx`).map((path) => {
  const source = readAtSource(path);
  let status = "unclassified";
  let evidence = "No authoritative data boundary detected";
  if (source.includes("@/live-api/")) {
    status = "live";
    evidence = "Uses the live API snapshot boundary and fail-closed unavailable state";
  } else if (source.includes("@/data/erp-repository")) {
    status = "demonstration";
    evidence = "Uses the in-repository mock ERP data source";
  }
  return { route: appRoute(path, "apps/control-center/src/app"), file: path, status, evidence };
}).sort((left, right) => left.route.localeCompare(right.route));

const bffRoutes = filesAtSource(`${pageRoot}/api`).filter((path) => path.endsWith("/route.ts")).map((path) => ({
  route: appRoute(path, "apps/control-center/src/app"),
  file: path,
})).sort((left, right) => left.route.localeCompare(right.route));

const mobilePresentationFiles = filesAtSource("apps/sales-mobile/lib/presentation")
  .filter((path) => /^apps\/sales-mobile\/lib\/presentation\/[^/]+\.dart$/.test(path))
  .sort();

const coreModules = [...new Set(filesAtSource("services/core-api/internal")
  .map((path) => path.split("/")[3])
  .filter(Boolean))]
  .sort();

const migrations = filesAtSource("services/core-api/migrations")
  .filter((path) => path.endsWith(".up.sql"))
  .map((path) => path.split("/").at(-1))
  .sort();

const inventory = {
  schema_version: 1,
  as_of: asOf,
  git_commit: sourceCommit,
  summary: {
    openapi_paths: new Set(openApiOperations.map((operation) => operation.path)).size,
    openapi_operations: openApiOperations.length,
    implemented_openapi_operations: openApiOperations.filter((operation) => operation.status === "implemented").length,
    planned_openapi_operations: openApiOperations.filter((operation) => operation.status === "planned").length,
    control_center_pages: controlCenterPages.length,
    live_control_center_pages: controlCenterPages.filter((page) => page.status === "live").length,
    demonstration_control_center_pages: controlCenterPages.filter((page) => page.status === "demonstration").length,
    bff_routes: bffRoutes.length,
    mobile_presentation_files: mobilePresentationFiles.length,
    core_modules: coreModules.length,
    database_migrations: migrations.length,
  },
  openapi_operations: openApiOperations,
  control_center_pages: controlCenterPages,
  bff_routes: bffRoutes,
  mobile_presentation_files: mobilePresentationFiles,
  core_modules: coreModules,
  database_migrations: migrations,
};

const encoded = `${JSON.stringify(inventory, null, 2)}\n`;
const output = args.get("--output");
if (output) {
  const outputPath = resolve(repositoryRoot, output);
  mkdirSync(dirname(outputPath), { recursive: true });
  writeFileSync(outputPath, encoded, "utf8");
} else {
  process.stdout.write(encoded);
}
