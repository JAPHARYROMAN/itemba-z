"use client";

import Link from "next/link";
import { ArrowRight, CheckCircle2, ClipboardCheck, FilePlus2, PackageCheck, ReceiptText, Truck, WalletCards } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { PurchaseModuleNav } from "@/components/purchases/purchase-module-nav";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import type { PurchaseWorkspace } from "@/live-api/types";
import { canAccessPurchaseRoute, PURCHASE_ROUTE_HREFS, type PurchaseRoute } from "@/lib/purchase-permissions";
import { purchaseStatusLabel, purchaseStatusTone, purchaseTypeLabel, PURCHASE_ROUTE_BY_TYPE } from "@/lib/purchase-presentation";
import { text } from "@/lib/i18n";
import styles from "./purchases.module.css";

const quickActions: Array<{ route: PurchaseRoute; icon: LucideIcon; en: string; sw: string; detailEn: string; detailSw: string }> = [
  { route: "requests", icon: FilePlus2, en: "New request", sw: "Ombi jipya", detailEn: "Start an internal purchase need", detailSw: "Anza hitaji la ndani la ununuzi" },
  { route: "orders", icon: ClipboardCheck, en: "Create order", sw: "Unda oda", detailEn: "Order from an approved supplier", detailSw: "Agiza kwa msambazaji aliyeidhinishwa" },
  { route: "receipts", icon: Truck, en: "Receive goods", sw: "Pokea bidhaa", detailEn: "Match delivery to an approved order", detailSw: "Linganisha mzigo na oda iliyoidhinishwa" },
  { route: "bills", icon: ReceiptText, en: "Record bill", sw: "Rekodi ankara", detailEn: "Match a supplier bill to received goods", detailSw: "Linganisha ankara na bidhaa zilizopokelewa" },
  { route: "payments", icon: WalletCards, en: "Pay supplier", sw: "Lipa msambazaji", detailEn: "Settle a posted supplier bill", detailSw: "Lipa ankara ya msambazaji iliyochapishwa" },
  { route: "returns", icon: PackageCheck, en: "Return goods", sw: "Rudisha bidhaa", detailEn: "Correct a posted purchase with evidence", detailSw: "Sahihisha ununuzi uliochapishwa kwa ushahidi" },
];

