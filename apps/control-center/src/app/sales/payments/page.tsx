import type { Metadata } from "next";
import { CustomerPaymentTask } from "@/components/sales/customer-payment-task";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadSalesPaymentWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = {
  title: "Record customer payment",
  description: "Allocate a customer payment to one open credit invoice.",
};

export const dynamic = "force-dynamic";

export default async function SalesPaymentsPage() {
  const snapshot = await loadSalesPaymentWorkspace();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <CustomerPaymentTask workspace={snapshot.data} />;
}
