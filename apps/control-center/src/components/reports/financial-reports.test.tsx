import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { FinancialReports } from "@/components/reports/financial-reports";
import type { FinancialReportsWorkspace } from "@/live-api/types";

const generated = "2026-08-05T10:00:00Z";
const workspace: FinancialReportsWorkspace = {
  context: { actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004", branch_name: "DSM", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main", currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["reports.financial.read", "reports.financial.export"], master_data_version: 1, price_version: 1, catalog_snapshot_token: "00000000-0000-4000-8000-000000000006" },
  accounts: [{ record_id: "00000000-0000-4000-8000-000000000007", id: "cash", tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", code: "cash", name: "Cash", type: "ASSET", control_account: false, allow_manual_posting: false, status: "ACTIVE", created_at: generated }],
  from: "2026-08-01", to: "2026-08-05", asOf: "2026-08-05",
  trialBalance: { as_of: generated, currency: "TZS", lines: [{ account_id: "cash", code: "cash", name: "Cash", type: "ASSET", amount_minor: 0, debit_minor: 100, credit_minor: 0 }], total_debit_minor: 100, total_credit_minor: 100, balanced: true, generated_at: generated },
  profitAndLoss: { from: generated, to: generated, currency: "TZS", revenue: [], expenses: [], total_revenue_minor: 0, total_expense_minor: 0, net_profit_minor: 0, unclassified_minor: 0, generated_at: generated },
  balanceSheet: { as_of: generated, currency: "TZS", assets: [], liabilities: [], equity: [], total_assets_minor: 0, total_liabilities_minor: 0, total_equity_minor: 0, current_earnings_minor: 0, unclassified_minor: 0, balanced: true, generated_at: generated },
  cashFlow: { from: generated, to: generated, currency: "TZS", opening_cash_minor: 0, operating: { activity: "OPERATING", lines: [], net_minor: 0 }, investing: { activity: "INVESTING", lines: [], net_minor: 0 }, financing: { activity: "FINANCING", lines: [], net_minor: 0 }, unclassified: { activity: "UNCLASSIFIED", lines: [], net_minor: 0 }, net_change_minor: 0, closing_cash_minor: 0, reconciled: true, classified: true, generated_at: generated },
};

describe("FinancialReports", () => {
  it("renders the governed statement library and export control", () => {
    render(<LanguageProvider><FinancialReports workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Financial statements" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Trial balance" })).toBeInTheDocument();
		expect(screen.getByRole("combobox", { name: "Export format" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Export PDF" })).toBeInTheDocument();
    expect(screen.getByText("Balanced")).toBeInTheDocument();
  });
});
