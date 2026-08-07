import type { Metadata } from "next";
import { CustomerAccountDetail } from "@/components/customers/customer-account-detail";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadCustomerAccount } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Customer Account", description: "Authoritative customer receivables detail." };
export const dynamic = "force-dynamic";

export default async function CustomerPage({ params }: { params: Promise<{ customerId: string }> }) {
  const { customerId } = await params;
  const snapshot = await loadCustomerAccount(customerId);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <CustomerAccountDetail workspace={snapshot.data} />;
}
