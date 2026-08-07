import type { Metadata } from "next";
import { ReconciliationList } from "@/components/reconciliation/reconciliation-list";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadReconciliationWorkspace } from "@/live-api/snapshots";
import type { MobileReconciliationStatus } from "@/live-api/types";

export const metadata: Metadata = { title: "Offline reconciliation", description: "Governed offline-sale exception register." };
export const dynamic = "force-dynamic";

export default async function ReconciliationPage({ searchParams }: { searchParams: Promise<{ cursor?: string; status?: string }> }) {
  const { cursor, status: requestedStatus } = await searchParams;
  const status: MobileReconciliationStatus | "" = requestedStatus === "OPEN" || requestedStatus === "RESOLVED" ? requestedStatus : "";
  const snapshot = await loadReconciliationWorkspace(status, cursor);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <ReconciliationList workspace={snapshot.data} />;
}
