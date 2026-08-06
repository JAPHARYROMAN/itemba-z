import type { Metadata } from "next";
import { IntegrationOperations } from "@/components/integrations/integration-operations";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadIntegrationOperationsWorkspace } from "@/live-api/snapshots";
export const metadata: Metadata={title:"Integration Operations",description:"Governed provider routes, delivery reconciliation, replay, and recovery."};
export default async function IntegrationsPage(){const snapshot=await loadIntegrationOperationsWorkspace();if(snapshot.state==="unavailable")return <LiveUnavailable problem={snapshot.problem}/>;return <IntegrationOperations workspace={snapshot.data}/>;}
