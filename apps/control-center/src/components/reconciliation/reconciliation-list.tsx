"use client";

import Link from "next/link";
import { ArrowRight, CheckCircle2, Clock3, FileWarning, Search, ShieldCheck } from "lucide-react";
import { useDeferredValue, useMemo, useState } from "react";
import { LiveBadge } from "@/components/live-sales/live-state";
import { useLanguage } from "@/components/language-provider";
import { compactId, formatTimestamp } from "@/live-api/format";
import type { MobileReconciliationCase, ReconciliationWorkspace } from "@/live-api/types";
import { text } from "@/lib/i18n";

function evidenceSearch(value: MobileReconciliationCase): string {
  return [value.id, value.device_id, value.client_transaction_id, value.failure_code, JSON.stringify(value.command)].join(" ").toLowerCase();
}

export function ReconciliationList({ workspace }: { workspace: ReconciliationWorkspace }) {
  const { locale, l } = useLanguage();
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query.trim().toLowerCase());
  const visible = useMemo(() => deferredQuery ? workspace.cases.filter((item) => evidenceSearch(item).includes(deferredQuery)) : workspace.cases, [deferredQuery, workspace.cases]);
  const openCount = workspace.cases.filter((item) => item.status === "OPEN").length;
  const resolvedCount = workspace.cases.filter((item) => item.status === "RESOLVED").length;

  return <div className="page-stack reconciliation-page">
    <section className="page-heading module-heading">
      <div><div className="heading-badges"><LiveBadge context={workspace.context} /><span className="scope-chip">{workspace.context.company_name} · {workspace.context.branch_name}</span></div><h1>{l(text("Offline reconciliation", "Upatanisho wa mauzo nje ya mtandao"))}</h1><p>{l(text("Investigate commands that could not be posted from immutable historical evidence. Dispositions are recorded separately and never create accounting effects implicitly.", "Chunguza amri ambazo hazikuweza kuchapishwa kutoka ushahidi wa kihistoria usiobadilika. Maamuzi yanarekodiwa tofauti na hayatengenezi athari za uhasibu moja kwa moja."))}</p></div>
    </section>

    <section className="metric-grid reconciliation-metrics" aria-label={l(text("Reconciliation metrics", "Vipimo vya upatanisho"))}>
      <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><FileWarning size={18} /></span><span className="live-mini">LIVE</span></div><p>{l(text("Loaded cases", "Kesi zilizopakiwa"))}</p><strong>{workspace.cases.length}</strong><small>{l(text("Exact authenticated scope", "Upeo halisi uliothibitishwa"))}</small></article>
      <article className="metric-card"><div className="metric-card-top"><span className="metric-icon warning-icon"><Clock3 size={18} /></span></div><p>{l(text("Open review", "Mapitio yaliyo wazi"))}</p><strong>{openCount}</strong><small>{l(text("No automatic retry or posting", "Hakuna kujaribu au kuchapisha kiotomatiki"))}</small></article>
      <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><CheckCircle2 size={18} /></span></div><p>{l(text("Resolved", "Zilizotatuliwa"))}</p><strong>{resolvedCount}</strong><small>{l(text("Immutable operator dispositions", "Maamuzi ya mtumiaji yasiyobadilika"))}</small></article>
    </section>

    <aside className="live-control-note"><ShieldCheck size={18} /><div><strong>{l(text("Evidence before action", "Ushahidi kabla ya hatua"))}</strong><p>{l(text("Confirm the device, client transaction, timestamp, and full command before recording a resolution. A posted-external disposition requires a reference.", "Thibitisha kifaa, muamala wa mteja, muda na amri kamili kabla ya kurekodi uamuzi. Uamuzi wa kuchapishwa nje unahitaji rejea."))}</p></div></aside>

    <section className="card records-card">
      <div className="records-toolbar"><div><h2>{l(text("Exception register", "Rejesta ya hitilafu"))}</h2><span>{visible.length} {l(text("cases", "kesi"))}</span></div><div className="reconciliation-tools"><label className="module-search"><Search size={17} /><span className="sr-only">{l(text("Search cases", "Tafuta kesi"))}</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={l(text("Search device or transaction…", "Tafuta kifaa au muamala…"))} /></label><div className="status-tabs" aria-label={l(text("Case status", "Hali ya kesi"))}><Link className={!workspace.status ? "active" : ""} href="/reconciliation">{l(text("All", "Zote"))}</Link><Link className={workspace.status === "OPEN" ? "active" : ""} href="/reconciliation?status=OPEN">{l(text("Open", "Wazi"))}</Link><Link className={workspace.status === "RESOLVED" ? "active" : ""} href="/reconciliation?status=RESOLVED">{l(text("Resolved", "Zilizotatuliwa"))}</Link></div></div></div>
      {visible.length ? <div className="table-scroll"><table className="data-table reconciliation-table"><caption className="sr-only">{l(text("Offline reconciliation cases", "Kesi za upatanisho nje ya mtandao"))}</caption><thead><tr><th>{l(text("Case", "Kesi"))}</th><th>{l(text("Device / transaction", "Kifaa / muamala"))}</th><th>{l(text("Client time", "Muda wa mteja"))}</th><th>{l(text("Failure", "Hitilafu"))}</th><th>{l(text("Status", "Hali"))}</th><th><span className="sr-only">{l(text("Open", "Fungua"))}</span></th></tr></thead><tbody>{visible.map((item) => <tr key={item.id}><td><Link className="record-primary" href={`/reconciliation/${item.id}`}><span>{l(text("Evidence case", "Kesi ya ushahidi"))}</span><strong title={item.id}>{compactId(item.id)}</strong><small>{formatTimestamp(item.created_at, locale, workspace.context.timezone)}</small></Link></td><td><strong>{compactId(item.device_id)}</strong><small className="table-subline">{compactId(item.client_transaction_id)}</small></td><td>{formatTimestamp(item.client_timestamp, locale, workspace.context.timezone)}</td><td><code className="failure-code">{item.failure_code}</code></td><td><span className={`reconciliation-status status-${item.status.toLowerCase()}`}>{item.status === "OPEN" ? l(text("Open", "Wazi")) : l(text("Resolved", "Imetatuliwa"))}</span></td><td><Link className="row-link" href={`/reconciliation/${item.id}`} aria-label={`${l(text("Open case", "Fungua kesi"))} ${item.id}`}><ArrowRight size={17} /></Link></td></tr>)}</tbody></table></div> : <div className="live-empty"><CheckCircle2 size={30} /><strong>{l(text("No reconciliation cases match this view", "Hakuna kesi za upatanisho zinazolingana"))}</strong><p>{l(text("Change the status filter or search. An empty open register means no retained offline command currently awaits a disposition.", "Badilisha kichujio cha hali au utafutaji. Rejesta wazi ikiwa tupu inamaanisha hakuna amri iliyohifadhiwa inayosubiri uamuzi."))}</p></div>}
      <div className="table-footer"><span>{l(text("Authenticated scope", "Upeo uliothibitishwa"))} · {workspace.context.branch_name}</span>{workspace.nextCursor ? <Link href={`/reconciliation?${new URLSearchParams({ ...(workspace.status ? { status: workspace.status } : {}), cursor: workspace.nextCursor }).toString()}`}>{l(text("Next page", "Ukurasa unaofuata"))}<ArrowRight size={14} /></Link> : <span>{l(text("End of live page", "Mwisho wa ukurasa hai"))}</span>}</div>
    </section>
  </div>;
}
