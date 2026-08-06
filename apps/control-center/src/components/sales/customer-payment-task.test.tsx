import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { CustomerPaymentTask, parseTzsMajorUnits } from "@/components/sales/customer-payment-task";
import type { CustomerAccountDetail, CustomerCollection, Sale, SalesRegisterWorkspace } from "@/live-api/types";

const { loadAccount } = vi.hoisted(() => ({ loadAccount: vi.fn() }));

vi.mock("@/app/sales/payments/actions", () => ({ loadCustomerPaymentAccount: loadAccount }));
vi.mock("next/navigation", () => ({ usePathname: () => "/sales/payments" }));

const scope = {
  tenant_id: "00000000-0000-4000-8000-000000000001",
  company_id: "00000000-0000-4000-8000-000000000002",
  branch_id: "00000000-0000-4000-8000-000000000003",
  warehouse_id: "00000000-0000-4000-8000-000000000004",
};

const customerId = "00000000-0000-4000-8000-000000000010";
const invoiceSaleId = "00000000-0000-4000-8000-000000000101";

const sale: Sale = {
  id: invoiceSaleId,
  scope,
  record_type: "SALE",
  kind: "CREDIT",
  status: "POSTED",
  customer_id: customerId,
  currency: "TZS",
  subtotal_minor: 10_000_000,
  tax_minor: 1_800_000,
  total_minor: 11_800_000,
  receipt_reference: "INV-2026-0042",
  fiscal_status: "FISCALIZED",
  created_by: "00000000-0000-4000-8000-000000000005",
  correlation_id: "00000000-0000-4000-8000-000000000006",
  document_at: "2026-08-01T09:00:00Z",
  received_at: "2026-08-01T09:00:00Z",
  accounting_at: "2026-08-01T09:00:00Z",
  accounting_time_basis: "SERVER_RECEIPT",
  created_at: "2026-08-01T09:00:00Z",
  lines: [],
};

const workspace: SalesRegisterWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000005",
    ...scope,
    company_name: "Itemba Trading Co. Ltd",
    branch_name: "Dar es Salaam HQ",
    warehouse_name: "Main Warehouse",
    currency: "TZS",
    locale: "en-TZ",
    timezone: "Africa/Dar_es_Salaam",
    permissions: ["sales.read", "customers.collections.post", "customers.accounts.read"],
    master_data_version: 1,
    price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000007",
  },
  customers: [
    { id: "00000000-0000-4000-8000-000000000099", code: "GEN", name: "General Customer", status: "active", is_general_customer: true, credit_enabled: false, credit_limit_minor: 0, current_exposure_minor: 0, available_credit_minor: 0 },
    { id: customerId, code: "CUS-0042", name: "Amani Stores", status: "active", is_general_customer: false, credit_enabled: true, credit_limit_minor: 50_000_000, current_exposure_minor: 11_800_000, available_credit_minor: 38_200_000 },
  ],
  sales: [sale],
  nextCursor: null,
};

const account: CustomerAccountDetail = {
  customer: {
    id: customerId,
    code: "CUS-0042",
    ...scope,
    name: "Amani Stores",
    active: true,
    is_general_customer: false,
    legacy_credit_enabled: true,
    legacy_credit_limit_minor: 50_000_000,
  },
  active_policy: {
    id: "00000000-0000-4000-8000-000000000201",
    scope,
    customer_id: customerId,
    credit_enabled: true,
    credit_limit_minor: 50_000_000,
    payment_terms_days: 30,
    max_overdue_days: 30,
    risk_status: "STANDARD",
    reason: "Approved trading terms",
    effective_from: "2026-01-01T00:00:00Z",
    approved_by: "SYSTEM",
    created_at: "2026-01-01T00:00:00Z",
    correlation_id: "00000000-0000-4000-8000-000000000202",
  },
  scheduled_policies: [],
  aging: {
    ledger_balance_minor: 11_800_000,
    open_invoice_minor: 11_800_000,
    unapplied_credit_minor: 0,
    calculated_exposure_minor: 11_800_000,
    overdue_minor: 0,
    oldest_overdue_days: 0,
    reconciled: true,
    buckets: { current_minor: 11_800_000, days_1_30_minor: 0, days_31_60_minor: 0, days_61_90_minor: 0, days_over_90_minor: 0 },
  },
  open_items: [
    {
      id: "00000000-0000-4000-8000-000000000301",
      tenant_id: scope.tenant_id,
      company_id: scope.company_id,
      customer_id: customerId,
      kind: "INVOICE",
      source_type: "SALE",
      source_id: invoiceSaleId,
      amount_minor: 11_800_000,
      outstanding_minor: 11_800_000,
      currency: "TZS",
      document_at: "2026-08-01T09:00:00Z",
      due_at: "2026-08-31T09:00:00Z",
      occurred_at: "2026-08-01T09:00:00Z",
    },
  ],
};

