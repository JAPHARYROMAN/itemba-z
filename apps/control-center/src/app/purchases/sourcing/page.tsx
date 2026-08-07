import type { Metadata } from "next";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { SourcingWorkbench } from "@/components/sourcing/sourcing-workbench";
import { loadCommercialWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Competitive Sourcing" };
export const dynamic = "force-dynamic";

export default async function SourcingPage() {
  const snapshot = await loadCommercialWorkspace();
  return snapshot.state === "unavailable"
    ? <LiveUnavailable problem={snapshot.problem} />
    : <SourcingWorkbench workspace={snapshot.data} />;
}
