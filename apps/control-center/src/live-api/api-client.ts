import type { BackendIdentity } from "@/live-api/auth";
import { buildIdentityHeaders } from "@/live-api/auth";
import type {
  ChangeMobileDeviceAllocationCommand, ChangeMobileDeviceStatusCommand, CompleteSaleCommand, CustomerAccountDetail, CustomerPage,
  MobileDevice, MobileDevicePage, MobileReconciliationCase, MobileReconciliationPage,
  MobileReconciliationStatus, ProductPage, PublicProblem, ResolveMobileReconciliationCommand,
  ReverseSaleCommand, Sale, SalePage, ScheduleCreditPolicyCommand, CreditPolicy, WorkingContext,
} from "@/live-api/types";
import { firstUnsafeIntegerPath } from "@/live-api/integer-safety";

type FetchImplementation = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

export class LiveApiError extends Error {
  readonly problem: PublicProblem;

  constructor(problem: PublicProblem) {
    super(problem.detail);
    this.name = "LiveApiError";
    this.problem = problem;
  }
}

export interface ApiClientOptions {
  baseUrl: string;
  identity: BackendIdentity;
  fetchImplementation?: FetchImplementation;
  createCorrelationId?: () => string;
  requestTimeoutMs?: number;
}

interface RequestOptions {
  method?: "GET" | "POST";
  body?: unknown;
  idempotencyKey?: string;
}

function normalizeBaseUrl(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:configuration",
      title: "Live API configuration invalid",
      status: 500,
      code: "api_base_url_invalid",
      detail: "ITEMBA_API_BASE_URL must be an absolute HTTP or HTTPS URL.",
    });
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:configuration",
      title: "Live API configuration invalid",
      status: 500,
      code: "api_base_url_invalid",
      detail: "ITEMBA_API_BASE_URL must use HTTP or HTTPS.",
    });
  }
  return url.toString().replace(/\/$/, "");
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && !/[\r\n]/.test(value);
}

function problemFromPayload(status: number, payload: unknown, correlationId: string): PublicProblem {
  if (payload && typeof payload === "object") {
    const value = payload as Record<string, unknown>;
    return {
      type: typeof value.type === "string" ? value.type : "about:blank",
      title: typeof value.title === "string" ? value.title : "Live API request failed",
      status,
      code: typeof value.code === "string" ? value.code : `upstream_http_${status}`,
      detail: typeof value.detail === "string" ? value.detail : "The ERP service rejected the request.",
      correlation_id: typeof value.correlation_id === "string" ? value.correlation_id : correlationId,
    };
  }
  return {
    type: "urn:itemba-z:control-center:upstream",
    title: "Live API request failed",
    status,
    code: `upstream_http_${status}`,
    detail: "The ERP service returned an unreadable error response.",
    correlation_id: correlationId,
  };
}

export class ItembaApiClient {
  private readonly baseUrl: string;
  private readonly identity: BackendIdentity;
  private readonly fetchImplementation: FetchImplementation;
  private readonly createCorrelationId: () => string;
  private readonly requestTimeoutMs: number;

