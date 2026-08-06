"use client";

import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  CircleAlert,
  FilePlus2,
  HandCoins,
  PackageCheck,
  Plus,
  ReceiptText,
  RotateCcw,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useMemo } from "react";
import { useLanguage } from "@/components/language-provider";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import type { SalesWorkspace } from "@/live-api/types";
import { salesCopy, type SalesCopyKey } from "@/lib/sales-copy";
import { canAccessSalesRoute, type SalesRoute } from "@/lib/sales-permissions";
import { saleBusinessReference, saleNeedsAttention, saleStatusPresentation } from "@/lib/sales-presentation";
import { SalesModuleNav } from "@/components/sales/sales-module-nav";
import styles from "./sales-home.module.css";

interface QuickAction {
  label: SalesCopyKey;
  detail: SalesCopyKey;
  href: string;
  route: SalesRoute;
  permission?: string;
  icon: LucideIcon;
}

const quickActions: QuickAction[] = [
  { label: "newQuote", detail: "startQuoteDetail", href: "/sales/documents#new-document", route: "documents", permission: "sales.orders.manage", icon: FilePlus2 },
  { label: "recordPayment", detail: "paymentDetail", href: "/sales/payments", route: "payments", icon: HandCoins },
  { label: "processReturn", detail: "returnDetail", href: "/sales/returns", route: "returns", icon: RotateCcw },
];

