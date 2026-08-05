import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FinanceControls } from "@/components/finance/finance-controls";
import { LanguageProvider } from "@/components/language-provider";
import type { FinanceControlWorkspace } from "@/live-api/types";

const workspace: FinanceControlWorkspace = {
  context: { actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004", branch_name: "DSM", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main", currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["finance.journals.read", "finance.journals.manage", "finance.journals.post", "finance.cash.transfer", "finance.periods.read", "finance.periods.close"], master_data_version: 1, price_version: 1, catalog_snapshot_token: "00000000-0000-4000-8000-000000000006" },
  accounts: [
    { id: "00000000-0000-4000-8000-000000000007", code: "CASH", name: "Main till", type: "CASH", currency: "TZS", gl_account_id: "cash", active: true },
    { id: "00000000-0000-4000-8000-000000000008", code: "CRDB", name: "Current account", type: "BANK", currency: "TZS", gl_account_id: "bank", active: true },
  ],
  documents: [],
  periods: [{ id: "00000000-0000-4000-8000-000000000009", tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", starts_at: "2026-08-01T00:00:00Z", ends_at: "2026-09-01T00:00:00Z", open: true }],
  periodActions: [],
};

describe("FinanceControls", () => {
  it("renders permission-aware posting and fiscal close controls", () => {
    render(<LanguageProvider><FinanceControls workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Journals & fiscal close" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create draft" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Request close" })).toBeInTheDocument();
    expect(screen.getAllByRole("option", { name: "CASH · Main till" })).toHaveLength(2);
  });
});
