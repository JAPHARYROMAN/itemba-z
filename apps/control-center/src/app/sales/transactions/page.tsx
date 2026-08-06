import type { Metadata } from "next";
import { LiveSalesList } from "@/components/live-sales/live-sales-list";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSalesRegisterWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Sales transactions", description: "Search and review posted sales transactions." };
export const dynamic = "force-dynamic";

export default async function SalesTransactionsPage({ searchParams }: { searchParams: Promise<{ cursor?: string; intent?: string; status?: string }> }) {
  const { cursor, intent, status } = await searchParams;
  const snapshot = await loadSalesRegisterWorkspace("transactions", cursor);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <LiveSalesList key={`${intent ?? "browse"}:${status ?? "ALL"}`} workspace={snapshot.data} intent={intent} initialStatus={status} />;
}
