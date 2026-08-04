"use client";

import Link from "next/link";
import { useDeferredValue, useMemo, useState } from "react";
import { ArrowRight, Download, Filter, Plus, Search, SlidersHorizontal, TrendingDown, TrendingUp } from "lucide-react";
import type { ModuleData } from "@/domain/erp";
import { useLanguage } from "@/components/language-provider";
import { StatusPill } from "@/components/status-pill";

function csvCell(value: string): string { return `"${value.replaceAll('"', '""')}"`; }

export function ModulePageView({ module }: { module: ModuleData }) {
  const { t, l } = useLanguage();
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const deferredQuery = useDeferredValue(query);
  const statuses = useMemo(
    () => Array.from(new Map(module.records.map((record) => [record.status.label.en, record.status])).values()),
    [module.records],
  );
  const filtered = useMemo(() => module.records.filter((record) => {
    const matchesQuery = [record.id, record.primary.en, record.primary.sw, record.secondary, ...Object.values(record.values)].join(" ").toLowerCase().includes(deferredQuery.toLowerCase());
    return matchesQuery && (statusFilter === "all" || record.status.label.en === statusFilter);
  }), [deferredQuery, module.records, statusFilter]);

  function exportCsv() {
    const headers = ["ID", l({ en: "Name", sw: "Jina" }), ...module.columns.map((column) => l(column.label)), t("status")];
    const rows = filtered.map((record) => [record.id, l(record.primary), ...module.columns.map((column) => record.values[column.key] ?? ""), l(record.status.label)]);
    const csv = [headers, ...rows].map((row) => row.map(csvCell).join(",")).join("\n");
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
    const anchor = document.createElement("a"); anchor.href = url; anchor.download = `${module.key}-export.csv`; anchor.click(); URL.revokeObjectURL(url);
  }

  return (
    <div className="page-stack">
      <section className="page-heading module-heading">
        <div><p className="eyebrow">{l(module.eyebrow)}</p><h1>{l(module.title)}</h1><p>{l(module.description)}</p></div>
        <div className="page-actions"><button type="button" className="secondary-button" onClick={exportCsv}><Download size={17} />{t("export")}</button><Link className="primary-button" href={`/${module.key}/new`}><Plus size={17} />{l(module.primaryAction)}</Link></div>
      </section>

      <section className="metric-grid compact-metrics" aria-label={`${l(module.title)} ${t("metrics")}`}>
        {module.metrics.map((metric) => <article className="metric-card" key={metric.label.en}><div className="metric-card-top"><span className="metric-label">{l(metric.label)}</span>{metric.change ? <span className={`metric-change ${metric.direction}`}>{metric.direction === "down" ? <TrendingDown size={14} /> : <TrendingUp size={14} />}{metric.change}</span> : null}</div><strong>{metric.value}</strong><small>{l(metric.detail)}</small></article>)}
      </section>

      <aside className={`module-insight insight-${module.insight.tone}`}><span><SlidersHorizontal size={18} /></span><div><strong>{l(module.insight.title)}</strong><p>{l(module.insight.detail)}</p></div><ArrowRight size={17} /></aside>

      <section className="card records-card">
        <div className="records-toolbar"><div><h2>{l(module.title)}</h2><span>{module.records.length} {t("records")}</span></div><div className="table-tools"><label className="module-search"><Search size={17} /><span className="sr-only">{t("searchWithin")}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("searchWithin")} /></label><label className="status-filter"><Filter size={16} /><span className="sr-only">{t("filter")}</span><select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="all">{t("allStatuses")}</option>{statuses.map((item) => <option value={item.label.en} key={item.label.en}>{l(item.label)}</option>)}</select></label></div></div>
        <div className="table-scroll"><table className="data-table"><caption className="sr-only">{l(module.title)} {t("records")}</caption><thead><tr><th>{l(module.singular)}</th>{module.columns.map((column) => <th key={column.key} className={column.align === "right" ? "align-right" : ""}>{l(column.label)}</th>)}<th>{t("status")}</th><th><span className="sr-only">{t("openRecord")}</span></th></tr></thead><tbody>{filtered.map((record) => <tr key={record.id}><td><Link href={`/${module.key}/${record.id}`} className="record-primary"><span>{record.id}</span><strong>{l(record.primary)}</strong><small>{record.secondary}</small></Link></td>{module.columns.map((column) => <td key={column.key} className={column.align === "right" ? "align-right numeric" : ""}>{record.values[column.key]}</td>)}<td><StatusPill status={record.status} /></td><td><Link href={`/${module.key}/${record.id}`} className="row-link" aria-label={`${t("openRecord")} ${record.id}`}><ArrowRight size={17} /></Link></td></tr>)}</tbody></table></div>
        {filtered.length === 0 ? <div className="empty-table"><Search size={28} /><strong>{t("noResults")}</strong></div> : null}
        <div className="table-footer"><span>{t("showing")} <strong>{filtered.length}</strong> {t("of")} {module.records.length}</span><span>1 / 1</span></div>
      </section>
    </div>
  );
}
