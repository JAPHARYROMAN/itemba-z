"use client";

import Link from "next/link";
import { ArrowLeft, Building2, FileCheck2, PackageCheck, ShieldCheck } from "lucide-react";
import { CommercialModuleNav } from "@/components/commercial/commercial-module-nav";
import { useLanguage } from "@/components/language-provider";
import { formatMinorUnits } from "@/live-api/format";
import type { SupplierSummary, SupplierWorkspace } from "@/live-api/types";
import { text } from "@/lib/i18n";
import styles from "@/components/commercial/commercial.module.css";

export function SupplierDetail({ supplier, workspace }: { supplier: SupplierSummary; workspace: SupplierWorkspace }) {
  const { locale, l } = useLanguage();
  const quotes = workspace.commercial?.quotes.filter((quote) => quote.supplier_id === supplier.id) ?? [];
  const awards = workspace.commercial?.awards.filter((award) => award.supplier_id === supplier.id) ?? [];
  const latestRevision = workspace.commercial?.revisions.find((revision) => revision.entity_type === "SUPPLIER" && revision.entity_id === supplier.id);
  return <div className={styles.page}>
    <CommercialModuleNav area="suppliers" permissions={workspace.context.permissions} />
    <Link className={styles.back} href="/suppliers"><ArrowLeft size={16} />{l(text("Supplier directory", "Orodha ya wasambazaji"))}</Link>
    <header className={styles.hero}><div><p className={styles.scope}>{supplier.code} · {workspace.context.company_name}</p><h1>{supplier.name}</h1><p>{l(text("Trading terms, sourcing history, and governed supplier evidence.", "Masharti ya biashara, historia ya manunuzi, na ushahidi uliodhibitiwa wa msambazaji."))}</p></div><span className={`${styles.status} ${supplier.active ? styles.success : styles.neutral}`}>{supplier.active ? l(text("Approved for use", "Ameidhinishwa kutumika")) : l(text("Inactive", "Hatumiki"))}</span></header>
    <section className={styles.metrics}><article className={styles.metric}><span><FileCheck2 size={17} />{l(text("Payment terms", "Masharti ya malipo"))}</span><strong>{supplier.payment_terms_days} {l(text("days", "siku"))}</strong></article><article className={styles.metric}><span><Building2 size={17} />{l(text("Supplier quotes", "Bei za msambazaji"))}</span><strong>{quotes.length}</strong></article><article className={styles.metric}><span><PackageCheck size={17} />{l(text("Sourcing awards", "Tuzo za manunuzi"))}</span><strong>{awards.length}</strong></article></section>
    <div className={styles.detailGrid}><div className={styles.stack}><section className={styles.card}><div className={styles.sectionHead}><div><h2>{l(text("Sourcing history", "Historia ya manunuzi"))}</h2><p>{l(text("Quotation evidence recorded against this supplier.", "Ushahidi wa bei uliorekodiwa kwa msambazaji huyu."))}</p></div></div>{quotes.length ? <div className={styles.tableRegion}><table className={styles.table}><thead><tr><th>{l(text("Reference", "Rejea"))}</th><th>{l(text("Decision state", "Hali ya uamuzi"))}</th><th className={styles.alignRight}>{l(text("Quoted total", "Jumla ya bei"))}</th></tr></thead><tbody>{quotes.map((quote) => <tr key={quote.id}><td>{quote.reference}</td><td data-label={l(text("Decision state", "Hali"))}>{quote.status === "SELECTED" ? l(text("Selected", "Imechaguliwa")) : quote.status === "SUBMITTED" ? l(text("Under review", "Inakaguliwa")) : quote.status === "REJECTED" ? l(text("Not selected", "Haijachaguliwa")) : l(text("Draft", "Rasimu"))}</td><td data-label={l(text("Quoted total", "Jumla"))} className={styles.alignRight}>{formatMinorUnits(quote.total_minor, quote.currency, locale)}</td></tr>)}</tbody></table></div> : <div className={styles.empty}><PackageCheck size={28} /><strong>{l(text("No sourcing activity yet", "Hakuna shughuli ya manunuzi bado"))}</strong><p>{l(text("Supplier quotations will appear here when they are recorded.", "Bei za msambazaji zitaonekana hapa zitakaporekodiwa."))}</p></div>}</section></div>
      <aside className={styles.stack}><section className={styles.card}><div className={styles.sectionHead}><div><h2>{l(text("Trading profile", "Wasifu wa biashara"))}</h2><p>{l(text("Current approved operating terms.", "Masharti ya sasa yaliyoidhinishwa."))}</p></div><ShieldCheck size={19} /></div><dl className={styles.details}><div><dt>{l(text("Supplier code", "Msimbo"))}</dt><dd>{supplier.code}</dd></div><div><dt>{l(text("Payment terms", "Masharti"))}</dt><dd>{supplier.payment_terms_days} {l(text("days", "siku"))}</dd></div><div><dt>{l(text("Status", "Hali"))}</dt><dd>{supplier.active ? l(text("Active", "Hai")) : l(text("Inactive", "Hatumiki"))}</dd></div>{latestRevision?.supplier?.tax_id ? <div><dt>{l(text("Tax number", "Namba ya kodi"))}</dt><dd>{latestRevision.supplier.tax_id}</dd></div> : null}</dl><details className={styles.systemDetails}><summary>{l(text("System details", "Maelezo ya mfumo"))}</summary><p>{l(text("Supplier record identifier", "Kitambulisho cha rekodi ya msambazaji"))}: {supplier.id}</p></details></section></aside></div>
  </div>;
}
