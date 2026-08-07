import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { SupplierDetail } from "@/components/suppliers/supplier-detail";
import { loadSupplierWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Supplier Profile", description: "Approved supplier terms and sourcing evidence." };
export const dynamic = "force-dynamic";

export default async function SupplierPage({ params }: { params: Promise<{ supplierId: string }> }) {
  const { supplierId } = await params;
  const snapshot = await loadSupplierWorkspace();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  const supplier = snapshot.data.suppliers.find((item) => item.id === supplierId);
  if (!supplier) notFound();
  return <SupplierDetail supplier={supplier} workspace={snapshot.data} />;
}
