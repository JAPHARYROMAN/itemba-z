import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CustomerAccounts } from "@/components/customers/customer-accounts";
import { LanguageProvider } from "@/components/language-provider";
import type { CustomerAccountsWorkspace } from "@/live-api/types";

const workspace: CustomerAccountsWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002",
    company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004",
    branch_name: "Mwanza", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main",
    currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["customers.accounts.read"],
    master_data_version: 1, price_version: 1, catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
  },
  customers: [{
    id: "00000000-0000-4000-8000-000000000010", code: "C-001", name: "Amani Stores", status: "active",
    is_general_customer: false, credit_enabled: true, credit_limit_minor: 5_000_000,
    current_exposure_minor: 1_250_000, available_credit_minor: 3_750_000,
  }],
};

describe("CustomerAccounts", () => {
  it("renders live exposure and links to authoritative account detail", () => {
    render(<LanguageProvider><CustomerAccounts workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Customers" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Amani Stores/ })).toHaveAttribute("href", "/customers/00000000-0000-4000-8000-000000000010");
    expect(screen.getAllByText("TZS 12,500.00").length).toBeGreaterThan(0);
  });
});
