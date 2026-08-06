import { LiveUnavailable } from "@/components/live-sales/live-state";
import { SupplierDirectory } from "@/components/suppliers/supplier-directory";
import { loadSupplierWorkspace } from "@/live-api/snapshots";

export const metadata = { title: "Suppliers", description: "Approved supplier directory and governed supplier changes." };

export const dynamic = "force-dynamic";

export default async function SuppliersPage() {
  const snapshot = await loadSupplierWorkspace();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <SupplierDirectory workspace={snapshot.data} />;
}
