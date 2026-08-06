import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";

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

function filesUnder(directory, name) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return filesUnder(path, name);
    return entry.name === name ? [path] : [];
  });
}

function portable(path) {
  return relative(repositoryRoot, path).split(sep).join("/");
}

function appRoute(path, marker) {
  const suffix = portable(path).split(marker)[1];
  const withoutFile = suffix.replace(/\/(page|route)\.tsx$/, "");
  return withoutFile === "" ? "/" : withoutFile;
}

function parseOpenApi() {
  const lines = readFileSync(join(repositoryRoot, "contracts/openapi/itemba-z.v1.yaml"), "utf8").split(/\r?\n/);
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
const pageRoot = join(repositoryRoot, "apps/control-center/src/app");
const controlCenterPages = filesUnder(pageRoot, "page.tsx").map((path) => {
  const source = readFileSync(path, "utf8");
  let status = "unclassified";
  let evidence = "No authoritative data boundary detected";
  if (source.includes("@/live-api/")) {
    status = "live";
    evidence = "Uses the live API snapshot boundary and fail-closed unavailable state";
  } else if (source.includes("@/data/erp-repository")) {
    status = "demonstration";
    evidence = "Uses the in-repository mock ERP data source";
  }
  return { route: appRoute(path, "apps/control-center/src/app"), file: portable(path), status, evidence };
}).sort((left, right) => left.route.localeCompare(right.route));

const bffRoutes = filesUnder(join(pageRoot, "api"), "route.ts").map((path) => ({
  route: appRoute(path, "apps/control-center/src/app"),
  file: portable(path),
})).sort((left, right) => left.route.localeCompare(right.route));

const mobilePresentationFiles = readdirSync(join(repositoryRoot, "apps/sales-mobile/lib/presentation"), { withFileTypes: true })
  .filter((entry) => entry.isFile() && entry.name.endsWith(".dart"))
  .map((entry) => `apps/sales-mobile/lib/presentation/${entry.name}`)
  .sort();

const coreModules = readdirSync(join(repositoryRoot, "services/core-api/internal"), { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
  .sort();

const migrations = readdirSync(join(repositoryRoot, "services/core-api/migrations"))
  .filter((name) => name.endsWith(".up.sql"))
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
