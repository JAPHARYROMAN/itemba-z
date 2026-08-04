"use client";

import Link from "next/link";
import { ArrowRight, Search } from "lucide-react";
import type { SearchResult } from "@/domain/erp";
import { useLanguage } from "@/components/language-provider";
import { StatusPill } from "@/components/status-pill";

export function SearchResultsView({ query, results }: { query: string; results: SearchResult[] }) {
  const { t, l } = useLanguage();
  return <div className="page-stack"><section className="page-heading"><div><p className="eyebrow">{t("searchResults")}</p><h1>{query ? `${t("searchFor")} “${query}”` : t("searchResults")}</h1><p>{results.length} {t("records")}</p></div></section><section className="card search-results">{results.length ? results.map((result) => <Link href={result.href} key={`${result.module}-${result.id}`}><span className="search-result-icon"><Search size={17} /></span><div><small>{l(result.moduleLabel)} · {result.id}</small><strong>{l(result.title)}</strong><span>{result.subtitle}</span></div><StatusPill status={result.status} compact /><ArrowRight size={17} /></Link>) : <div className="search-empty"><Search size={32} /><strong>{t("noSearchResults")}</strong></div>}</section></div>;
}
