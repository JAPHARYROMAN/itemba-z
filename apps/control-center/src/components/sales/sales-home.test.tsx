import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { SalesHome } from "@/components/sales/sales-home";
import type { Sale, SalesWorkspace } from "@/live-api/types";

vi.mock("next/navigation", () => ({ usePathname: () => "/sales" }));

const scope = {
  tenant_id: "00000000-0000-4000-8000-000000000001",
  company_id: "00000000-0000-4000-8000-000000000002",
  branch_id: "00000000-0000-4000-8000-000000000003",
  warehouse_id: "00000000-0000-4000-8000-000000000004",
};

function sale(overrides: Partial<Sale> = {}): Sale {
  return {
    id: "00000000-0000-4000-8000-000000000101",
    scope,
    record_type: "SALE",
    kind: "CASH",
    status: "POSTED",
    customer_id: "00000000-0000-4000-8000-000000000010",
    currency: "TZS",
    subtotal_minor: 100_000,
    tax_minor: 18_000,
    total_minor: 118_000,
    payment_method: "CASH",
    receipt_reference: "RCT-2026-0001",
    fiscal_status: "FISCALIZED",
    created_by: "00000000-0000-4000-8000-000000000005",
    correlation_id: "00000000-0000-4000-8000-000000000006",
    document_at: "2026-08-06T09:30:00Z",
    received_at: "2026-08-06T09:30:00Z",
    accounting_at: "2026-08-06T09:30:00Z",
    accounting_time_basis: "SERVER_RECEIPT",
    created_at: "2026-08-06T09:30:00Z",
    lines: [],
    ...overrides,
  };
}

const workspace: SalesWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000005",
    ...scope,
    company_name: "Itemba Trading Co. Ltd",
    branch_name: "Dar es Salaam HQ",
    warehouse_name: "Main Warehouse",
    currency: "TZS",
    locale: "en-TZ",
    timezone: "Africa/Dar_es_Salaam",
    permissions: ["sales.read", "sales.complete", "sales.orders.manage", "sales.reverse", "customers.read", "customers.accounts.read", "customers.collections.post", "products.read", "operations.read"],
    master_data_version: 1,
    price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000007",
  },
  customers: [
    { id: "00000000-0000-4000-8000-000000000010", code: "GEN", name: "General Customer", status: "active", is_general_customer: true, credit_enabled: false, credit_limit_minor: 0, current_exposure_minor: 0, available_credit_minor: 0 },
  ],
  products: [],
  sales: [sale()],
  documents: [
    {
      id: "00000000-0000-4000-8000-000000000201",
      number: "SO-2026-0042",
      type: "SALES_ORDER",
      status: "APPROVED",
      party_type: "CUSTOMER",
      party_id: "00000000-0000-4000-8000-000000000010",
      currency: "TZS",
      total_minor: 118_000,
      reason: "Customer order approved",
      created_at: "2026-08-06T08:00:00Z",
      lines: [],
    },
  ],
  nextCursor: null,
};

describe("SalesHome", () => {
  beforeEach(() => {
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" }));
  });

  afterEach(() => cleanup());

  it("renders task-first actions and live business labels without primary-view UUIDs", () => {
    render(<LanguageProvider><SalesHome workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { level: 1, name: "Sales" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "New sale" })).toHaveAttribute("href", "/sales/new");
    expect(screen.getByRole("link", { name: /New quote/ })).toHaveAttribute("href", "/sales/documents#new-document");
    expect(screen.getByRole("link", { name: /Record payment/ })).toHaveAttribute("href", "/sales/payments");
    expect(screen.getByText("Ready to fulfil")).toBeInTheDocument();
    expect(screen.getByText("RCT-2026-0001")).toBeInTheDocument();
    expect(screen.getByText("General Customer")).toBeInTheDocument();
    expect(screen.queryByText(workspace.sales[0]!.id)).not.toBeInTheDocument();
    expect(screen.queryByText(workspace.sales[0]!.correlation_id)).not.toBeInTheDocument();
  });

  it("removes actions the server context does not permit", () => {
    const restricted = { ...workspace, context: { ...workspace.context, permissions: ["sales.read", "customers.read", "products.read"] } };
    render(<LanguageProvider><SalesHome workspace={restricted} /></LanguageProvider>);
    expect(screen.queryByRole("link", { name: "New sale" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Record payment/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Transactions" })).toBeInTheDocument();
  });

  it("renders the reference flow in Swahili", () => {
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "sw" }));
    render(<LanguageProvider><SalesHome workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { level: 1, name: "Mauzo" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Mauzo mapya" })).toBeInTheDocument();
    expect(screen.getByText("Inahitaji umakini wako")).toBeInTheDocument();
  });
});
