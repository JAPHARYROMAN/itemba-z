"use client";

import Link from "next/link";
import { ArrowLeft, ArrowRight, FileCheck2, Link2, PackageCheck } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { PurchaseModuleNav } from "@/components/purchases/purchase-module-nav";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import type { OperationDocument, PurchaseDocumentWorkspace } from "@/live-api/types";
import { text } from "@/lib/i18n";
import { PURCHASE_ROUTE_HREFS } from "@/lib/purchase-permissions";
import { purchaseStatusLabel, purchaseStatusTone, purchaseTypeLabel, PURCHASE_ROUTE_BY_TYPE } from "@/lib/purchase-presentation";
import styles from "./purchases.module.css";

export function PurchaseDocumentDetail({ workspace }: { workspace: PurchaseDocumentWorkspace }) {
  const { locale, l } = useLanguage();
  const { document } = workspace;
  const supplier = workspace.suppliers.find((item) => item.id === document.party_id);
  const sourceChain = buildSourceChain(document, workspace.documents);
  const route = PURCHASE_ROUTE_BY_TYPE[document.type];
  const tone = purchaseStatusTone(document.status);
  return <div className={styles.page}>
    <PurchaseModuleNav permissions={workspace.context.permissions} />
    <Link className={styles.back} href={route ? PURCHASE_ROUTE_HREFS[route] : "/purchases"}><ArrowLeft size={16} />{l(text("Back to purchase task", "Rudi kwenye jukumu la ununuzi"))}</Link>
    <header className={styles.hero}><div><p className={styles.scope}>{purchaseTypeLabel(document.type, locale)} · {workspace.context.company_name}</p><h1>{document.number}</h1><p>{document.reason}</p></div><span className={`${styles.status} ${styles[tone]}`}>{purchaseStatusLabel(document.status, locale)}</span></header>
    <div className={styles.detailGrid}><div className={styles.detailStack}><section className={`${styles.card} ${styles.detailCard}`}><h2>{l(text("Business evidence", "Ushahidi wa biashara"))}</h2><div className={styles.tableRegion}><table className={styles.table}><thead><tr><th>{l(text("Product", "Bidhaa"))}</th><th className={styles.alignRight}>{l(text("Quantity", "Kiasi"))}</th><th className={styles.alignRight}>{l(text("Unit price", "Bei kwa kipimo"))}</th><th className={styles.alignRight}>{l(text("Amount", "Kiasi cha fedha"))}</th></tr></thead><tbody>{document.lines.map((line) => { const product = workspace.products.find((item) => item.id === line.product_id); return <tr key={line.id}><td>{product?.name ?? l(text("Product record", "Rekodi ya bidhaa"))}<small className={styles.meta}>{product?.code}</small></td><td data-label={l(text("Quantity", "Kiasi"))} className={styles.alignRight}>{line.quantity}</td><td data-label={l(text("Unit price", "Bei"))} className={styles.alignRight}>{formatMinorUnits(line.unit_price_minor, document.currency, locale)}</td><td data-label={l(text("Amount", "Kiasi cha fedha"))} className={styles.alignRight}>{formatMinorUnits(line.amount_minor, document.currency, locale)}</td></tr>; })}</tbody></table></div></section>
      <section className={`${styles.card} ${styles.detailCard}`}><h2>{l(text("Source chain", "Mnyororo wa chanzo"))}</h2>{sourceChain.length ? <div className={styles.detailStack}>{sourceChain.map((source, index) => <Link key={source.id} className={styles.quickAction} href={`/purchases/${source.id}`}><span className={styles.quickIcon}><Link2 size={18} /></span><span><strong>{source.number}</strong><small>{purchaseTypeLabel(source.type, locale)} · {purchaseStatusLabel(source.status, locale)}</small></span>{index === 0 ? <ArrowRight size={18} /> : <FileCheck2 size={18} />}</Link>)}</div> : <div className={styles.empty}><PackageCheck size={28} /><strong>{l(text("This record starts the source chain", "Rekodi hii inaanza mnyororo wa chanzo"))}</strong></div>}</section></div>
      <aside className={styles.detailStack}><section className={`${styles.card} ${styles.detailCard}`}><h2>{l(text("Summary", "Muhtasari"))}</h2><dl className={styles.summary}><div><dt>{l(text("Supplier", "Msambazaji"))}</dt><dd>{supplier?.name ?? l(text("Internal", "Ndani"))}</dd></div><div><dt>{l(text("Total", "Jumla"))}</dt><dd>{formatMinorUnits(document.total_minor, document.currency, locale)}</dd></div><div><dt>{l(text("Created", "Imeundwa"))}</dt><dd>{formatTimestamp(document.created_at, locale, workspace.context.timezone)}</dd></div><div><dt>{l(text("Products", "Bidhaa"))}</dt><dd>{document.lines.length}</dd></div></dl>{route ? <Link className={styles.primaryAction} href={`${PURCHASE_ROUTE_HREFS[route]}#documents`}>{l(text("Open permitted actions", "Fungua hatua zinazoruhusiwa"))}</Link> : null}<details><summary>{l(text("System details", "Maelezo ya mfumo"))}</summary><code>{l(text("Document identifier", "Kitambulisho cha hati"))}: {document.id}</code>{document.source_document_id ? <><br /><code>{l(text("Source identifier", "Kitambulisho cha chanzo"))}: {document.source_document_id}</code></> : null}</details></section></aside></div>
  </div>;
}

function buildSourceChain(document: OperationDocument, documents: readonly OperationDocument[]): OperationDocument[] {
  const byId = new Map(documents.map((item) => [item.id, item]));
  const chain: OperationDocument[] = [];
  const seen = new Set<string>();
  let sourceId = document.source_document_id;
  while (sourceId && !seen.has(sourceId)) {
    seen.add(sourceId);
    const source = byId.get(sourceId);
    if (!source) break;
    chain.push(source);
    sourceId = source.source_document_id;
  }
  return chain;
}
