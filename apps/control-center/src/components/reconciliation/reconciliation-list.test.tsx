import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { ReconciliationList } from "@/components/reconciliation/reconciliation-list";
import type { ReconciliationWorkspace } from "@/live-api/types";

const workspace: ReconciliationWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000001", tenant_id: "00000000-0000-4000-8000-000000000002",
    company_id: "00000000-0000-4000-8000-000000000003", company_name: "Itemba", branch_id: "00000000-0000-4000-8000-000000000004",
    branch_name: "Mwanza", warehouse_id: "00000000-0000-4000-8000-000000000005", warehouse_name: "Main",
    currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam", permissions: ["mobile.reconciliation.read"],
    master_data_version: 1, price_version: 1, catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
  },
  cases: [{
    id: "00000000-0000-4000-8000-000000000010", scope: { tenant_id: "00000000-0000-4000-8000-000000000002", company_id: "00000000-0000-4000-8000-000000000003", branch_id: "00000000-0000-4000-8000-000000000004", warehouse_id: "00000000-0000-4000-8000-000000000005" },
    status: "OPEN", device_id: "00000000-0000-4000-8000-000000000011", client_transaction_id: "00000000-0000-4000-8000-000000000012",
    client_timestamp: "2026-08-04T09:00:00Z", app_version: "1.0.0", master_data_version: 1, price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000006", failure_code: "offline_reconciliation_required",
    command: { offline: true }, created_by: "00000000-0000-4000-8000-000000000001", correlation_id: "00000000-0000-4000-8000-000000000013", created_at: "2026-08-04T10:00:00Z",
  }], nextCursor: null, status: "OPEN",
};

describe("ReconciliationList", () => {
  it("renders authoritative exception evidence without a posting action", () => {
    render(<LanguageProvider><ReconciliationList workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Offline reconciliation" })).toBeInTheDocument();
    expect(screen.getByText("offline_reconciliation_required")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /post/i })).not.toBeInTheDocument();
  });
});
