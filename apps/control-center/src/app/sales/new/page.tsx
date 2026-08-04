import type { Metadata } from "next";
import { LiveSaleEntry } from "@/components/live-sales/live-sale-entry";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSalesBootstrap } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Complete Live Sale" };
export const dynamic = "force-dynamic";

export default async function NewLiveSalePage() {
  const snapshot = await loadSalesBootstrap();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <LiveSaleEntry bootstrap={snapshot.data} />;
}
