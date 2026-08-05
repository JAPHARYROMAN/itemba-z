import { PeopleWorkbench } from "@/components/people/people-workbench";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadPeopleWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function HumanResourcesPage() { const snapshot = await loadPeopleWorkspace(); if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />; return <PeopleWorkbench workspace={snapshot.data} />; }
