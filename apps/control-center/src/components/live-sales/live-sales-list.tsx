"use client";

import Link from "next/link";
import { ArrowRight, Banknote, CircleDollarSign, Plus, ReceiptText, RotateCcw, Search, ShieldCheck } from "lucide-react";
import { useDeferredValue, useMemo, useState } from "react";
import type { SalesWorkspace } from "@/live-api/types";
import { compactId, formatMinorUnits, formatTimestamp } from "@/live-api/format";
import { grossOriginalSaleMinor } from "@/live-api/sales-metrics";
import { LiveBadge } from "@/components/live-sales/live-state";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";

export function LiveSalesList({ workspace }: { workspace: SalesWorkspace }) {
  const { locale, l } = useLanguage();
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const customers = useMemo(() => new Map(workspace.customers.map((customer) => [customer.id, customer])), [workspace.customers]);
  const filteredSales = useMemo(() => {
    const normalized = deferredQuery.trim().toLocaleLowerCase();
    if (!normalized) return workspace.sales;
    return workspace.sales.filter((sale) => {
      const customer = customers.get(sale.customer_id);
      return [sale.id, customer?.name, customer?.code, sale.kind, sale.status, sale.record_type]
        .filter(Boolean).join(" ").toLocaleLowerCase().includes(normalized);
    });
  }, [customers, deferredQuery, workspace.sales]);
  const originalSaleTotalMinor = grossOriginalSaleMinor(workspace.sales);
  const cashCount = workspace.sales.filter((sale) => sale.kind === "CASH" && sale.record_type === "SALE").length;
  const reversedCount = workspace.sales.filter((sale) => sale.status === "REVERSED" || sale.record_type === "REVERSAL").length;

  return (
    <div className="page-stack live-sales-page">
      <section className="page-heading module-heading">
        <div><div className="heading-badges"><LiveBadge context={workspace.context} /><span className="scope-chip">{workspace.context.company_name} · {workspace.context.branch_name}</span></div><h1>{l(text("Live sales", "Mauzo hai"))}</h1><p>{l(text("Immutable posted sales from the authenticated ERP scope. Prices, tax, stock and ledgers are controlled by the server.", "Mauzo yaliyothibitishwa kutoka upeo wa ERP. Bei, kodi, bidhaa na madaftari yanadhibitiwa na seva."))}</p></div>
        <div className="page-actions"><Link className="secondary-button" href="/sales/lifecycle">{l(text("Orders & collections", "Oda na makusanyo"))}</Link><Link className="primary-button" href="/sales/new"><Plus size={17} />{l(text("Complete sale", "Kamilisha mauzo"))}</Link></div>
      </section>

      <section className="metric-grid compact-metrics" aria-label={l(text("Live sales metrics", "Vipimo vya mauzo hai"))}>
        <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><ReceiptText size={18} /></span><span className="live-mini">LIVE</span></div><p>{l(text("Loaded transactions", "Miamala iliyopakiwa"))}</p><strong>{workspace.sales.length}</strong><small>{l(text("Current authenticated page", "Ukurasa wa sasa uliothibitishwa"))}</small></article>
        <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><CircleDollarSign size={18} /></span></div><p>{l(text("Gross original-sale value", "Thamani jumla ya mauzo asili"))}</p><strong>{originalSaleTotalMinor === null ? l(text("Unavailable", "Haipatikani")) : formatMinorUnits(originalSaleTotalMinor, workspace.context.currency, locale)}</strong><small>{originalSaleTotalMinor === null ? l(text("Safe total limit exceeded", "Kiwango salama cha jumla kimezidi")) : l(text("Reversal documents excluded", "Nyaraka za ubatilisho hazijajumuishwa"))}</small></article>
        <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><Banknote size={18} /></span></div><p>{l(text("Cash sales", "Mauzo ya fedha"))}</p><strong>{cashCount}</strong><small>{l(text("Payment recorded atomically", "Malipo yamerekodiwa kwa pamoja"))}</small></article>
        <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><RotateCcw size={18} /></span></div><p>{l(text("Reversed / reversal", "Yaliyobatilishwa / ubatilisho"))}</p><strong>{reversedCount}</strong><small>{l(text("Original records remain immutable", "Rekodi asili hazibadilishwi"))}</small></article>
      </section>

      <aside className="live-control-note"><ShieldCheck size={18} /><div><strong>{l(text("Authoritative live data", "Data hai rasmi"))}</strong><p>{l(text("This workspace never falls back to demonstration sales. If authentication or the API fails, transaction controls close safely.", "Eneo hili halitumii mauzo ya maonyesho. Uthibitisho au API ikishindwa, udhibiti wa miamala hufungwa kwa usalama."))}</p></div></aside>

      <section className="card records-card">
        <div className="records-toolbar"><div><h2>{l(text("Posted sales", "Mauzo yaliyochapishwa"))}</h2><span>{filteredSales.length} {l(text("records", "rekodi"))}</span></div><label className="module-search"><Search size={17} /><span className="sr-only">{l(text("Search live sales", "Tafuta mauzo hai"))}</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={l(text("Search customer, ID or status…", "Tafuta mteja, ID au hali…"))} /></label></div>
        {filteredSales.length ? <div className="table-scroll"><table className="data-table live-sales-table"><caption className="sr-only">{l(text("Live posted sales", "Mauzo hai yaliyochapishwa"))}</caption><thead><tr><th>{l(text("Sale", "Mauzo"))}</th><th>{l(text("Customer", "Mteja"))}</th><th>{l(text("Kind", "Aina"))}</th><th className="align-right">{l(text("Total", "Jumla"))}</th><th>{l(text("Posted", "Imechapishwa"))}</th><th>{l(text("Status", "Hali"))}</th><th><span className="sr-only">{l(text("Open", "Fungua"))}</span></th></tr></thead><tbody>{filteredSales.map((sale) => {
          const customer = customers.get(sale.customer_id);
          return <tr key={sale.id}><td><Link className="record-primary" href={`/sales/${sale.id}`}><span>{sale.record_type}</span><strong title={sale.id}>{compactId(sale.id)}</strong><small>{compactId(sale.correlation_id)}</small></Link></td><td><strong>{customer?.name ?? l(text("Customer unavailable", "Mteja hapatikani"))}</strong><small className="table-subline">{customer?.code ?? compactId(sale.customer_id)}</small></td><td><span className={`sale-kind kind-${sale.kind.toLowerCase()}`}>{sale.kind === "CASH" ? l(text("Cash", "Fedha")) : l(text("Credit", "Mkopo"))}</span></td><td className="align-right numeric">{formatMinorUnits(sale.total_minor, sale.currency, locale)}</td><td>{formatTimestamp(sale.created_at, locale, workspace.context.timezone)}</td><td><span className={`live-sale-status status-${sale.status.toLowerCase()}`}>{sale.status === "POSTED" ? l(text("Posted", "Imechapishwa")) : l(text("Reversed", "Imebatilishwa"))}</span></td><td><Link className="row-link" href={`/sales/${sale.id}`} aria-label={`${l(text("Open sale", "Fungua mauzo"))} ${sale.id}`}><ArrowRight size={17} /></Link></td></tr>;
        })}</tbody></table></div> : <div className="live-empty"><ReceiptText size={30} /><strong>{l(text("No live sales match this view", "Hakuna mauzo hai yanayolingana"))}</strong><p>{query ? l(text("Clear the search to see all loaded transactions.", "Futa utafutaji kuona miamala yote.")) : l(text("Complete the first controlled sale for this scope.", "Kamilisha mauzo ya kwanza yaliyodhibitiwa katika upeo huu."))}</p>{!query ? <Link className="primary-button" href="/sales/new"><Plus size={16} />{l(text("Complete first sale", "Kamilisha mauzo ya kwanza"))}</Link> : null}</div>}
        <div className="table-footer"><span>{l(text("Authenticated scope", "Upeo uliothibitishwa"))} · {workspace.context.branch_name}</span>{workspace.nextCursor ? <Link href={`/sales?cursor=${encodeURIComponent(workspace.nextCursor)}`}>{l(text("Next page", "Ukurasa unaofuata"))}<ArrowRight size={14} /></Link> : <span>{l(text("End of live page", "Mwisho wa ukurasa hai"))}</span>}</div>
      </section>
    </div>
  );
}
