import { existsSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const read = (path) => readFileSync(path, "utf8");
const json = (path) => JSON.parse(read(path));
const fail = (message) => { throw new Error(`Wave 4 evidence invalid: ${message}`); };

const evidence = json("docs/implementation/evidence/wave-4/control-set.json");
if (evidence.repository_status !== "COMPLETE" || evidence.release_exit_status !== "BLOCKED_EXTERNAL") {
  fail("status must distinguish repository completion from production acceptance");
}
if (!Array.isArray(evidence.repository_controls) || evidence.repository_controls.length < 9) fail("repository control set is incomplete");
for (const control of evidence.repository_controls) {
  if (control.status !== "IMPLEMENTED") fail(`${control.id} is not implemented`);
  for (const path of control.evidence ?? []) if (!existsSync(path)) fail(`${control.id} evidence is missing: ${path}`);
}
if (!Array.isArray(evidence.external_gates) || evidence.external_gates.length < 5) fail("external gates are incomplete");
for (const gate of evidence.external_gates) if (gate.status !== "BLOCKED_EXTERNAL" || !gate.required) fail(`${gate.id} must remain external-blocked`);

const environments = json("infra/environments/control-set.json");
const requiredOrder = ["configuration", "test", "staging", "pilot", "production"];
if (JSON.stringify(environments.promotion_order) !== JSON.stringify(requiredOrder)) fail("environment promotion order is invalid");
for (let index = 0; index < requiredOrder.length; index += 1) {
  const item = environments.environments[index];
  const predecessor = index === 0 ? null : requiredOrder[index - 1];
  if (item?.name !== requiredOrder[index] || item.predecessor !== predecessor || item.approval_required !== true) fail(`environment boundary is invalid at ${requiredOrder[index]}`);
}
for (const [control, enabled] of Object.entries(environments.required_controls ?? {})) if (enabled !== true) fail(`${control} is not fail-closed`);
for (const required of ["development_seed_disabled", "header_identity_disabled", "unsafe_debug_disabled", "point_in_time_recovery", "immutable_backup_copy"]) {
  if (environments.required_controls[required] !== true) fail(`missing platform control ${required}`);
}

const terraform = read("infra/terraform/variables.tf");
for (const environment of requiredOrder) if (!terraform.includes(`"${environment}"`)) fail(`Terraform does not allow ${environment}`);
for (const marker of ["alltrue(values(var.deployment_safety))", "development_seed_disabled", "header_identity_disabled", "unsafe_debug_disabled"]) if (!terraform.includes(marker)) fail(`Terraform safety contract is missing ${marker}`);

const promotion = read(".github/workflows/platform-promotion.yml");
for (const marker of ["workflow_dispatch", "cancel-in-progress: false", "environment:", "release_bundle_sha256", "migration_bundle_sha256", "predecessor_evidence", "change_ticket", "rollback_plan", "retention-days: 365", "BLOCKED_UNTIL_APPROVED_ADAPTER_IS_CONFIGURED"]) {
  if (!promotion.includes(marker) && !read("scripts/validate-promotion-inputs.mjs").includes(marker)) fail(`promotion gate is missing ${marker}`);
}
execFileSync(process.execPath, ["scripts/validate-promotion-inputs.mjs", "--self-test"], { stdio: "inherit" });

const adapter = json("infra/deployment/adapter-contract.json");
if (adapter.status !== "READY_FOR_PROVIDER_IMPLEMENTATION" || adapter.security?.authentication !== "workload_identity_only" || adapter.security?.long_lived_credentials !== false || adapter.security?.unmanaged_production_shell !== false) fail("provider adapter contract is not fail-closed");
for (const action of ["plan", "promote", "rollback"]) if (!adapter.actions.includes(action)) fail(`provider adapter is missing ${action}`);
for (const environment of requiredOrder) if (!adapter.promotion.ordered_environments.includes(environment)) fail(`provider adapter is missing ${environment}`);

const slos = json("infra/observability/slo-catalog.json");
if (slos.status !== "PROPOSED_EXTERNAL_APPROVAL" || slos.measurement_window_days !== 28) fail("SLO status or window is unsafe");
const journeys = new Set(slos.slos.map((slo) => slo.journey));
for (const journey of ["online_sale_posting", "mobile_sale_synchronization", "financial_and_stock_posting", "governed_financial_reporting", "mandatory_integration_delivery"]) if (!journeys.has(journey)) fail(`SLO is missing ${journey}`);

const alerts = json("infra/observability/alert-catalog.json");
const requiredSignals = ["service_availability", "request_latency", "request_error_rate", "database_saturation", "oldest_unpublished_outbox_age", "fiscal_delivery_backlog", "payment_callback_failure_rate", "mobile_sync_failure_rate", "unresolved_reconciliation_exception", "backup_or_restore_verification_failure"];
for (const signal of requiredSignals) if (!alerts.alerts.some((alert) => alert.signal === signal)) fail(`alert catalog is missing ${signal}`);
const prometheusRules = read("infra/observability/prometheus-rules.yaml");
for (const alert of alerts.alerts) if (!prometheusRules.includes(`catalog_id: ${alert.id}`)) fail(`Prometheus rules are missing ${alert.id}`);
json("infra/observability/grafana-platform-dashboard.json");

const productionCollector = read("infra/observability/otel-collector.production.example.yaml");
for (const marker of ["otlphttp/managed", "insecure: false", "retry_on_failure", "sending_queue", "http.request.header.authorization", "http.request.header.cookie", "db.statement", "action: delete"]) if (!productionCollector.includes(marker)) fail(`production telemetry is missing ${marker}`);
if (productionCollector.includes("exporters:\n  debug:")) fail("production telemetry exports to debug");

const runbooks = read("docs/operations/platform-runbooks.md");
const runbookIds = new Set(alerts.alerts.map((alert) => alert.runbook));
for (const slo of slos.slos) runbookIds.add(slo.runbook);
for (const runbook of runbookIds) if (!runbooks.includes(`## ${runbook}`)) fail(`runbook is missing ${runbook}`);

const recovery = read("docs/operations/recovery-and-dr.md");
for (const marker of ["Proposed RPO", "Proposed RTO", "point-in-time recovery", "journal headers/lines", "stock movements", "audit events", "outbox events", "document", "actual RPO/RTO"]) if (!recovery.includes(marker)) fail(`recovery control is missing ${marker}`);

const apiMain = read("services/core-api/cmd/api/main.go");
if (!apiMain.includes('return environment == "development"')) fail("API unsafe fallback is no longer exact-development-only");
const devSeed = read("services/core-api/cmd/devseed/main.go");
if (!devSeed.includes("refusing to seed ITEMBA_ENV")) fail("development seed does not fail closed");
const telemetry = read("infra/observability/otel-collector.yaml");
for (const marker of ["http.request.header.authorization", "action: delete", "enduser.id", "action: hash"]) if (!telemetry.includes(marker)) fail(`telemetry redaction is missing ${marker}`);

const runtimeMetrics = read("services/core-api/internal/platform/telemetry/metrics.go");
for (const marker of ["itemba_http_requests_total", "itemba_database_pool_connections", "itemba_outbox_oldest_unpublished_seconds", "itemba_integration_backlog", "operationalLabelPattern"]) if (!runtimeMetrics.includes(marker)) fail(`runtime metrics are missing ${marker}`);
const runtimeLogger = read("services/core-api/internal/platform/telemetry/logger.go");
for (const marker of ["RedactingHandler", "authorization", "password", "private_key", "request_body", "payload", "urlCredentialPattern"]) if (!runtimeLogger.includes(marker)) fail(`runtime redaction is missing ${marker}`);
if (read("services/core-api/internal/httpapi/handler.go").includes('"path", request.URL.Path')) fail("HTTP logs include raw request paths");

const operationalMigration = read("services/core-api/migrations/000036_platform_operational_metrics.up.sql");
for (const marker of ["SECURITY DEFINER", "REVOKE ALL", "itemba_outbox_oldest_unpublished_seconds", "itemba_integration_backlog", "itemba_reconciliation_open_critical"]) if (!operationalMigration.includes(marker)) fail(`operational metric migration is missing ${marker}`);

const recoveryWorkflow = read(".github/workflows/recovery-assurance.yml");
for (const marker of ["pg_dump", "pg_restore", "source-manifest.json", "restored-manifest.json", "forward-compatible-manifest.json", "previous-binary", "retention-days: 365"]) if (!recoveryWorkflow.includes(marker)) fail(`recovery workflow is missing ${marker}`);
const recoveryManifest = read("services/core-api/internal/platform/recovery/manifest.go");
if (!recoveryManifest.includes("SequenceSHA256")) fail("recovery manifest omits sequence state");
for (const table of ["journals", "journal_lines", "inventory_stock_ledger", "customer_ledger", "supplier_ledger", "audit_events", "outbox_events", "integration_deliveries", "employee_documents", "payroll_export_artifacts", "report_exports"]) if (!recoveryManifest.includes(`"${table}"`)) fail(`recovery manifest is missing ${table}`);

const platformExercise = read(".github/workflows/platform-exercises.yml");
for (const marker of ["platform-load-probe.mjs", "ITEMBA_METRICS_ADDRESS", "ITEMBA_LOAD_MAX_P95_MS", "ITEMBA_LOAD_MAX_ERROR_RATE", "retention-days: 365"]) if (!platformExercise.includes(marker)) fail(`platform exercise is missing ${marker}`);
execFileSync(process.execPath, ["scripts/platform-load-probe.mjs", "--self-test"], { stdio: "inherit" });

const release = read(".github/workflows/release-supply-chain.yml");
if (!release.includes("itemba-recoverymanifest")) fail("release bundle omits recovery manifest tool");
const acceptance = read("docs/operations/operational-acceptance-checklist.md");
for (const marker of ["[ ]", "Platform/SRE", "production-like restore", "Operations", "release_exit_status=BLOCKED_EXTERNAL"]) if (!acceptance.includes(marker)) fail(`operational acceptance checklist is missing ${marker}`);

console.log(`Wave 4 repository controls verified: ${evidence.repository_controls.length} implemented controls; ${evidence.external_gates.length} production gates remain external-blocked.`);
