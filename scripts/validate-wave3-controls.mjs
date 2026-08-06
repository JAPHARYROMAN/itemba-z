import { readFileSync, existsSync } from "node:fs";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");
const controls = JSON.parse(read("docs/implementation/evidence/wave-3/control-set.json"));
const fail = (message) => { throw new Error(`Wave 3 evidence invalid: ${message}`); };

if (controls.repository_status !== "COMPLETE" || controls.release_exit_status !== "BLOCKED_EXTERNAL") fail("status must distinguish repository completion from external release approval");
if (!Array.isArray(controls.repository_controls) || controls.repository_controls.some((control) => control.status !== "IMPLEMENTED")) fail("repository controls must be implemented");
for (const control of controls.repository_controls) for (const evidence of control.evidence) if (!existsSync(new URL(evidence, root))) fail(`missing evidence ${evidence}`);
const capabilities = ["TRA_FISCALIZATION","PAYMENT_CALLBACK","BANK_STATEMENT_IMPORT","PAYROLL_EXPORT","EMAIL","WHATSAPP","RECEIPT_PRINT"];
for (const capability of capabilities) {
  const gate = controls.external_gates.find((candidate) => candidate.capability === capability);
  if (!gate || gate.status !== "BLOCKED_EXTERNAL" || !gate.required) fail(`${capability} must remain explicitly blocked without provider evidence`);
}
const openapi = read("contracts/openapi/itemba-z.v1.yaml");
for (const operation of ["getIntegrationWorkspace","createIntegrationRoute","transitionIntegrationRoute","resetIntegrationCircuit","replayIntegrationDelivery"]) if (!openapi.includes(`operationId: ${operation}`)) fail(`missing OpenAPI operation ${operation}`);
const migration = read("services/core-api/migrations/000035_integration_operations.up.sql");
for (const table of ["integration_route_transitions","integration_delivery_replays","integration_circuit_resets"]) if (!migration.includes(table)) fail(`missing immutable evidence table ${table}`);
const worker = read("services/core-api/cmd/integration-worker/main.go");
if (!worker.includes("Registry: integrations.ConnectorMap{}")) fail("unapproved provider connector was registered");
const ui = read("apps/control-center/src/components/integrations/integration-operations.tsx");
for (const phrase of ["Integration operations","Uendeshaji wa miunganisho","replay_delivery","integrations.replay"]) if (!ui.includes(phrase)) fail(`missing bilingual/operator UI evidence ${phrase}`);
console.log("Wave 3 repository controls verified; all provider and professional gates remain explicitly external-blocked.");
