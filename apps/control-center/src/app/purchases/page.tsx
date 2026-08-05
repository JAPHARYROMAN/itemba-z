import { OperationsWorkbench } from "@/components/operations/operations-workbench";
import { SourcingWorkbench } from "@/components/sourcing/sourcing-workbench";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadCommercialWorkspace, loadOperationsWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function PurchasesPage() { const [operations,commercial]=await Promise.all([loadOperationsWorkspace(),loadCommercialWorkspace()]);if(commercial.state==="unavailable")return <LiveUnavailable problem={commercial.problem}/>;if(operations.state==="unavailable")return <LiveUnavailable problem={operations.problem}/>;return <><SourcingWorkbench workspace={commercial.data}/><OperationsWorkbench mode="purchases" workspace={operations.data}/></>; }
