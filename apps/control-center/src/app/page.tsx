import { DashboardView } from "@/components/dashboard-view";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadDashboardWorkspace } from "@/live-api/snapshots";

export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  const snapshot = await loadDashboardWorkspace();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <DashboardView workspace={snapshot.data} />;
}
