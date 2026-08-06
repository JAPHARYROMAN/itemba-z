import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { SupplierDirectory } from "@/components/suppliers/supplier-directory";
import type { SupplierWorkspace, WorkingContext } from "@/live-api/types";

const context: WorkingContext = {
  actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002",
  company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004",
  branch_name: "Mwanza", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main", currency: "TZS",
  locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["operations.read"], master_data_version: 1, price_version: 1,
  catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
};

describe("SupplierDirectory", () => {
  it("renders approved suppliers without requiring sourcing permissions", () => {
    const workspace: SupplierWorkspace = { context, commercial: null, suppliers: [{ id: "00000000-0000-4000-8000-000000000010", code: "SUP-01", name: "Mwanza Packaging", active: true, payment_terms_days: 30 }] };
    render(<LanguageProvider><SupplierDirectory workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Suppliers" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Mwanza Packaging/ })).toHaveAttribute("href", "/suppliers/00000000-0000-4000-8000-000000000010");
    expect(screen.queryByRole("heading", { name: "Supplier approval queue" })).not.toBeInTheDocument();
  });
});
