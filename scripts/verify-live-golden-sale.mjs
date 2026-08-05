import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";

const baseUrl = (process.env.ITEMBA_API_BASE_URL ?? "http://127.0.0.1:8080").replace(/\/$/, "");

const ids = Object.freeze({
  tenant: "00000000-0000-4000-8000-000000000001",
  company: "00000000-0000-4000-8000-000000000002",
  branch: "00000000-0000-4000-8000-000000000003",
  warehouse: "00000000-0000-4000-8000-000000000004",
  actor: "00000000-0000-4000-8000-000000000005",
  generalCustomer: "00000000-0000-4000-8000-000000000007",
  creditCustomer: "00000000-0000-4000-8000-000000000008",
  product: "00000000-0000-4000-8000-000000000009",
  device: "00000000-0000-4000-8000-000000000011",
});

function authenticationHeaders(overrides = {}) {
  const bearer = process.env.ITEMBA_BEARER_TOKEN?.trim();
  if (bearer) return { Authorization: `Bearer ${bearer}` };
  return {
    "X-Actor-ID": ids.actor,
    "X-Tenant-ID": ids.tenant,
    "X-Company-ID": ids.company,
    "X-Branch-ID": ids.branch,
    "X-Warehouse-ID": ids.warehouse,
    ...overrides,
  };
}

