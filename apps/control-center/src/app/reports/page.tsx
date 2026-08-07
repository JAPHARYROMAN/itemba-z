import { FinancialReports } from "@/components/reports/financial-reports";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadFinancialReportsWorkspace } from "@/live-api/snapshots";

export const dynamic = "force-dynamic";
export default async function ReportsPage() { const workspace = await loadFinancialReportsWorkspace(); if (workspace.state === "unavailable") return <LiveUnavailable problem={workspace.problem} />; return <FinancialReports workspace={workspace.data} />; }
