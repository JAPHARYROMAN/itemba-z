import type { Metadata } from "next";
import { LiveSaleDetail } from "@/components/live-sales/live-sale-detail";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSaleDetail } from "@/live-api/snapshots";

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: { params: Promise<{ recordId: string }> }): Promise<Metadata> {
  const { recordId } = await params;
  return { title: `Live Sale · ${recordId.slice(0, 8)}` };
}

export default async function LiveSaleDetailPage({ params }: { params: Promise<{ recordId: string }> }) {
  const { recordId } = await params;
  const snapshot = await loadSaleDetail(recordId);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <LiveSaleDetail workspace={snapshot.data} />;
}
