import { describe, expect, it, vi } from "vitest";
import { ItembaApiClient, LiveApiError } from "@/live-api/api-client";
import type { CompleteSaleCommand } from "@/live-api/types";

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
    await Promise.all([client.listCustomers(), client.listProducts()]);
    const urls = fetchImplementation.mock.calls.map(([url]) => String(url));
    expect(urls).toContain("http://core-api:8080/v1/customers?page_size=200");
    expect(urls).toContain("http://core-api:8080/v1/products?page_size=200");
    expect(urls.join(" ")).not.toContain("company_id");
    expect(urls.join(" ")).not.toContain("warehouse_id");
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
});
