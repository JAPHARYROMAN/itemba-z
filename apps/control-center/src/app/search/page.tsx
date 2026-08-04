import { SearchResultsView } from "@/components/search-results-view";
import { searchRecords } from "@/data/erp-repository";

export default async function SearchPage({ searchParams }: { searchParams: Promise<{ q?: string }> }) {
  const query = (await searchParams).q?.trim() ?? "";
  return <SearchResultsView query={query} results={await searchRecords(query)} />;
}