export function SalesHome({ workspace }: { workspace: SalesWorkspace }) {
  const { locale } = useLanguage();
  const copy = (key: SalesCopyKey) => salesCopy(locale, key);
  const permissions = workspace.context.permissions;
  const customers = useMemo(() => new Map(workspace.customers.map((customer) => [customer.id, customer])), [workspace.customers]);
  const recentSales = useMemo(
    () => [...workspace.sales]
      .sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at) || right.receipt_reference.localeCompare(left.receipt_reference))
      .slice(0, 8),
    [workspace.sales],
  );
  const readyToFulfil = useMemo(
    () => workspace.documents.filter((document) => document.type === "SALES_ORDER" && document.status === "APPROVED"),
    [workspace.documents],
  );
  const submittedDocuments = useMemo(
    () => workspace.documents.filter((document) => (document.type === "QUOTATION" || document.type === "SALES_ORDER") && document.status === "SUBMITTED"),
    [workspace.documents],
  );
  const exceptionIds = useMemo(
    () => new Set(workspace.sales.filter(saleNeedsAttention).map((sale) => sale.id)),
    [workspace.sales],
  );
  const availableActions = useMemo(
    () => quickActions.filter((action) => canAccessSalesRoute(permissions, action.route) && (!action.permission || permissions.includes(action.permission))),
    [permissions],
  );
  const hasAttention = readyToFulfil.length > 0 || submittedDocuments.length > 0 || exceptionIds.size > 0;

  return (
    <div className={styles.page}>
      <SalesModuleNav permissions={permissions} />

      <header className={styles.hero}>
        <div>
          <p className={styles.scope}>{workspace.context.company_name} · {workspace.context.branch_name}</p>
          <h1>{copy("sales")}</h1>
          <p>{copy("homeIntro")}</p>
        </div>
        {canAccessSalesRoute(permissions, "new") ? (
          <Link className={styles.primaryAction} href="/sales/new"><Plus size={19} aria-hidden="true" />{copy("newSale")}</Link>
        ) : null}
      </header>

      {availableActions.length ? (
        <section className={styles.quickSection} aria-labelledby="sales-quick-actions">
          <h2 id="sales-quick-actions">{copy("quickActions")}</h2>
          <div className={styles.quickGrid}>
            {availableActions.map((action) => {
              const Icon = action.icon;
              return (
                <Link key={action.label} href={action.href} className={styles.quickAction}>
                  <span className={styles.quickIcon}><Icon size={20} aria-hidden="true" /></span>
                  <span><strong>{copy(action.label)}</strong><small>{copy(action.detail)}</small></span>
                  <ArrowRight size={18} aria-hidden="true" />
                </Link>
              );
            })}
          </div>
        </section>
      ) : null}

      <section className={styles.attentionSection} aria-labelledby="sales-attention">
        <div className={styles.sectionHeading}>
          <div><h2 id="sales-attention">{copy("needsAttention")}</h2></div>
        </div>
        <div className={styles.attentionGrid}>
          {readyToFulfil.length ? (
            <AttentionCard icon={PackageCheck} title={copy("readyToFulfil")} detail={copy("readyToFulfilDetail")} count={readyToFulfil.length} href="/sales/documents#documents" tone="success" />
          ) : null}
          {submittedDocuments.length ? (
            <AttentionCard icon={ReceiptText} title={copy("submittedDocuments")} detail={copy("submittedDocumentsDetail")} count={submittedDocuments.length} href="/sales/documents#documents" tone="info" />
          ) : null}
          {exceptionIds.size ? (
            <AttentionCard icon={CircleAlert} title={copy("exceptions")} detail={copy("exceptionsDetail")} count={exceptionIds.size} href="/sales/transactions?status=EXCEPTION" tone="warning" />
          ) : null}
          {!hasAttention ? (
            <div className={styles.caughtUp}>
              <span><CheckCircle2 size={22} aria-hidden="true" /></span>
              <div><strong>{copy("caughtUp")}</strong><p>{copy("caughtUpDetail")}</p></div>
            </div>
          ) : null}
        </div>
      </section>

      <section className={styles.transactionsCard} aria-labelledby="recent-sales-heading">
        <div className={styles.sectionHeading}>
          <div><h2 id="recent-sales-heading">{copy("recentTransactions")}</h2><p>{copy("recentTransactionsDetail")}</p></div>
          <Link href="/sales/transactions">{copy("viewAll")}<ArrowRight size={16} aria-hidden="true" /></Link>
        </div>

        {recentSales.length ? (
          <div className={styles.tableRegion} role="region" aria-labelledby="recent-sales-heading" tabIndex={0}>
            <table className={styles.table}>
              <caption className="sr-only">{copy("recentTransactions")}</caption>
              <thead><tr><th>{copy("type")}</th><th>{copy("number")}</th><th>{copy("customer")}</th><th className={styles.alignRight}>{copy("amount")}</th><th>{copy("status")}</th><th>{copy("date")}</th><th><span className="sr-only">{copy("openTransaction")}</span></th></tr></thead>
              <tbody>
                {recentSales.map((sale) => {
                  const customer = customers.get(sale.customer_id);
                  const status = saleStatusPresentation(sale);
                  const reference = saleBusinessReference(sale);
                  const formattedAmount = formatMinorUnits(sale.total_minor, sale.currency, locale);
                  return (
                    <tr key={sale.id}>
                      <td data-label={copy("type")}><span className={styles.typeCell}>{sale.record_type === "REVERSAL" ? <RotateCcw size={16} aria-hidden="true" /> : <ReceiptText size={16} aria-hidden="true" />}<strong>{copy(sale.record_type === "REVERSAL" ? "reversal" : "sale")}</strong></span></td>
                      <td data-label={copy("number")}><Link className={styles.receiptLink} href={`/sales/${sale.id}`}>{reference}</Link><small>{copy(sale.kind === "CASH" ? "cash" : "credit")}</small></td>
                      <td data-label={copy("customer")}><strong>{customer?.name ?? customer?.code ?? "—"}</strong></td>
                      <td data-label={copy("amount")} className={styles.alignRight}><strong>{sale.record_type === "REVERSAL" ? `−${formattedAmount}` : formattedAmount}</strong></td>
                      <td data-label={copy("status")}><span className={`${styles.status} ${styles[`status${status.tone}`]}`}>{copy(status.label)}</span></td>
                      <td data-label={copy("date")}><time dateTime={sale.created_at}>{formatTimestamp(sale.created_at, locale, workspace.context.timezone)}</time></td>
                      <td><Link className={styles.rowAction} href={`/sales/${sale.id}`} aria-label={`${copy("openTransaction")} ${reference}`}><ArrowRight size={17} aria-hidden="true" /></Link></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <div className={styles.emptyState}>
            <ReceiptText size={28} aria-hidden="true" />
            <p>{copy("noTransactions")}</p>
            {canAccessSalesRoute(permissions, "new") ? <Link className={styles.secondaryAction} href="/sales/new">{copy("completeFirstSale")}</Link> : null}
          </div>
        )}
      </section>
    </div>
  );
}

function AttentionCard({ icon: Icon, title, detail, count, href, tone }: { icon: LucideIcon; title: string; detail: string; count: number; href: string; tone: "success" | "warning" | "info" }) {
  return (
    <Link className={`${styles.attentionCard} ${styles[`attention${tone}`]}`} href={href}>
      <span className={styles.attentionIcon}><Icon size={21} aria-hidden="true" /></span>
      <span><strong>{title}</strong><small>{detail}</small></span>
      <b>{count}</b>
      <ArrowRight size={18} aria-hidden="true" />
    </Link>
  );
}
