import { DashboardView } from "@/components/dashboard-view";
import { getDashboard } from "@/data/erp-repository";

export default async function DashboardPage() {
  return <DashboardView data={await getDashboard()} />;
}
