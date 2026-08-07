import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DashboardView } from "@/components/dashboard-view";
import { LanguageProvider } from "@/components/language-provider";
import type { DashboardWorkspace } from "@/live-api/types";

const workspace: DashboardWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000001",
    tenant_id: "00000000-0000-4000-8000-000000000002",
    company_id: "00000000-0000-4000-8000-000000000003",
    company_name: "Itemba Trading",
    branch_id: "00000000-0000-4000-8000-000000000004",
    branch_name: "Dar es Salaam",
    warehouse_id: "00000000-0000-4000-8000-000000000005",
    warehouse_name: "Main Warehouse",
    currency: "TZS",
    locale: "en-TZ",
    timezone: "Africa/Dar_es_Salaam",
    permissions: ["dashboard.read", "sales.complete"],
    master_data_version: 1,
    price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
  },
  dashboard: {
    as_of: "2026-08-06T10:00:00Z",
    metrics: [
      { key: "revenue", label: "Month-to-date revenue", value: { amount: "85000.00", currency: "TZS" }, trend_percent: null },
      { key: "net_profit", label: "Month-to-date net profit", value: { amount: "22000.00", currency: "TZS" }, trend_percent: null },
      { key: "cash_position", label: "Cash position", value: { amount: "104000.00", currency: "TZS" }, trend_percent: null },
      { key: "total_assets", label: "Total assets", value: { amount: "160000.00", currency: "TZS" }, trend_percent: null },
    ],
    alerts: [],
    pending_approvals: 3,
  },
};

describe("DashboardView", () => {
  it("renders only governed live values and permission-scoped actions", () => {
    render(<LanguageProvider><DashboardView workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "Executive overview" })).toBeInTheDocument();
    expect(screen.getByText("TZS 85,000.00")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Record sale/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Start purchase/ })).not.toBeInTheDocument();
    expect(screen.getByText("No governed alerts emitted")).toBeInTheDocument();
    expect(screen.queryByText("246")).not.toBeInTheDocument();
    expect(screen.queryByText("76%")).not.toBeInTheDocument();
  });
});
