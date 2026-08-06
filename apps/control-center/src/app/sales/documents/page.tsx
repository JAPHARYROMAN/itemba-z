import type { Metadata } from "next";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { OperationsWorkbench } from "@/components/operations/operations-workbench";
import { SalesModuleNav } from "@/components/sales/sales-module-nav";
import { loadSalesDocumentsWorkspace } from "@/live-api/snapshots";
import styles from "@/components/sales/sales-home.module.css";

export const metadata: Metadata = { title: "Sales quotes and orders", description: "Prepare, approve, and fulfil governed customer documents." };
export const dynamic = "force-dynamic";

export default async function SalesDocumentsPage() {
  const snapshot = await loadSalesDocumentsWorkspace();
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;

  return (
    <div className={styles.page}>
      <SalesModuleNav permissions={snapshot.data.context.permissions} />
      <OperationsWorkbench mode="sales" view="documents" workspace={snapshot.data} />
    </div>
  );
}
