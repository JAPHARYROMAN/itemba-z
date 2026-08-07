import type { Metadata } from "next";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { PurchaseHome } from "@/components/purchases/purchase-home";
import { loadPurchaseWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Purchases", description: "Task-first procure-to-pay workspace." };
export const dynamic = "force-dynamic";

export default async function PurchasesPage() {
  const snapshot = await loadPurchaseWorkspace("overview");
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <PurchaseHome workspace={snapshot.data} />;
}
