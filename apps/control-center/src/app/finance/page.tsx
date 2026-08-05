import { BankingWorkbench } from "@/components/banking/banking-workbench";
import { FinanceControls } from "@/components/finance/finance-controls";
import { AccountGovernance } from "@/components/finance/account-governance";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import {
  loadBankingWorkspace,
  loadFinanceControlWorkspace,
} from "@/live-api/snapshots";
import { AdvancedFinance } from "@/components/finance/advanced-finance";
import { loadAdvancedFinanceWorkspace } from "@/live-api/snapshots";
import { BudgetActual } from "@/components/finance/budget-actual";
import { TreasuryWorkbench } from "@/components/finance/treasury-workbench";
import { loadTreasuryWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function FinancePage() {
  const [banking, controls, advanced, treasury] = await Promise.all([
    loadBankingWorkspace(),
    loadFinanceControlWorkspace(),
    loadAdvancedFinanceWorkspace(),
    loadTreasuryWorkspace(),
  ]);
  if (banking.state === "unavailable")
    return <LiveUnavailable problem={banking.problem} />;
  if (controls.state === "unavailable")
    return <LiveUnavailable problem={controls.problem} />;
  if (advanced.state === "unavailable")
    return <LiveUnavailable problem={advanced.problem} />;
  if (treasury.state === "unavailable")
    return <LiveUnavailable problem={treasury.problem} />;
  return (
    <>
      <AccountGovernance workspace={controls.data} />
      <FinanceControls workspace={controls.data} />
      <AdvancedFinance workspace={advanced.data} />
      <BudgetActual workspace={advanced.data} />
      <TreasuryWorkbench workspace={treasury.data} />
      <BankingWorkbench workspace={banking.data} />
    </>
  );
}
