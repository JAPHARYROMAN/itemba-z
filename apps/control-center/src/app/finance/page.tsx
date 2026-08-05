import { BankingWorkbench } from "@/components/banking/banking-workbench";
import { FinanceControls } from "@/components/finance/finance-controls";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadBankingWorkspace, loadFinanceControlWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function FinancePage() { const [banking, controls] = await Promise.all([loadBankingWorkspace(), loadFinanceControlWorkspace()]); if (banking.state === "unavailable") return <LiveUnavailable problem={banking.problem} />; if (controls.state === "unavailable") return <LiveUnavailable problem={controls.problem} />; return <><FinanceControls workspace={controls.data} /><BankingWorkbench workspace={banking.data} /></>; }
