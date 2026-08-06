import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { OperationsWorkbench } from "@/components/operations/operations-workbench";
import type { OperationDocument, OperationsWorkspace } from "@/live-api/types";

const customerId = "00000000-0000-4000-8000-000000000010";
const productId = "00000000-0000-4000-8000-000000000020";

function operation(overrides: Partial<OperationDocument> = {}): OperationDocument {
  return { id: "00000000-0000-4000-8000-000000000030", number: "SO-30", type: "SALES_ORDER", status: "DRAFT", party_type: "CUSTOMER", party_id: customerId, currency: "TZS", total_minor: 12_000, reason: "Customer order", created_at: "2026-08-06T00:00:00Z", lines: [], ...overrides };
}

function workspace(documents: OperationDocument[] = []): OperationsWorkspace {
  return {
    context: { actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004", branch_name: "HQ", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main", currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["operations.read", "sales.orders.manage", "sales.complete", "customers.read", "products.read"], master_data_version: 1, price_version: 1, catalog_snapshot_token: "00000000-0000-4000-8000-000000000006" },
    customers: [{ id: customerId, code: "C-10", name: "Customer", status: "active", is_general_customer: false, credit_enabled: true, credit_limit_minor: 100_000, current_exposure_minor: 0, available_credit_minor: 100_000 }],
    products: [{ id: productId, code: "SKU-20", name: "Water", unit: "CASE", currency: "TZS", unit_price_minor: 12_000, available_quantity: 10, price_version: 1, master_data_version: 1, tax_basis_points: 0 }],
    suppliers: [], documents, nextCursor: null,
  };
}

function idempotencyKeys(fetchMock: ReturnType<typeof vi.fn>): Array<string | null> {
  return fetchMock.mock.calls.map((call) => new Headers((call[1] as RequestInit | undefined)?.headers).get("Idempotency-Key"));
}

describe("OperationsWorkbench command recovery", () => {
  beforeEach(() => { window.sessionStorage.clear(); window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" })); });
  afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

  it("reuses the create identity after an ambiguous response", async () => {
    const created = operation({ status: "DRAFT", number: "SO-31" });
    const fetchMock = vi.fn().mockRejectedValueOnce(new TypeError("lost response")).mockResolvedValueOnce(Response.json(created, { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<LanguageProvider><OperationsWorkbench mode="sales" view="documents" workspace={workspace()} /></LanguageProvider>);
    fireEvent.change(screen.getByRole("combobox", { name: "Customer" }), { target: { value: customerId } });
    fireEvent.click(screen.getByRole("button", { name: "Create draft" }));
    await screen.findByText(/protected retry identity/);
    fireEvent.click(screen.getByRole("button", { name: "Create draft" }));
    await screen.findByText("SO-31");
    expect(idempotencyKeys(fetchMock)[0]).toBe(idempotencyKeys(fetchMock)[1]);
  });

  it("reuses the transition identity after an ambiguous response", async () => {
    const submitted = operation({ status: "SUBMITTED" });
    const fetchMock = vi.fn().mockRejectedValueOnce(new TypeError("lost response")).mockResolvedValueOnce(Response.json(submitted));
    vi.stubGlobal("fetch", fetchMock);
    render(<LanguageProvider><OperationsWorkbench mode="sales" view="documents" workspace={workspace([operation()])} /></LanguageProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Submitted" }));
    await screen.findByText(/protected retry identity/);
    fireEvent.click(screen.getByRole("button", { name: "Submitted" }));
    await waitFor(() => expect(idempotencyKeys(fetchMock)).toHaveLength(2));
    expect(idempotencyKeys(fetchMock)[0]).toBe(idempotencyKeys(fetchMock)[1]);
  });
});
