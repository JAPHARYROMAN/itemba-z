import type { Metadata } from "next";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { PurchaseTaskWorkspace } from "@/components/purchases/purchase-task-workspace";
import { loadPurchaseWorkspace } from "@/live-api/snapshots";
export const metadata: Metadata = { title: "Purchase Orders" }; export const dynamic = "force-dynamic";
export default async function Page() { const snapshot = await loadPurchaseWorkspace("orders"); return snapshot.state === "unavailable" ? <LiveUnavailable problem={snapshot.problem} /> : <PurchaseTaskWorkspace workspace={snapshot.data} route="orders" />; }
