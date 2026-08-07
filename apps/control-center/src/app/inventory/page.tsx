import { OperationsWorkbench } from "@/components/operations/operations-workbench";
import { InventoryControlWorkbench } from "@/components/inventory/inventory-control-workbench";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadInventoryControlWorkspace, loadOperationsWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function InventoryPage() { const [operations, control] = await Promise.all([loadOperationsWorkspace(), loadInventoryControlWorkspace()]); if (control.state === "unavailable") return <LiveUnavailable problem={control.problem} />; if (operations.state === "unavailable") return <LiveUnavailable problem={operations.problem} />; return <><InventoryControlWorkbench workspace={control.data} /><OperationsWorkbench mode="inventory" workspace={operations.data} /></>; }
