import type { Metadata } from "next";
import { SalesHome } from "@/components/sales/sales-home";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSalesWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Sales", description: "Sales tasks, attention queues, and recent transactions." };
export const dynamic = "force-dynamic";

export default async function SalesPage({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  const { cursor } = await searchParams;
  const snapshot = await loadSalesWorkspace(cursor);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <SalesHome workspace={snapshot.data} />;
}
