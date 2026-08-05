import { OperationsWorkbench } from "@/components/operations/operations-workbench";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadOperationsWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function SalesLifecyclePage() { const snapshot = await loadOperationsWorkspace(); if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />; return <OperationsWorkbench mode="sales" workspace={snapshot.data} />; }
