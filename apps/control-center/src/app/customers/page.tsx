import type { Metadata } from "next";
import { CustomerAccounts } from "@/components/customers/customer-accounts";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadCustomerAccounts } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "Customer Accounts", description: "Live receivables and credit-control workspace." };
export const dynamic = "force-dynamic";

export default async function CustomersPage() {
  const snapshot = await loadCustomerAccounts();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <CustomerAccounts workspace={snapshot.data} />;
}
