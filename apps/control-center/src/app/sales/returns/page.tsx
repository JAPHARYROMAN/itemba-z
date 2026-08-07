import type { Metadata } from "next";
import { LiveSalesList } from "@/components/live-sales/live-sales-list";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSalesRegisterWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Sales returns", description: "Find the original sale and begin an authorized correction." };
export const dynamic = "force-dynamic";

export default async function SalesReturnsPage({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  const { cursor } = await searchParams;
  const snapshot = await loadSalesRegisterWorkspace("returns", cursor);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <LiveSalesList key={`return:${cursor ?? "first"}`} workspace={snapshot.data} intent="return" />;
}