async function request(path, { method = "GET", body, headers = {}, expected = 200, identity = {} } = {}) {
  const response = await fetch(`${baseUrl}${path}`, {
    method,
    headers: {
      Accept: "application/json, application/problem+json",
      "X-Correlation-ID": randomUUID(),
      ...authenticationHeaders(identity),
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const payload = await response.json().catch(() => undefined);
  assert.equal(
    response.status,
    expected,
    `${method} ${path} returned ${response.status}: ${JSON.stringify(payload)}`,
  );
  return payload;
}

function cashCommand(quantity = 1, paymentMethod = "CASH") {
  return {
    customer_id: ids.generalCustomer,
    kind: "CASH",
    payment_method: paymentMethod,
    lines: [{ product_id: ids.product, quantity }],
  };
}

const context = await request("/v1/context");
assert.equal(context.actor_id, ids.actor);
assert.equal(context.tenant_id, ids.tenant);
assert.equal(context.company_id, ids.company);
assert.equal(context.branch_id, ids.branch);
assert.equal(context.warehouse_id, ids.warehouse);
assert.equal(context.currency, "TZS");
assert.equal(context.timezone, "Africa/Dar_es_Salaam");
assert.ok(context.permissions.includes("sales.complete"));

const snapshotQuery = `snapshot_token=${encodeURIComponent(context.catalog_snapshot_token)}`;
const customers = await request(`/v1/customers?page_size=200&${snapshotQuery}`);
assert.equal(customers.catalog_snapshot_token, context.catalog_snapshot_token);
assert.equal(customers.master_data_version, context.master_data_version);
assert.equal(customers.price_version, context.price_version);
assert.ok(customers.items.some((customer) => customer.id === ids.generalCustomer && customer.is_general_customer));
assert.ok(customers.items.some((customer) => customer.id === ids.creditCustomer && customer.credit_enabled));
const products = await request(`/v1/products?page_size=200&${snapshotQuery}`);
assert.equal(products.catalog_snapshot_token, context.catalog_snapshot_token);
assert.equal(products.master_data_version, context.master_data_version);
assert.equal(products.price_version, context.price_version);
const verifierProduct = products.items.find((product) => product.id === ids.product);
assert.ok(verifierProduct && verifierProduct.available_quantity > 0);
assert.equal(verifierProduct.tax_basis_points, 0, "offline verifier product must remain explicitly zero-rated");

const crossScope = await request("/v1/customers?page_size=1", {
  expected: 403,
  identity: { "X-Warehouse-ID": "00000000-0000-4000-8000-000000009999" },
});
assert.equal(crossScope.code, "forbidden");

const enrollment = await request("/v1/mobile/devices/enroll", {
  method: "POST",
  body: { device_id: ids.device, device_name: "Golden Sale HTTP Verifier", app_version: "smoke-1" },
});

assert.equal(enrollment.device_id, ids.device);
assert.equal(enrollment.actor_id, ids.actor);
assert.equal(enrollment.status, "ACTIVE");
assert.equal(enrollment.scope.warehouse_id, ids.warehouse);
assert.equal(enrollment.available_master_data_version, context.master_data_version);
assert.equal(enrollment.available_price_version, context.price_version);
assert.equal(enrollment.available_catalog_snapshot_token, context.catalog_snapshot_token);
const initialLeaseDeadline = Date.parse(enrollment.offline_sales_valid_until);
assert.ok(Number.isFinite(initialLeaseDeadline), "initial enrollment must return a lease deadline");
assert.ok(initialLeaseDeadline <= Date.now(), "unacknowledged enrollment must remain offline-expired");
assert.ok(Array.isArray(enrollment.stock_allocations));
const productAllocation = enrollment.stock_allocations.find(
  (allocation) => allocation.product_id === ids.product,
);
assert.ok(productAllocation, "the verifier product must have a device stock allocation");
assert.ok(productAllocation.remaining_quantity > 0, "the verifier allocation must have remaining stock");

const staleAcknowledgement = await request("/v1/mobile/devices/enroll", {
  method: "POST",
  body: {
    device_id: ids.device,
    device_name: "Golden Sale HTTP Verifier",
    app_version: "smoke-1",
    installed_master_data_version: enrollment.available_master_data_version + 1,
    installed_price_version: enrollment.available_price_version,
    installed_catalog_snapshot_token: enrollment.available_catalog_snapshot_token,
  },
  expected: 422,
});
assert.equal(staleAcknowledgement.code, "business_rule_violation");

const acknowledged = await request("/v1/mobile/devices/enroll", {
  method: "POST",
  body: {
    device_id: ids.device,
    device_name: "Golden Sale HTTP Verifier",
    app_version: "smoke-1",
    installed_master_data_version: enrollment.available_master_data_version,
    installed_price_version: enrollment.available_price_version,
    installed_catalog_snapshot_token: enrollment.available_catalog_snapshot_token,
  },
});
assert.equal(acknowledged.master_data_version, enrollment.available_master_data_version);
assert.equal(acknowledged.price_version, enrollment.available_price_version);
assert.equal(acknowledged.catalog_snapshot_token, enrollment.available_catalog_snapshot_token);
assert.equal(acknowledged.app_version, "smoke-1");
const acknowledgedAt = Date.parse(acknowledged.last_seen_at);
const acknowledgedLeaseDeadline = Date.parse(acknowledged.offline_sales_valid_until);
assert.ok(Number.isFinite(acknowledgedAt) && Number.isFinite(acknowledgedLeaseDeadline));
assert.ok(acknowledgedLeaseDeadline > Date.now(), "successful acknowledgement must issue a live lease");
assert.ok(
  acknowledgedLeaseDeadline > acknowledgedAt && acknowledgedLeaseDeadline <= acknowledgedAt + 4 * 60 * 60 * 1000,
  "acknowledged lease must be exclusive, positive, and no longer than four hours",
);

const unacknowledgedUpgrade = await request("/v1/mobile/devices/enroll", {
  method: "POST",
  body: { device_id: ids.device, device_name: "Golden Sale HTTP Verifier", app_version: "smoke-2" },
});
assert.equal(unacknowledgedUpgrade.app_version, "smoke-1");
assert.equal(Date.parse(unacknowledgedUpgrade.offline_sales_valid_until), acknowledgedLeaseDeadline);
assert.equal(unacknowledgedUpgrade.master_data_version, acknowledged.master_data_version);
assert.equal(unacknowledgedUpgrade.price_version, acknowledged.price_version);

// A second read-through enrollment proves the no-ack request did not merely
// echo the old app while persisting the new one.
const unacknowledgedUpgradeReadback = await request("/v1/mobile/devices/enroll", {
  method: "POST",
  body: { device_id: ids.device, device_name: "Golden Sale HTTP Verifier", app_version: "smoke-2" },
});
assert.equal(unacknowledgedUpgradeReadback.app_version, "smoke-1");
assert.equal(Date.parse(unacknowledgedUpgradeReadback.offline_sales_valid_until), acknowledgedLeaseDeadline);

const beforeMobileCreditSales = await request("/v1/sales?page_size=200");
const beforeMobileCreditProducts = await request(`/v1/products?page_size=200&${snapshotQuery}`);
const beforeMobileCreditStock = beforeMobileCreditProducts.items.find((product) => product.id === ids.product)?.available_quantity;
const mobileCreditTransactionId = randomUUID();
const mobileCreditCommand = {
  customer_id: ids.creditCustomer,
  kind: "CREDIT",
  lines: [{ product_id: ids.product, quantity: 1 }],
  device_id: ids.device,
  client_transaction_id: mobileCreditTransactionId,
  client_timestamp: new Date().toISOString(),
  app_version: "smoke-1",
  master_data_version: context.master_data_version,
  price_version: context.price_version,
  catalog_snapshot_token: context.catalog_snapshot_token,
  sync_attempt: 1,
  offline: false,
};
const mobileCredit = await request("/v1/mobile/sync/sales", {
  method: "POST",
  body: mobileCreditCommand,
});
assert.equal(mobileCredit.state, "synced");
assert.equal(mobileCredit.sale.kind, "CREDIT");
assert.equal(mobileCredit.sale.customer_id, ids.creditCustomer);
const mobileCreditReplay = await request("/v1/mobile/sync/sales", {
  method: "POST", body: { ...mobileCreditCommand, sync_attempt: 2 },
});
assert.equal(mobileCreditReplay.sale.id, mobileCredit.sale.id);
assert.equal(mobileCreditReplay.idempotent_replay, true);
const afterMobileCreditSales = await request("/v1/sales?page_size=200");
const afterMobileCreditProducts = await request(`/v1/products?page_size=200&${snapshotQuery}`);
assert.equal(afterMobileCreditSales.items.length, beforeMobileCreditSales.items.length + 1);
assert.equal(
  afterMobileCreditProducts.items.find((product) => product.id === ids.product)?.available_quantity,
  beforeMobileCreditStock - 1,
  "authorized online mobile credit must consume stock exactly once",
);
const offlineCredit = await request("/v1/mobile/sync/sales", {
  method: "POST",
  body: { ...mobileCreditCommand, client_transaction_id: randomUUID(), client_timestamp: new Date().toISOString(), offline: true },
  expected: 422,
});
assert.equal(offlineCredit.code, "business_rule_violation");

const clientTransactionId = randomUUID();
const mobileCommand = {
  ...cashCommand(),
  device_id: ids.device,
  client_transaction_id: clientTransactionId,
  client_timestamp: new Date().toISOString(),
  app_version: "smoke-1",
  master_data_version: context.master_data_version,
  price_version: context.price_version,
  catalog_snapshot_token: context.catalog_snapshot_token,
  sync_attempt: 1,
  offline: true,
};
const mobileFirst = await request("/v1/mobile/sync/sales", { method: "POST", body: mobileCommand });
assert.equal(mobileFirst.state, "synced");
assert.equal(mobileFirst.client_transaction_id, clientTransactionId);
assert.equal(mobileFirst.idempotent_replay, false);
assert.equal(mobileFirst.sale.offline, true);
assert.equal(Date.parse(mobileFirst.sale.client_timestamp), Date.parse(mobileCommand.client_timestamp));
assert.equal(mobileFirst.sale.app_version, mobileCommand.app_version);
assert.equal(mobileFirst.sale.master_data_version, mobileCommand.master_data_version);
assert.equal(mobileFirst.sale.price_version, mobileCommand.price_version);
assert.equal(mobileFirst.sale.catalog_snapshot_token, mobileCommand.catalog_snapshot_token);
assert.equal(mobileFirst.sale.fiscal_status, "NOT_CONFIGURED");
assert.equal(mobileFirst.receipt_reference, mobileFirst.sale.receipt_reference);
assert.equal("cogs_minor" in mobileFirst.sale, false);
assert.ok(mobileFirst.sale.lines.every((line) => !("unit_cost_minor" in line) && !("cogs_minor" in line)));
assert.equal("cogs_minor" in mobileFirst.sale, false);
assert.equal(mobileFirst.sale.lines.some((line) => "unit_cost_minor" in line || "cogs_minor" in line), false);

const mobileReplay = await request("/v1/mobile/sync/sales", { method: "POST", body: mobileCommand });
assert.equal(mobileReplay.sale.id, mobileFirst.sale.id);
assert.equal(mobileReplay.receipt_reference, mobileFirst.receipt_reference);
assert.equal(mobileReplay.idempotent_replay, true);

const mobileConflict = await request("/v1/mobile/sync/sales", {
  method: "POST",
  body: { ...mobileCommand, lines: [{ product_id: ids.product, quantity: 2 }] },
  expected: 409,
});
assert.equal(mobileConflict.code, "idempotency_conflict");

const offlineNonCash = await request("/v1/mobile/sync/sales", {
  method: "POST",
  body: { ...mobileCommand, payment_method: "BANK_CARD", client_transaction_id: randomUUID(), sync_attempt: 1 },
  expected: 422,
});
assert.equal(offlineNonCash.code, "business_rule_violation");

const unsupportedPayment = await request("/v1/sales", {
  method: "POST",
  body: cashCommand(1, "UNSUPPORTED"),
  headers: { "Idempotency-Key": `unsupported-payment-${randomUUID()}` },
  expected: 422,
});
assert.equal(unsupportedPayment.code, "business_rule_violation");

const unsafeWireInteger = await request("/v1/sales", {
  method: "POST",
  body: cashCommand(9_007_199_254_740_992),
  headers: { "Idempotency-Key": `unsafe-wire-integer-${randomUUID()}` },
  expected: 422,
});
assert.equal(unsafeWireInteger.code, "wire_integer_out_of_range");

const webKey = `control-center-${randomUUID()}`;
const webCommand = cashCommand(1, "BANK_CARD");
const webSale = await request("/v1/sales", {
  method: "POST",
  body: webCommand,
  headers: { "Idempotency-Key": webKey },
  expected: 201,
});
assert.equal(webSale.payment_method, "BANK_CARD");
assert.equal("cogs_minor" in webSale, false);
assert.ok(webSale.lines.every((line) => !("unit_cost_minor" in line) && !("cogs_minor" in line)));
const webReplay = await request("/v1/sales", {
  method: "POST",
  body: webCommand,
  headers: { "Idempotency-Key": webKey },
  expected: 201,
});
assert.equal(webReplay.id, webSale.id);

const webConflict = await request("/v1/sales", {
  method: "POST",
  body: cashCommand(2, "BANK_CARD"),
  headers: { "Idempotency-Key": webKey },
  expected: 409,
});
assert.equal(webConflict.code, "idempotency_conflict");

for (const paymentMethod of ["CASH", "MOBILE_MONEY", "BANK_CARD", "BANK_TRANSFER"]) {
  const settlementSale = await request("/v1/sales", {
    method: "POST",
    body: { ...cashCommand(), payment_method: paymentMethod },
    headers: { "Idempotency-Key": `canonical-${paymentMethod.toLowerCase()}-${randomUUID()}` },
    expected: 201,
  });
  assert.equal(settlementSale.payment_method, paymentMethod);
}

const bankTransferSync = await request("/v1/mobile/sync/sales", {
  method: "POST",
  body: {
    ...mobileCommand,
    payment_method: "BANK_TRANSFER",
    client_transaction_id: randomUUID(),
    sync_attempt: 1,
    offline: false,
  },
});
assert.equal(bankTransferSync.sale.payment_method, "BANK_TRANSFER");
assert.equal(bankTransferSync.sale.offline ?? false, false);

const generalCredit = await request("/v1/sales", {
  method: "POST",
  body: { customer_id: ids.generalCustomer, kind: "CREDIT", lines: [{ product_id: ids.product, quantity: 1 }] },
  headers: { "Idempotency-Key": `general-credit-${randomUUID()}` },
  expected: 422,
});
assert.equal(generalCredit.code, "business_rule_violation");

const reversalKey = `reversal-${randomUUID()}`;
const reversal = await request(`/v1/sales/${webSale.id}/reversals`, {
  method: "POST",
  body: { reason: "Automated live golden-sale verification" },
  headers: { "Idempotency-Key": reversalKey },
  expected: 201,
});
assert.equal(reversal.reversal_of, webSale.id);
assert.equal(reversal.fiscal_status, "NOT_CONFIGURED");

const reversalReplay = await request(`/v1/sales/${webSale.id}/reversals`, {
  method: "POST",
  body: { reason: "Automated live golden-sale verification" },
  headers: { "Idempotency-Key": reversalKey },
  expected: 201,
});
assert.equal(reversalReplay.id, reversal.id);

const original = await request(`/v1/sales/${webSale.id}`);
assert.equal(original.status, "REVERSED");
const sales = await request("/v1/sales?page_size=200");
assert.ok(sales.items.some((sale) => sale.id === mobileFirst.sale.id));
assert.ok(sales.items.some((sale) => sale.id === webSale.id));
assert.ok(sales.items.some((sale) => sale.id === reversal.id));

console.log(`Live golden sale verified: mobile=${mobileFirst.sale.id} web=${webSale.id} reversal=${reversal.id}`);