const collection: CustomerCollection = {
  id: "00000000-0000-4000-8000-000000000401",
  customer_id: customerId,
  invoice_sale_id: invoiceSaleId,
  method: "MOBILE_MONEY",
  account_id: "1100-AR",
  amount_minor: 5_000_000,
  currency: "TZS",
  occurred_at: "2026-08-06T10:00:00Z",
  correlation_id: "00000000-0000-4000-8000-000000000402",
};

function response(body: unknown, status = 201): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

async function reachReview() {
  fireEvent.change(screen.getByLabelText("Customer account"), { target: { value: customerId } });
  await waitFor(() => expect(screen.getByRole("radio", { name: /INV-2026-0042/ })).toBeInTheDocument());
  fireEvent.click(screen.getByRole("radio", { name: /INV-2026-0042/ }));
  fireEvent.click(screen.getByRole("button", { name: "Continue to payment" }));
  fireEvent.change(screen.getByLabelText(/Amount received/), { target: { value: "50000.00" } });
  fireEvent.click(screen.getByRole("radio", { name: /Mobile money/ }));
  fireEvent.click(screen.getByRole("button", { name: "Review payment" }));
  expect(screen.getByRole("heading", { name: "Review before posting" })).toBeInTheDocument();
}

describe("CustomerPaymentTask", () => {
  beforeEach(() => {
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" }));
    window.sessionStorage.clear();
    loadAccount.mockReset();
    loadAccount.mockResolvedValue({ ok: true, account });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("uses a named customer and human-readable invoice details without exposing UUIDs", async () => {
    render(<LanguageProvider><CustomerPaymentTask workspace={workspace} /></LanguageProvider>);

    expect(screen.queryByRole("option", { name: /General Customer/ })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Customer account"), { target: { value: customerId } });

    await waitFor(() => expect(screen.getByText("INV-2026-0042")).toBeInTheDocument());
    expect(screen.getByText("TZS 118,000.00")).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent(invoiceSaleId);
    expect(document.body).not.toHaveTextContent(customerId);
  });

  it("converts TZS major units exactly and preserves one idempotency key across a retry", async () => {
    const fetchMock = vi.fn()
      .mockRejectedValueOnce(new Error("connection lost"))
      .mockResolvedValueOnce(response(collection));
    vi.stubGlobal("fetch", fetchMock);
    render(<LanguageProvider><CustomerPaymentTask workspace={workspace} /></LanguageProvider>);

    await reachReview();
    expect(screen.getByText("TZS 50,000.00")).toBeInTheDocument();
    expect(screen.getByText("TZS 68,000.00")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Post payment" }));

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("safely preserved this payment attempt"));
    const firstRequest = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(firstRequest[0]).toBe(`/api/live/customers/${customerId}/collections`);
    expect(JSON.parse(String(firstRequest[1].body))).toEqual({ invoice_sale_id: invoiceSaleId, method: "MOBILE_MONEY", amount_minor: 5_000_000, currency: "TZS" });
    const firstKey = new Headers(firstRequest[1].headers).get("Idempotency-Key");
    expect(firstKey).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Retry same payment" }));
    await waitFor(() => expect(screen.getByRole("heading", { name: "Payment posted" })).toBeInTheDocument());
    const secondRequest = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(new Headers(secondRequest[1].headers).get("Idempotency-Key")).toBe(firstKey);
    expect(screen.getByText("Amani Stores")).toBeInTheDocument();
    expect(screen.getByText("Mobile money")).toBeInTheDocument();
  });

  it("localizes the task and payment methods in Swahili", async () => {
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "sw" }));
    render(<LanguageProvider><CustomerPaymentTask workspace={workspace} /></LanguageProvider>);

    expect(screen.getByRole("heading", { name: "Rekodi malipo ya mteja" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Akaunti ya mteja"), { target: { value: customerId } });
    await waitFor(() => expect(screen.getByRole("radio", { name: /INV-2026-0042/ })).toBeInTheDocument());
    fireEvent.click(screen.getByRole("radio", { name: /INV-2026-0042/ }));
    fireEvent.click(screen.getByRole("button", { name: "Endelea kwenye malipo" }));
    expect(screen.getByRole("radio", { name: /Pesa kwa simu/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Hamisho la benki/ })).toBeInTheDocument();
  });
});

describe("parseTzsMajorUnits", () => {
  it("parses exact decimal input without binary floating-point arithmetic", () => {
    expect(parseTzsMajorUnits("50000.25")).toBe(5_000_025);
    expect(parseTzsMajorUnits("0.01")).toBe(1);
    expect(parseTzsMajorUnits("0")).toBeNull();
    expect(parseTzsMajorUnits("1.001")).toBeNull();
    expect(parseTzsMajorUnits("1,000")).toBeNull();
  });
});
