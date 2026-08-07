import type { Metadata } from "next";
import { DeviceManagement } from "@/components/devices/device-management";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadDeviceManagementWorkspace } from "@/live-api/snapshots";

export const metadata: Metadata = { title: "POS devices", description: "Governed POS device and offline allocation operations." };
export const dynamic = "force-dynamic";

export default async function DevicesPage({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  const { cursor } = await searchParams;
  const snapshot = await loadDeviceManagementWorkspace(cursor);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <DeviceManagement workspace={snapshot.data} />;
}
