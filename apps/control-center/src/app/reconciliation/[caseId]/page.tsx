import type { Metadata } from "next";
import { ReconciliationDetail } from "@/components/reconciliation/reconciliation-detail";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadReconciliationDetail } from "@/live-api/snapshots";

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: { params: Promise<{ caseId: string }> }): Promise<Metadata> {
  const { caseId } = await params;
  return { title: `Reconciliation · ${caseId.slice(0, 8)}` };
}

export default async function ReconciliationDetailPage({ params }: { params: Promise<{ caseId: string }> }) {
  const { caseId } = await params;
  const snapshot = await loadReconciliationDetail(caseId);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <ReconciliationDetail workspace={snapshot.data} />;
}
