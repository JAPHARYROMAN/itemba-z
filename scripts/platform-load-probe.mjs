import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { performance } from "node:perf_hooks";

const percentile = (values, fraction) => {
  if (!values.length) return 0;
  const ordered = [...values].sort((left, right) => left - right);
  return ordered[Math.min(ordered.length - 1, Math.ceil(ordered.length * fraction) - 1)];
};

if (process.argv.includes("--self-test")) {
  if (percentile([40, 10, 20, 30], 0.95) !== 40 || percentile([], 0.95) !== 0) throw new Error("percentile self-test failed");
  console.log("Platform load probe self-test passed.");
  process.exit(0);
}

const integer = (name, fallback, minimum, maximum) => {
  const value = Number(process.env[name] ?? fallback);
  if (!Number.isInteger(value) || value < minimum || value > maximum) throw new Error(`${name} must be an integer from ${minimum} to ${maximum}`);
  return value;
};
const number = (name, fallback, minimum, maximum) => {
  const value = Number(process.env[name] ?? fallback);
  if (!Number.isFinite(value) || value < minimum || value > maximum) throw new Error(`${name} must be from ${minimum} to ${maximum}`);
  return value;
};
const required = (name) => {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
};

const baseURL = new URL(required("ITEMBA_LOAD_BASE_URL"));
if (!/^https?:$/.test(baseURL.protocol)) throw new Error("ITEMBA_LOAD_BASE_URL must use HTTP or HTTPS");
const endpointPaths = required("ITEMBA_LOAD_ENDPOINTS").split(",").map((value) => value.trim()).filter(Boolean);
if (!endpointPaths.length || endpointPaths.some((value) => !value.startsWith("/") || value.includes("?") || value.includes("#"))) throw new Error("ITEMBA_LOAD_ENDPOINTS must contain safe absolute paths");
const durationSeconds = integer("ITEMBA_LOAD_DURATION_SECONDS", 20, 5, 900);
const concurrency = integer("ITEMBA_LOAD_CONCURRENCY", 20, 1, 500);
const timeoutMS = integer("ITEMBA_LOAD_TIMEOUT_MS", 5000, 100, 30000);
const maximumP95MS = number("ITEMBA_LOAD_MAX_P95_MS", 2000, 1, 30000);
const maximumErrorRate = number("ITEMBA_LOAD_MAX_ERROR_RATE", 0, 0, 1);
const minimumRequests = integer("ITEMBA_LOAD_MIN_REQUESTS", 100, 1, 10000000);
const evidencePath = required("ITEMBA_LOAD_EVIDENCE_PATH");
const identityHeaders = {};
for (const [header, environment] of [["X-Tenant-ID", "ITEMBA_LOAD_TENANT_ID"], ["X-Company-ID", "ITEMBA_LOAD_COMPANY_ID"], ["X-Branch-ID", "ITEMBA_LOAD_BRANCH_ID"], ["X-Warehouse-ID", "ITEMBA_LOAD_WAREHOUSE_ID"], ["X-Actor-ID", "ITEMBA_LOAD_ACTOR_ID"]]) {
  if (process.env[environment]?.trim()) identityHeaders[header] = process.env[environment].trim();
}

const deadline = performance.now() + durationSeconds * 1000;
const latencies = [];
const statusCounts = new Map();
let attempted = 0;
let failed = 0;

async function worker(workerIndex) {
  let sequence = workerIndex;
  while (performance.now() < deadline) {
    const path = endpointPaths[sequence % endpointPaths.length];
    sequence += concurrency;
    attempted++;
    const started = performance.now();
    try {
      const response = await fetch(new URL(path, baseURL), { headers: { ...identityHeaders, "X-Correlation-ID": randomUUID() }, signal: AbortSignal.timeout(timeoutMS) });
      await response.arrayBuffer();
      statusCounts.set(response.status, (statusCounts.get(response.status) ?? 0) + 1);
      if (response.status < 200 || response.status >= 300) failed++;
    } catch {
      failed++;
      statusCounts.set("transport_error", (statusCounts.get("transport_error") ?? 0) + 1);
    }
    latencies.push(performance.now() - started);
  }
}

await Promise.all(Array.from({ length: concurrency }, (_, index) => worker(index)));
const errorRate = attempted === 0 ? 1 : failed / attempted;
const p95MS = percentile(latencies, 0.95);
const evidence = {
  schema_version: 1,
  status: attempted >= minimumRequests && errorRate <= maximumErrorRate && p95MS <= maximumP95MS ? "PASSED_ISOLATED_LOAD_PROBE" : "FAILED",
  attempted,
  failed,
  error_rate: errorRate,
  latency_ms: { p50: percentile(latencies, 0.5), p95: p95MS, p99: percentile(latencies, 0.99), max: percentile(latencies, 1) },
  status_counts: Object.fromEntries([...statusCounts.entries()].sort(([left], [right]) => String(left).localeCompare(String(right)))),
  configuration: { endpoint_paths: endpointPaths, duration_seconds: durationSeconds, concurrency, timeout_ms: timeoutMS, maximum_p95_ms: maximumP95MS, maximum_error_rate: maximumErrorRate, minimum_requests: minimumRequests },
  limitations: ["ephemeral_ci_runtime", "synthetic_read_workload", "not_production_capacity_evidence"],
  recorded_at: new Date().toISOString()
};
writeFileSync(evidencePath, `${JSON.stringify(evidence, null, 2)}\n`, { flag: "wx", mode: 0o600 });
console.log(JSON.stringify(evidence));
if (evidence.status !== "PASSED_ISOLATED_LOAD_PROBE") process.exit(1);
