import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { BankingWorkbench } from "@/components/banking/banking-workbench";
import { LanguageProvider } from "@/components/language-provider";
import type { BankingWorkspace } from "@/live-api/types";

const workspace: BankingWorkspace = {
  context: { actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004", branch_name: "DSM", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main", currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["finance.bank.read", "finance.bank.import", "finance.bank.match", "finance.bank.reconcile"], master_data_version: 1, price_version: 1, catalog_snapshot_token: "00000000-0000-4000-8000-000000000006" },
  accounts: [{ id: "00000000-0000-4000-8000-000000000007", code: "CRDB", name: "CRDB current", type: "BANK", currency: "TZS", gl_account_id: "bank-current", active: true }],
  statements: [{ id: "00000000-0000-4000-8000-000000000008", account_id: "00000000-0000-4000-8000-000000000007", external_reference: "AUG-2026", currency: "TZS", period_start: "2026-08-01T00:00:00Z", period_end: "2026-08-05T23:59:59Z", opening_minor: 100_000, closing_minor: 125_000, status: "IMPORTED", imported_by: "00000000-0000-4000-8000-000000000001", imported_at: "2026-08-05T10:00:00Z", correlation_id: "00000000-0000-4000-8000-000000000009", lines: [] }],
  nextCursor: null,
};

describe("BankingWorkbench", () => {
  it("renders live account and immutable statement evidence", () => {
    render(<LanguageProvider><BankingWorkbench workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Cash & bank reconciliation" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "CRDB · CRDB current" })).toBeInTheDocument();
    expect(screen.getByText("AUG-2026")).toBeInTheDocument();
    expect(screen.getByText("TZS 1,250.00")).toBeInTheDocument();
  });
});
