import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { LiveSaleEntry } from "@/components/live-sales/live-sale-entry";
import { reservePendingCommand, saleCompletionPendingScope } from "@/live-api/pending-command";
import type { CompleteSaleCommand, SalesBootstrap } from "@/live-api/types";

const navigation = vi.hoisted(() => ({ push: vi.fn(), refresh: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => navigation }));

const bootstrap: SalesBootstrap = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000005",
    tenant_id: "00000000-0000-4000-8000-000000000001",
    company_id: "00000000-0000-4000-8000-000000000002",
    company_name: "Itemba Trading Co. Ltd",
    branch_id: "00000000-0000-4000-8000-000000000003",
    branch_name: "Dar es Salaam HQ",
    warehouse_id: "00000000-0000-4000-8000-000000000004",
    warehouse_name: "Main Warehouse",
    currency: "TZS",
    locale: "en-TZ",
    timezone: "Africa/Dar_es_Salaam",
    permissions: ["sales.complete"],
    master_data_version: 1,
  price_version: 1,
  catalog_snapshot_token: "00000000-0000-4000-8000-000000000001",
  },
  customers: [
    { id: "00000000-0000-4000-8000-000000000010", code: "GEN", name: "General Customer", status: "active", is_general_customer: true, credit_enabled: false, credit_limit_minor: 0, current_exposure_minor: 0, available_credit_minor: 0 },
    { id: "00000000-0000-4000-8000-000000000011", code: "C-011", name: "Kijiji Supermarket", status: "active", is_general_customer: false, credit_enabled: true, credit_limit_minor: 5_000_000, current_exposure_minor: 1_000_000, available_credit_minor: 4_000_000 },
  ],
  products: [
    { id: "00000000-0000-4000-8000-000000000020", code: "SKU-20", name: "Itemba Water 1.5L", unit: "CASE", currency: "TZS", unit_price_minor: 12_000, available_quantity: 10, price_version: 1, master_data_version: 1, tax_basis_points: 0 },
  ],
  documents: [],
};

