import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { LiveSaleDetail } from "@/components/live-sales/live-sale-detail";
import { reservePendingCommand, saleReversalPendingScope } from "@/live-api/pending-command";
import type { ReverseSaleCommand, SaleDetailWorkspace } from "@/live-api/types";

const navigation = vi.hoisted(() => ({ push: vi.fn(), refresh: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => navigation }));

const workspace: SaleDetailWorkspace = {
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
    permissions: ["sales.reverse"],
    master_data_version: 1,
    price_version: 1,
  },
  customers: [
    { id: "00000000-0000-4000-8000-000000000011", code: "C-011", name: "Kijiji Supermarket", status: "active", is_general_customer: false, credit_enabled: true, credit_limit_minor: 5_000_000, current_exposure_minor: 1_000_000, available_credit_minor: 4_000_000 },
  ],
  products: [
    { id: "00000000-0000-4000-8000-000000000020", code: "SKU-20", name: "Itemba Water 1.5L", unit: "CASE", currency: "TZS", unit_price_minor: 12_000, available_quantity: 10, price_version: 1, master_data_version: 1, tax_basis_points: 0 },
  ],
  sale: {
    id: "00000000-0000-4000-8000-000000000030",
    scope: {
      tenant_id: "00000000-0000-4000-8000-000000000001",
      company_id: "00000000-0000-4000-8000-000000000002",
      branch_id: "00000000-0000-4000-8000-000000000003",
      warehouse_id: "00000000-0000-4000-8000-000000000004",
    },
    record_type: "SALE",
    kind: "CASH",
    status: "POSTED",
    customer_id: "00000000-0000-4000-8000-000000000011",
    currency: "TZS",
    subtotal_minor: 12_000,
    tax_minor: 0,
    total_minor: 12_000,
    payment_method: "CASH",
    receipt_reference: "ITEMBA-RCP-000030",
    fiscal_status: "NOT_CONFIGURED",
    created_by: "00000000-0000-4000-8000-000000000005",
    correlation_id: "00000000-0000-4000-8000-000000000006",
    created_at: "2026-08-04T08:00:00Z",
    lines: [{
      id: "00000000-0000-4000-8000-000000000040",
      product_id: "00000000-0000-4000-8000-000000000020",
      quantity: 1,
      unit_price_minor: 12_000,
      subtotal_minor: 12_000,
      tax_minor: 0,
      total_minor: 12_000,
    }],
  },
};

describe("LiveSaleDetail", () => {
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

  it("labels internal receipts truthfully and reuses a lost-response reversal key after remount", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      void input;
      void init;
      return Response.json({ id: "00000000-0000-4000-8000-000000000031" }, { status: 201 });
    });
    fetchMock.mockRejectedValueOnce(new TypeError("connection lost after send"));
    vi.stubGlobal("fetch", fetchMock);

    const firstView = render(<LanguageProvider><LiveSaleDetail workspace={workspace} /></LanguageProvider>);
    expect(screen.getByText("Internal receipt reference")).toBeInTheDocument();
    expect(screen.getByText(/Never present the internal reference as a TRA fiscal receipt/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Reverse sale" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Reason for reversal" }), { target: { value: "Verified customer return" } });
    fireEvent.click(screen.getByRole("checkbox", { name: /I confirm that this sale must be reversed/ }));
    fireEvent.click(screen.getByRole("button", { name: "Post linked reversal" }));
    await screen.findByText(/same reversal reason retains its idempotency key/i);
    const firstKey = new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get("Idempotency-Key");
    firstView.unmount();

    render(<LanguageProvider><LiveSaleDetail workspace={workspace} /></LanguageProvider>);
    await screen.findByText("Unconfirmed reversal recovered");
    fireEvent.click(screen.getByRole("button", { name: "Restore exact reason" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /I confirm that this sale must be reversed/ }));
    fireEvent.click(screen.getByRole("button", { name: "Retry same reversal" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const recoveredKey = new Headers(fetchMock.mock.calls[1]?.[1]?.headers).get("Idempotency-Key");
    expect(recoveredKey).toBe(firstKey);
    await waitFor(() => expect(window.sessionStorage.length).toBe(0));
  });

  it.each([
    ["actor", { ...workspace.context, actor_id: "00000000-0000-4000-8000-000000000006" }],
    ["warehouse", { ...workspace.context, warehouse_id: "00000000-0000-4000-8000-000000000007", warehouse_name: "Secondary Warehouse" }],
  ])("does not restore another %s scope's pending reversal in the same tab", (_scopeName, context) => {
    const command: ReverseSaleCommand = { reason: "Verified customer return" };
    reservePendingCommand({
      storage: window.sessionStorage,
      scope: saleReversalPendingScope(workspace.context, workspace.sale.id),
      fingerprint: JSON.stringify(command),
      payload: command,
      createKey: () => "00000000-0000-4000-8000-000000000091",
    });

    const view = render(<LanguageProvider><LiveSaleDetail workspace={workspace} /></LanguageProvider>);
    expect(screen.getByText("Unconfirmed reversal recovered")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Restore exact reason" }));
    expect(screen.getByRole("textbox", { name: "Reason for reversal" })).toHaveValue(command.reason);
    view.rerender(<LanguageProvider><LiveSaleDetail workspace={{ ...workspace, context }} /></LanguageProvider>);
    expect(screen.queryByText("Unconfirmed reversal recovered")).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Reason for reversal" })).not.toBeInTheDocument();
    expect(window.sessionStorage.length).toBe(1);
  });
});
