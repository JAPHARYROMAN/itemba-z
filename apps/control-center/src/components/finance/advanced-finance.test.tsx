import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { AdvancedFinance } from "@/components/finance/advanced-finance";
import type { AdvancedFinanceWorkspace } from "@/live-api/types";

const generated = "2026-08-05T10:00:00Z";
const workspace: AdvancedFinanceWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000001",
    tenant_id: "00000000-0000-4000-8000-000000000002",
    company_id: "00000000-0000-4000-8000-000000000003",
    company_name: "Itemba",
    branch_id: "00000000-0000-4000-8000-000000000004",
    branch_name: "DSM",
    warehouse_id: "00000000-0000-4000-8000-000000000005",
    warehouse_name: "Main",
    currency: "TZS",
    locale: "en-TZ",
    timezone: "Africa/Dar_es_Salaam",
    permissions: ["finance.budgets.read", "finance.assets.read"],
    master_data_version: 1,
    price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
  },
  accounts: [
    {
      record_id: "00000000-0000-4000-8000-000000000007",
      id: "expense",
      tenant_id: "00000000-0000-4000-8000-000000000002",
      company_id: "00000000-0000-4000-8000-000000000003",
      code: "expense",
      name: "Expense",
      type: "EXPENSE",
      control_account: false,
      allow_manual_posting: true,
      status: "ACTIVE",
      created_at: generated,
    },
  ],
  budgets: [
    {
      id: "00000000-0000-4000-8000-000000000008",
      scope: {
        tenant_id: "00000000-0000-4000-8000-000000000002",
        company_id: "00000000-0000-4000-8000-000000000003",
        branch_id: "00000000-0000-4000-8000-000000000004",
        warehouse_id: "00000000-0000-4000-8000-000000000005",
      },
      name: "Operating budget",
      fiscal_year: 2026,
      currency: "TZS",
      status: "APPROVED",
      reason: "Approved annual plan",
      lines: [{ account_id: "expense", month: generated, amount_minor: 10000 }],
      created_by: "00000000-0000-4000-8000-000000000001",
      created_at: generated,
    },
  ],
  assets: [],
  purchaseInvoices: [],
};

describe("AdvancedFinance", () => {
  it("renders governed budget and fixed asset workspaces", () => {
    render(
      <LanguageProvider>
        <AdvancedFinance workspace={workspace} />
      </LanguageProvider>,
    );
    expect(
      screen.getByRole("heading", { name: "Budgets and fixed assets" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Operating budget")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Register asset" }),
    ).toBeInTheDocument();
  });

  it("exposes posted matched purchases for controlled capitalization", () => {
    render(<LanguageProvider><AdvancedFinance workspace={{...workspace,purchaseInvoices:[{id:"00000000-0000-4000-8000-000000000010",number:"SI-001",type:"SUPPLIER_INVOICE",status:"POSTED",party_type:"SUPPLIER",currency:"TZS",total_minor:10000,reason:"Matched supplier invoice",created_at:generated,lines:[{id:"00000000-0000-4000-8000-000000000011",product_id:"00000000-0000-4000-8000-000000000012",quantity:1,unit_price_minor:10000,amount_minor:10000}]}]}} /></LanguageProvider>);
    expect(screen.getByRole("button",{name:"Create purchase-linked asset"})).toBeInTheDocument();
  });
});
