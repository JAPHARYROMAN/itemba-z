import { describe, expect, it, vi } from "vitest";
import { ItembaApiClient, LiveApiError } from "@/live-api/api-client";
import type { CompleteSaleCommand, CreateInventoryPolicyCommand, ScheduleCreditPolicyCommand } from "@/live-api/types";

const bearerIdentity = { kind: "bearer" as const, token: "server-only-token" };
const correlationId = "00000000-0000-4000-8000-000000000099";

describe("ItembaApiClient", () => {
  it("forwards OIDC, correlation and a stable idempotency key without client-owned prices", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ id: "sale-id" }, { status: 201 });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080/", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    const command: CompleteSaleCommand = {
      customer_id: "00000000-0000-4000-8000-000000000010",
      kind: "CASH",
      payment_method: "MOBILE_MONEY",
      lines: [{ product_id: "00000000-0000-4000-8000-000000000011", quantity: 2 }],
    };
    const idempotencyKey = "00000000-0000-4000-8000-000000000012";

    await client.completeSale(command, idempotencyKey);
    const [url, request] = fetchImplementation.mock.calls[0] ?? [];
    const headers = new Headers(request?.headers);
    expect(url).toBe("http://core-api:8080/v1/sales");
    expect(headers.get("Authorization")).toBe("Bearer server-only-token");
    expect(headers.get("Idempotency-Key")).toBe(idempotencyKey);
    expect(headers.get("X-Correlation-ID")).toBe(correlationId);
    expect(JSON.parse(String(request?.body))).toEqual(command);
    expect(String(request?.body)).not.toContain("unit_price");
  });

  it("uses scope-derived bootstrap URLs without organization query parameters", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ items: [], next_cursor: null });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    await Promise.all([client.listCustomers(), client.listProducts(), client.getDashboard()]);
    const urls = fetchImplementation.mock.calls.map(([url]) => String(url));
    expect(urls).toContain("http://core-api:8080/v1/customers?page_size=200");
    expect(urls).toContain("http://core-api:8080/v1/products?page_size=200");
    expect(urls).toContain("http://core-api:8080/v1/dashboard");
    expect(urls.join(" ")).not.toContain("company_id");
    expect(urls.join(" ")).not.toContain("warehouse_id");
  });

  it("encodes an optional governed dashboard period only when both dates are supplied", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ as_of: "2026-08-06T10:00:00Z", metrics: [], alerts: [], pending_approvals: 0 });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    await client.getDashboard("2026-08-01", "2026-08-06");
    expect(String(fetchImplementation.mock.calls[0]?.[0])).toBe("http://core-api:8080/v1/dashboard?from=2026-08-01&to=2026-08-06");
    await expect(client.getDashboard("2026-08-01")).rejects.toMatchObject({ problem: { code: "dashboard_period_invalid" } });
    expect(fetchImplementation).toHaveBeenCalledTimes(1);
  });

  it("uses customer-scoped receivables URLs and preserves the policy idempotency key", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input; void init;
      return Response.json({ id: "policy-id" }, { status: 201 });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    const command: ScheduleCreditPolicyCommand = {
      credit_enabled: true, credit_limit_minor: 5_000_000, payment_terms_days: 30, max_overdue_days: 7,
      risk_status: "WATCH", reason: "Approved after finance review", effective_from: "2026-08-06T09:00:00Z",
    };
    await client.scheduleCustomerCreditPolicy("customer/id", command, "credit-policy-request-0001");
    const [url, request] = fetchImplementation.mock.calls[0] ?? [];
    expect(String(url)).toBe("http://core-api:8080/v1/customers/customer%2Fid/credit-policies");
    expect(new Headers(request?.headers).get("Idempotency-Key")).toBe("credit-policy-request-0001");
    expect(JSON.parse(String(request?.body))).toEqual(command);
  });

  it("rejects an invalid idempotency key before making a network request", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({});
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    await expect(client.reverseSale("sale-id", { reason: "Verified return" }, "short")).rejects.toBeInstanceOf(LiveApiError);
    expect(fetchImplementation).not.toHaveBeenCalled();
  });

  it("rejects any successful API payload containing a non-safe integer", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ items: [{ unit_price_minor: Number.MAX_SAFE_INTEGER + 1 }], next_cursor: null });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });

    await expect(client.listProducts()).rejects.toMatchObject({
      problem: {
        status: 502,
        code: "upstream_integer_unsafe",
        detail: expect.stringContaining("$.items[0].unit_price_minor"),
      },
    });
  });

  it("rejects a non-safe integer before interpreting an API error payload", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({
        title: "Rejected",
        detail: "Invalid command",
        retry_after: Number.MAX_SAFE_INTEGER + 1,
      }, { status: 422 });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });

    await expect(client.listProducts()).rejects.toMatchObject({
      problem: {
        status: 502,
        code: "upstream_integer_unsafe",
        detail: expect.stringContaining("$.retry_after"),
      },
    });
  });

  it("rejects unsafe command integers before making a network request", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({});
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });

    await expect(client.completeSale({
      customer_id: "00000000-0000-4000-8000-000000000010",
      kind: "CASH",
      payment_method: "CASH",
      lines: [{ product_id: "00000000-0000-4000-8000-000000000011", quantity: Number.MAX_SAFE_INTEGER + 1 }],
    }, "00000000-0000-4000-8000-000000000012")).rejects.toMatchObject({ problem: { code: "request_integer_unsafe" } });
    expect(fetchImplementation).not.toHaveBeenCalled();
  });

  it("uses scoped reconciliation URLs and forwards an idempotent disposition", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ items: [], next_cursor: null });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    await client.listReconciliationCases("OPEN", "cursor-token");
    await client.resolveReconciliationCase("case/id", { action: "CASH_REFUNDED", reason: "Cash returned to customer" }, "reconciliation-idempotency-0001");

    const [listUrl] = fetchImplementation.mock.calls[0] ?? [];
    const [resolutionUrl, resolutionRequest] = fetchImplementation.mock.calls[1] ?? [];
    expect(String(listUrl)).toBe("http://core-api:8080/v1/mobile/reconciliation-cases?page_size=100&status=OPEN&cursor=cursor-token");
    expect(String(resolutionUrl)).toBe("http://core-api:8080/v1/mobile/reconciliation-cases/case%2Fid/resolutions");
    expect(new Headers(resolutionRequest?.headers).get("Idempotency-Key")).toBe("reconciliation-idempotency-0001");
    expect(JSON.parse(String(resolutionRequest?.body))).toEqual({ action: "CASH_REFUNDED", reason: "Cash returned to customer" });
  });

  it("encodes inventory policy transitions and preserves governed policy commands", async () => {
    const fetchImplementation = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input; void init;
      return Response.json({ id: "policy-id", status: "SUBMITTED" });
    });
    const client = new ItembaApiClient({ baseUrl: "http://core-api:8080", identity: bearerIdentity, fetchImplementation, createCorrelationId: () => correlationId });
    const command: CreateInventoryPolicyCommand = { product_id: "00000000-0000-4000-8000-000000000011", cost_method: "MOVING_AVERAGE", lot_controlled: true, reorder_point: 20, reorder_quantity: 30, maximum_stock: 100, safety_stock: 10, lead_time_days: 7, reason: "Govern warehouse replenishment" };
    await client.createInventoryPolicy(command, "inventory-policy-create-0001");
    await client.transitionInventoryPolicy("policy/id", "SUBMITTED", "Submit for independent review", "inventory-policy-submit-0001");
    const [createUrl, createRequest] = fetchImplementation.mock.calls[0] ?? [];
    const [transitionUrl, transitionRequest] = fetchImplementation.mock.calls[1] ?? [];
    expect(String(createUrl)).toBe("http://core-api:8080/v1/inventory/policies");
    expect(JSON.parse(String(createRequest?.body))).toEqual(command);
    expect(String(transitionUrl)).toBe("http://core-api:8080/v1/inventory/policies/policy%2Fid/transitions");
    expect(new Headers(transitionRequest?.headers).get("Idempotency-Key")).toBe("inventory-policy-submit-0001");
  });
});
