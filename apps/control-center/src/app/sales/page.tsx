import type { Metadata } from "next";
import { LiveSalesList } from "@/components/live-sales/live-sales-list";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSalesWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Live Sales", description: "Authenticated ITEMBA-Z sales workspace." };
export const dynamic = "force-dynamic";

export default async function SalesPage({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  const { cursor } = await searchParams;
  const snapshot = await loadSalesWorkspace(cursor);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <LiveSalesList workspace={snapshot.data} />;
}