export function PurchaseHome({ workspace }: { workspace: PurchaseWorkspace }) {
  const { locale, l } = useLanguage();
  const submitted = workspace.documents.filter((document) => document.status === "SUBMITTED");
  const openOrders = workspace.documents.filter((document) => document.type === "PURCHASE_ORDER" && document.status === "APPROVED");
  const readyBills = workspace.documents.filter((document) => document.type === "SUPPLIER_INVOICE" && document.status === "APPROVED");
  const recent = workspace.documents.slice(0, 8);
  const actions = quickActions.filter((action) => canAccessPurchaseRoute(workspace.context.permissions, action.route));
  return <div className={styles.page}>
    <PurchaseModuleNav permissions={workspace.context.permissions} />
    <header className={styles.hero}><div><p className={styles.scope}>{workspace.context.company_name} · {workspace.context.branch_name}</p><h1>{l(text("Purchases", "Manunuzi"))}</h1><p>{l(text("Move each purchase from approved need to supplier settlement with a clear source chain.", "Hamisha kila ununuzi kutoka hitaji lililoidhinishwa hadi malipo ya msambazaji kwa mnyororo wazi wa chanzo."))}</p></div>{canAccessPurchaseRoute(workspace.context.permissions, "requests") ? <Link className={styles.primaryAction} href={PURCHASE_ROUTE_HREFS.requests}><FilePlus2 size={18} />{l(text("New request", "Ombi jipya"))}</Link> : null}</header>
    {actions.length ? <section className={styles.quickSection} aria-labelledby="purchase-actions"><h2 id="purchase-actions">{l(text("Start a task", "Anza jukumu"))}</h2><div className={styles.quickGrid}>{actions.map(({ route, icon: Icon, en, sw, detailEn, detailSw }) => <Link key={route} className={styles.quickAction} href={PURCHASE_ROUTE_HREFS[route]}><span className={styles.quickIcon}><Icon size={20} /></span><span><strong>{l(text(en, sw))}</strong><small>{l(text(detailEn, detailSw))}</small></span><ArrowRight size={18} /></Link>)}</div></section> : null}
    <section className={styles.attentionSection}><div className={styles.sectionHeading}><div><h2>{l(text("Needs attention", "Inahitaji uangalizi"))}</h2></div></div><div className={styles.attentionGrid}>{submitted.length ? <Attention href="/purchases/requests?status=SUBMITTED" icon={ClipboardCheck} count={submitted.length} title={l(text("Awaiting approval", "Inasubiri idhini"))} detail={l(text("Submitted purchase records need an independent decision.", "Rekodi za ununuzi zilizowasilishwa zinahitaji uamuzi huru."))} /> : null}{openOrders.length ? <Attention href="/purchases/receipts" icon={Truck} count={openOrders.length} title={l(text("Ready to receive", "Tayari kupokelewa"))} detail={l(text("Approved orders are waiting for delivery evidence.", "Oda zilizoidhinishwa zinasubiri ushahidi wa upokeaji."))} /> : null}{readyBills.length ? <Attention href="/purchases/bills" icon={ReceiptText} count={readyBills.length} title={l(text("Bills ready to post", "Ankara tayari kuchapishwa"))} detail={l(text("Approved supplier bills are waiting for posting.", "Ankara zilizoidhinishwa zinasubiri kuchapishwa."))} /> : null}{!submitted.length && !openOrders.length && !readyBills.length ? <div className={styles.caughtUp}><span><CheckCircle2 size={22} /></span><div><strong>{l(text("Purchasing is caught up", "Manunuzi yamekamilika kwa sasa"))}</strong><p>{l(text("No approvals, receipts, or bills require action in this scope.", "Hakuna idhini, mapokezi, au ankara zinazohitaji hatua katika upeo huu."))}</p></div></div> : null}</div></section>
    <section className={styles.card}><div className={styles.sectionHeading}><div><h2>{l(text("Recent purchase activity", "Shughuli za karibuni za ununuzi"))}</h2><p>{l(text("The latest records across the complete procure-to-pay chain.", "Rekodi za hivi karibuni katika mzunguko mzima wa manunuzi hadi malipo."))}</p></div></div>{recent.length ? <div className={styles.tableRegion} tabIndex={0} role="region" aria-label={l(text("Recent purchase documents", "Nyaraka za karibuni za ununuzi"))}><table className={styles.table}><thead><tr><th>{l(text("Document", "Hati"))}</th><th>{l(text("Type", "Aina"))}</th><th>{l(text("Status", "Hali"))}</th><th className={styles.alignRight}>{l(text("Total", "Jumla"))}</th><th>{l(text("Created", "Imeundwa"))}</th></tr></thead><tbody>{recent.map((document) => { const route = PURCHASE_ROUTE_BY_TYPE[document.type]; const tone = purchaseStatusTone(document.status); return <tr key={document.id}><td><Link className={styles.recordLink} href={`/purchases/${document.id}`}>{document.number}</Link></td><td data-label={l(text("Type", "Aina"))}>{purchaseTypeLabel(document.type, locale)}</td><td data-label={l(text("Status", "Hali"))}><span className={`${styles.status} ${styles[tone]}`}>{purchaseStatusLabel(document.status, locale)}</span></td><td data-label={l(text("Total", "Jumla"))} className={styles.alignRight}>{formatMinorUnits(document.total_minor, document.currency, locale)}</td><td data-label={l(text("Created", "Imeundwa"))}><time dateTime={document.created_at}>{formatTimestamp(document.created_at, locale, workspace.context.timezone)}</time>{route ? <small className={styles.meta}><Link href={PURCHASE_ROUTE_HREFS[route]}>{l(text("Open task", "Fungua jukumu"))}</Link></small> : null}</td></tr>; })}</tbody></table></div> : <div className={styles.empty}><ClipboardCheck size={30} /><strong>{l(text("No purchase activity yet", "Hakuna shughuli ya ununuzi bado"))}</strong><p>{l(text("Create a purchase request to begin a governed source chain.", "Unda ombi la ununuzi kuanza mnyororo wa chanzo uliodhibitiwa."))}</p></div>}</section>
  </div>;
}

function Attention({ href, icon: Icon, count, title, detail }: { href: string; icon: LucideIcon; count: number; title: string; detail: string }) {
  return <Link className={styles.attentionCard} href={href}><span className={styles.attentionIcon}><Icon size={21} /></span><span><strong>{title}</strong><small>{detail}</small></span><b>{count}</b><ArrowRight size={18} /></Link>;
}