  constructor(options: ApiClientOptions) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl);
    this.identity = options.identity;
    this.fetchImplementation = options.fetchImplementation ?? fetch;
    this.createCorrelationId = options.createCorrelationId ?? (() => crypto.randomUUID());
    this.requestTimeoutMs = options.requestTimeoutMs ?? 8_000;
  }

  private async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const correlationId = this.createCorrelationId();
    const unsafeRequestIntegerPath = options.body === undefined ? null : firstUnsafeIntegerPath(options.body);
    if (unsafeRequestIntegerPath) {
      throw new LiveApiError({
        type: "urn:itemba-z:control-center:request-contract",
        title: "Transaction integer is unsafe",
        status: 400,
        code: "request_integer_unsafe",
        detail: `The transaction command contains a non-safe integer at ${unsafeRequestIntegerPath}.`,
        correlation_id: correlationId,
      });
    }
    const headers = buildIdentityHeaders(this.identity);
    headers.set("Accept", "application/json, application/problem+json");
    headers.set("X-Correlation-ID", correlationId);

    if (options.body !== undefined) headers.set("Content-Type", "application/json");
    if (options.idempotencyKey !== undefined) {
      if (!validIdempotencyKey(options.idempotencyKey)) {
        throw new LiveApiError({
          type: "urn:itemba-z:control-center:idempotency",
          title: "Idempotency key invalid",
          status: 400,
          code: "idempotency_key_invalid",
          detail: "Idempotency-Key must contain 16 to 128 characters.",
          correlation_id: correlationId,
        });
      }
      headers.set("Idempotency-Key", options.idempotencyKey);
    }

    let response: Response;
    try {
      response = await this.fetchImplementation(`${this.baseUrl}${path}`, {
        method: options.method ?? "GET",
        headers,
        body: options.body === undefined ? undefined : JSON.stringify(options.body),
        cache: "no-store",
        signal: AbortSignal.timeout(this.requestTimeoutMs),
      });
    } catch {
      throw new LiveApiError({
        type: "urn:itemba-z:control-center:upstream",
        title: "Live ERP unavailable",
        status: 503,
        code: "upstream_unavailable",
        detail: "The Control Center could not reach the live ERP service.",
        correlation_id: correlationId,
      });
    }

    const payload: unknown = await response.json().catch(() => undefined);
    const unsafeIntegerPath = firstUnsafeIntegerPath(payload);
    if (unsafeIntegerPath) {
      throw new LiveApiError({
        type: "urn:itemba-z:control-center:upstream-contract",
        title: "Live API integer is unsafe",
        status: 502,
        code: "upstream_integer_unsafe",
        detail: `The ERP response contains a non-safe integer at ${unsafeIntegerPath}.`,
        correlation_id: correlationId,
      });
    }
    if (!response.ok) throw new LiveApiError(problemFromPayload(response.status, payload, correlationId));
    return payload as T;
  }

  getWorkingContext(): Promise<WorkingContext> {
    return this.request("/v1/context");
  }

  listCustomers(): Promise<CustomerPage> {
    return this.request("/v1/customers?page_size=200");
  }

  getCustomerAccount(customerId: string): Promise<CustomerAccountDetail> {
    return this.request(`/v1/customers/${encodeURIComponent(customerId)}/account`);
  }

  scheduleCustomerCreditPolicy(customerId: string, command: ScheduleCreditPolicyCommand, idempotencyKey: string): Promise<CreditPolicy> {
    return this.request(`/v1/customers/${encodeURIComponent(customerId)}/credit-policies`, {
      method: "POST", body: command, idempotencyKey,
    });
  }

  listProducts(): Promise<ProductPage> {
    return this.request("/v1/products?page_size=200");
  }

  listSales(cursor?: string): Promise<SalePage> {
    const query = new URLSearchParams({ page_size: "100" });
    if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/sales?${query.toString()}`);
  }

  getSale(saleId: string): Promise<Sale> {
    return this.request(`/v1/sales/${encodeURIComponent(saleId)}`);
  }

  completeSale(command: CompleteSaleCommand, idempotencyKey: string): Promise<Sale> {
    return this.request("/v1/sales", { method: "POST", body: command, idempotencyKey });
  }

  reverseSale(saleId: string, command: ReverseSaleCommand, idempotencyKey: string): Promise<Sale> {
    return this.request(`/v1/sales/${encodeURIComponent(saleId)}/reversals`, {
      method: "POST",
      body: command,
      idempotencyKey,
    });
  }

  listReconciliationCases(status?: MobileReconciliationStatus, cursor?: string): Promise<MobileReconciliationPage> {
    const query = new URLSearchParams({ page_size: "100" });
    if (status) query.set("status", status);
    if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/mobile/reconciliation-cases?${query.toString()}`);
  }

  getReconciliationCase(caseId: string): Promise<MobileReconciliationCase> {
    return this.request(`/v1/mobile/reconciliation-cases/${encodeURIComponent(caseId)}`);
  }

  resolveReconciliationCase(caseId: string, command: ResolveMobileReconciliationCommand, idempotencyKey: string): Promise<MobileReconciliationCase> {
    return this.request(`/v1/mobile/reconciliation-cases/${encodeURIComponent(caseId)}/resolutions`, {
      method: "POST",
      body: command,
      idempotencyKey,
    });
  }

  listManagedDevices(cursor?: string): Promise<MobileDevicePage> {
    const query = new URLSearchParams({ page_size: "100" });
    if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/mobile/devices?${query.toString()}`);
  }

  changeManagedDeviceStatus(deviceId: string, command: ChangeMobileDeviceStatusCommand, idempotencyKey: string): Promise<MobileDevice> {
    return this.request(`/v1/mobile/devices/${encodeURIComponent(deviceId)}/status-changes`, {
      method: "POST", body: command, idempotencyKey,
    });
  }

  changeManagedDeviceAllocation(deviceId: string, command: ChangeMobileDeviceAllocationCommand, idempotencyKey: string): Promise<MobileDevice> {
    return this.request(`/v1/mobile/devices/${encodeURIComponent(deviceId)}/allocation-changes`, {
      method: "POST", body: command, idempotencyKey,
    });
  }
}
