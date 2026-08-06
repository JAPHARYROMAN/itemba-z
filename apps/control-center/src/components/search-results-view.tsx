"use client";

import Link from "next/link";
import { ArrowRight, Search } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { StatusPill } from "@/components/status-pill";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import type { GlobalSearchWorkspace } from "@/live-api/types";

export function SearchResultsView({ workspace }: { workspace: GlobalSearchWorkspace }) {
  const { t, l } = useLanguage();
  const { query, results } = workspace;
  return <div className="page-stack"><section className="page-heading"><div><p className="eyebrow">{t("searchResults")}</p><h1>{query ? `${t("searchFor")} “${query}”` : t("searchResults")}</h1><p>{query.length < 2 ? l(text("Enter at least two characters to search live records.", "Andika angalau herufi mbili kutafuta rekodi hai.")) : `${results.length} ${t("records")} · ${workspace.searchedSources.map(l).join(", ")}`}</p></div><LiveBadge context={workspace.context} /></section><section className="card search-results" aria-live="polite">{results.length ? results.map((result) => <Link href={result.href} key={`${result.module}-${result.id}`}><span className="search-result-icon"><Search size={17} /></span><div><small>{l(result.moduleLabel)} · {result.id}</small><strong>{l(result.title)}</strong><span>{result.subtitle}</span></div><StatusPill status={result.status} compact /><ArrowRight size={17} /></Link>) : <div className="search-empty"><Search size={32} /><strong>{query.length < 2 ? l(text("Search live ERP records", "Tafuta rekodi hai za ERP")) : t("noSearchResults")}</strong></div>}</section></div>;
}