describe("LiveSaleEntry", () => {
  beforeEach(() => {
    navigation.push.mockReset();
    navigation.refresh.mockReset();
    window.sessionStorage.clear();
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" }));
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("blocks General Customer credit and posts server-owned line inputs with an idempotency key", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ id: "00000000-0000-4000-8000-000000000099" }, { status: 201 });
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<LanguageProvider><LiveSaleEntry bootstrap={bootstrap} /></LanguageProvider>);

    fireEvent.click(screen.getByRole("button", { name: /Credit sale/ }));
    expect(screen.getByRole("option", { name: /General Customer/ })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: /Cash sale/ }));
    fireEvent.change(screen.getByRole("combobox", { name: "Customer" }), { target: { value: bootstrap.customers[1]?.id } });
    fireEvent.change(screen.getByRole("combobox", { name: "Payment method" }), { target: { value: "BANK_TRANSFER" } });
    fireEvent.click(screen.getByRole("button", { name: /Add one Itemba Water/ }));
    fireEvent.click(screen.getByRole("button", { name: "Review controlled posting" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm & post" }));

    await waitFor(() => expect(navigation.push).toHaveBeenCalledWith("/sales/00000000-0000-4000-8000-000000000099"));
    const [, init] = fetchMock.mock.calls[0] ?? [];
    const headers = new Headers(init?.headers);
    expect(headers.get("Idempotency-Key")).toMatch(/^[0-9a-f-]{36}$/);
    expect(JSON.parse(String(init?.body))).toEqual({
      customer_id: bootstrap.customers[1]?.id,
      kind: "CASH",
      payment_method: "BANK_TRANSFER",
      lines: [{ product_id: bootstrap.products[0]?.id, quantity: 1 }],
    });
  });

  it("reuses a lost-response key after remount, then allocates a new key after confirmed success", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ id: "00000000-0000-4000-8000-000000000099" }, { status: 201 });
    });
    fetchMock.mockRejectedValueOnce(new TypeError("connection lost after send"));
    vi.stubGlobal("fetch", fetchMock);

    const firstView = render(<LanguageProvider><LiveSaleEntry bootstrap={bootstrap} /></LanguageProvider>);
    fireEvent.change(screen.getByRole("combobox", { name: "Customer" }), { target: { value: bootstrap.customers[1]?.id } });
    fireEvent.click(screen.getByRole("button", { name: /Add one Itemba Water/ }));
    fireEvent.click(screen.getByRole("button", { name: "Review controlled posting" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm & post" }));
    await screen.findByText(/safely preserved this sale attempt/i);
    const firstKey = new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get("Idempotency-Key");
    firstView.unmount();

    const recoveredView = render(<LanguageProvider><LiveSaleEntry bootstrap={bootstrap} /></LanguageProvider>);
    await screen.findByText("Unconfirmed sale recovered");
    fireEvent.click(screen.getByRole("button", { name: "Restore exact sale" }));
    fireEvent.click(screen.getByRole("button", { name: "Retry same sale" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const recoveredKey = new Headers(fetchMock.mock.calls[1]?.[1]?.headers).get("Idempotency-Key");
    expect(recoveredKey).toBe(firstKey);
    await waitFor(() => expect(window.sessionStorage.length).toBe(0));
    recoveredView.unmount();

    render(<LanguageProvider><LiveSaleEntry bootstrap={bootstrap} /></LanguageProvider>);
    fireEvent.change(screen.getByRole("combobox", { name: "Customer" }), { target: { value: bootstrap.customers[1]?.id } });
    fireEvent.click(screen.getByRole("button", { name: /Add one Itemba Water/ }));
    fireEvent.click(screen.getByRole("button", { name: "Review controlled posting" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm & post" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const nextKey = new Headers(fetchMock.mock.calls[2]?.[1]?.headers).get("Idempotency-Key");
    expect(nextKey).not.toBe(firstKey);
  });

  it("uses an approved-order picker and locks its customer and quantities without exposing a UUID", () => {
    const orderId = "00000000-0000-4000-8000-000000000042";
    const orderBootstrap: SalesBootstrap = {
      ...bootstrap,
      documents: [{
        id: orderId,
        number: "SALESORDER-000042",
        type: "SALES_ORDER",
        status: "APPROVED",
        party_type: "CUSTOMER",
        party_id: bootstrap.customers[1]!.id,
        currency: "TZS",
        total_minor: 12_000,
        reason: "Approved customer order",
        created_at: "2026-08-06T08:00:00Z",
        lines: [{ id: "00000000-0000-4000-8000-000000000043", product_id: bootstrap.products[0]!.id, quantity: 2, unit_price_minor: 6_000, amount_minor: 12_000 }],
      }],
    };
    render(<LanguageProvider><LiveSaleEntry bootstrap={orderBootstrap} /></LanguageProvider>);
    fireEvent.change(screen.getByRole("combobox", { name: "Approved sales order (optional)" }), { target: { value: orderId } });
    expect(screen.getByRole("combobox", { name: "Customer" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "Customer" })).toHaveValue(bootstrap.customers[1]!.id);
    expect(screen.getByText("Customer and quantities are locked to the approved order.")).toBeInTheDocument();
    expect(screen.queryByText(orderId)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Add one Itemba Water/ })).toBeDisabled();
  });

  it.each([
    ["actor", { ...bootstrap.context, actor_id: "00000000-0000-4000-8000-000000000006" }],
    ["warehouse", { ...bootstrap.context, warehouse_id: "00000000-0000-4000-8000-000000000007", warehouse_name: "Secondary Warehouse" }],
  ])("does not restore another %s scope's pending sale in the same tab", (_scopeName, context) => {
    const command: CompleteSaleCommand = {
      customer_id: bootstrap.customers[1]!.id,
      kind: "CASH",
      payment_method: "CASH",
      lines: [{ product_id: bootstrap.products[0]!.id, quantity: 1 }],
    };
    reservePendingCommand({
      storage: window.sessionStorage,
      scope: saleCompletionPendingScope(bootstrap.context),
      fingerprint: JSON.stringify(command),
      payload: command,
      createKey: () => "00000000-0000-4000-8000-000000000090",
    });

    const view = render(<LanguageProvider><LiveSaleEntry bootstrap={bootstrap} /></LanguageProvider>);
    expect(screen.getByText("Unconfirmed sale recovered")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Restore exact sale" }));
    expect(screen.getByRole("combobox", { name: "Customer" })).toHaveValue(command.customer_id);
    view.rerender(<LanguageProvider><LiveSaleEntry bootstrap={{ ...bootstrap, context }} /></LanguageProvider>);
    expect(screen.queryByText("Unconfirmed sale recovered")).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Customer" })).toHaveValue("");
    expect(window.sessionStorage.length).toBe(1);
  });
});
