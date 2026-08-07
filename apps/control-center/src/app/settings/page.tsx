import { SettingsWorkbench } from "@/components/settings/settings-workbench";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadConfigurationWorkspace } from "@/live-api/snapshots";
export const dynamic = "force-dynamic";
export default async function SettingsPage() { const snapshot = await loadConfigurationWorkspace(); if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />; return <SettingsWorkbench workspace={snapshot.data} />; }
