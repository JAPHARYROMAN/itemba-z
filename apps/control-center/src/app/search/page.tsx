import { SearchResultsView } from "@/components/search-results-view";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { loadGlobalSearch } from "@/live-api/snapshots";

export const dynamic = "force-dynamic";

export default async function SearchPage({ searchParams }: { searchParams: Promise<{ q?: string }> }) {
  const query = (await searchParams).q?.trim() ?? "";
  const snapshot = await loadGlobalSearch(query);
  if (snapshot.state === "unavailable") return <LiveUnavailable problem={snapshot.problem} />;
  return <SearchResultsView workspace={snapshot.data} />;
}
