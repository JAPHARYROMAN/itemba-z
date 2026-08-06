"use client";

import { useMemo, useState } from "react";
import { Search, Truck } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import type { SupplierWorkspace } from "@/live-api/types";

export function SupplierDirectory({ workspace }: { workspace: SupplierWorkspace }) {
  const { l } = useLanguage();
  const [query, setQuery] = useState("");
  const suppliers = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("en");
    return workspace.suppliers.filter((supplier) => !needle || supplier.code.toLocaleLowerCase("en").includes(needle) || supplier.name.toLocaleLowerCase("en").includes(needle));
  }, [query, workspace.suppliers]);
  const activeCount = workspace.suppliers.filter((supplier) => supplier.active).length;
  const pendingCount = workspace.commercial.revisions.filter((revision) => revision.entity_type === "SUPPLIER" && revision.status === "SUBMITTED").length;
  return <section className="page-stack">
    <header className="module-header"><div><p className="eyebrow">{l(text("Supplier master", "Data msingi ya wasambazaji"))}</p><h1>{l(text("Suppliers", "Wasambazaji"))}</h1><p>{l(text("Authoritative supplier records, payment terms, and governed pending revisions.", "Rekodi halali za wasambazaji, masharti ya malipo, na marekebisho yanayosubiri idhini."))}</p></div><LiveBadge context={workspace.context} /></header>
    <div className="metric-grid"><article className="metric-card"><span>{l(text("Active suppliers", "Wasambazaji hai"))}</span><strong>{activeCount}</strong></article><article className="metric-card"><span>{l(text("Pending approvals", "Idhini zinazosubiri"))}</span><strong>{pendingCount}</strong></article><article className="metric-card"><span>{l(text("Inactive suppliers", "Wasambazaji wasiotumika"))}</span><strong>{workspace.suppliers.length - activeCount}</strong></article></div>
    <section className="card records-card"><div className="records-toolbar"><div><h2>{l(text("Supplier directory", "Orodha ya wasambazaji"))}</h2><span>{suppliers.length}</span></div><label className="table-search"><Search size={16} /><span className="sr-only">{l(text("Search suppliers", "Tafuta wasambazaji"))}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={l(text("Code or name…", "Msimbo au jina…"))} /></label></div><div className="table-scroll"><table className="data-table"><thead><tr><th>{l(text("Code", "Msimbo"))}</th><th>{l(text("Supplier", "Msambazaji"))}</th><th>{l(text("Payment terms", "Masharti ya malipo"))}</th><th>{l(text("Status", "Hali"))}</th></tr></thead><tbody>{suppliers.map((supplier) => <tr key={supplier.id}><td><strong>{supplier.code}</strong></td><td>{supplier.name}</td><td>{supplier.payment_terms_days} {l(text("days", "siku"))}</td><td><span className={`status-pill status-${supplier.active ? "success" : "neutral"}`}><span aria-hidden="true" />{supplier.active ? l(text("Active", "Hai")) : l(text("Inactive", "Isiyotumika"))}</span></td></tr>)}</tbody></table></div>{suppliers.length === 0 ? <div className="search-empty"><Truck size={32} /><strong>{l(text("No suppliers match this search.", "Hakuna msambazaji anayelingana na utafutaji huu."))}</strong></div> : null}</section>
  </section>;
}
