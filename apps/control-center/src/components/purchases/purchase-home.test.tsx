import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { PurchaseHome } from "@/components/purchases/purchase-home";
import type { PurchaseWorkspace } from "@/live-api/types";

vi.mock("next/navigation", () => ({ usePathname: () => "/purchases" }));

const workspace = {
  context: {
    actor_id: "actor-1", tenant_id: "tenant-1", company_id: "company-1", branch_id: "branch-1", warehouse_id: "warehouse-1",
    company_name: "Itemba Trading Co. Ltd", branch_name: "Dar es Salaam HQ", warehouse_name: "Main Warehouse",
    currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam",
    permissions: ["operations.read", "products.read", "purchases.requests.manage", "purchases.orders.manage", "purchases.receive", "purchases.invoices.post", "purchases.payments.post"],
    master_data_version: 1, price_version: 1, catalog_snapshot_token: "catalog-1",
  },
  documents: [{ id: "purchase-id-should-not-render", number: "PO-2026-0042", type: "PURCHASE_ORDER", status: "APPROVED", party_type: "SUPPLIER", party_id: "supplier-1", currency: "TZS", total_minor: 118000, reason: "Approved supplier replenishment", created_at: "2026-08-07T08:00:00Z", lines: [] }],
  products: [], suppliers: [],
} as PurchaseWorkspace;

describe("PurchaseHome", () => {
  beforeEach(() => window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" })));
  afterEach(() => cleanup());

  it("renders task-first procure-to-pay actions without exposing internal ids", () => {
    render(<LanguageProvider><PurchaseHome workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { level: 1, name: "Purchases" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "New request" })).toHaveAttribute("href", "/purchases/requests");
    expect(screen.getByRole("link", { name: /Receive goods/ })).toHaveAttribute("href", "/purchases/receipts");
    expect(screen.getByText("PO-2026-0042")).toBeInTheDocument();
    expect(screen.queryByText("purchase-id-should-not-render")).not.toBeInTheDocument();
  });

  it("removes task actions that are not permitted", () => {
    const restricted = { ...workspace, context: { ...workspace.context, permissions: ["operations.read"] } };
    render(<LanguageProvider><PurchaseHome workspace={restricted} /></LanguageProvider>);
    expect(screen.queryByRole("link", { name: "New request" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Pay supplier/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Overview" })).toBeInTheDocument();
  });
});
