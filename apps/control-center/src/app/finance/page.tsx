import { BankingWorkbench } from "@/components/banking/banking-workbench";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadBankingWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function FinancePage() { const snapshot = await loadBankingWorkspace(); if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />; return <BankingWorkbench workspace={snapshot.data} />; }
