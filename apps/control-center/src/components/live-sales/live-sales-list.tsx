"use client";

import Link from "next/link";
import { ArrowRight, ReceiptText, RotateCcw, Search, X } from "lucide-react";
import { useDeferredValue, useMemo, useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { SalesModuleNav } from "@/components/sales/sales-module-nav";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import type { Sale, SalesRegisterWorkspace } from "@/live-api/types";
import { salesCopy, type SalesCopyKey } from "@/lib/sales-copy";
import { saleBusinessReference, saleNeedsAttention, saleStatusPresentation } from "@/lib/sales-presentation";
import styles from "@/components/sales/sales-home.module.css";

type StatusFilter = "ALL" | "POSTED" | "REVERSED" | "EXCEPTION";

function matchesStatus(sale: Sale, filter: StatusFilter): boolean {
  if (filter === "ALL") return true;
  if (filter === "POSTED") return sale.record_type === "SALE" && sale.status === "POSTED";
  if (filter === "REVERSED") return sale.record_type === "REVERSAL" || sale.status === "REVERSED";
  return saleNeedsAttention(sale);
}

export function LiveSalesList({ workspace, intent, initialStatus }: { workspace: SalesRegisterWorkspace; intent?: string; initialStatus?: string }) {
  const { locale } = useLanguage();
  const copy = (key: SalesCopyKey) => salesCopy(locale, key);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const validInitialStatus = ["POSTED", "REVERSED", "EXCEPTION"].includes(initialStatus ?? "") ? initialStatus as StatusFilter : "ALL";
  const [statusFilter, setStatusFilter] = useState<StatusFilter>(intent === "return" ? "POSTED" : validInitialStatus);
  const registerTitle = copy(intent === "return" ? "returnsRegister" : "transactionRegister");
  const registerIntro = copy(intent === "return" ? "returnsRegisterIntro" : "transactionRegisterIntro");
  const listTitle = copy(intent === "return" ? "eligibleSales" : "transactions");
  const customers = useMemo(() => new Map(workspace.customers.map((customer) => [customer.id, customer])), [workspace.customers]);

  const filteredSales = useMemo(() => {
    const normalized = deferredQuery.trim().toLocaleLowerCase(locale);
    return [...workspace.sales]
      .sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at) || right.receipt_reference.localeCompare(left.receipt_reference))
      .filter((sale) => {
        if (!matchesStatus(sale, statusFilter)) return false;
        if (!normalized) return true;
        const customer = customers.get(sale.customer_id);
        return [sale.receipt_reference, customer?.name, customer?.code, sale.kind, sale.status]
          .filter(Boolean)
          .join(" ")
          .toLocaleLowerCase(locale)
          .includes(normalized);
      });
  }, [customers, deferredQuery, locale, statusFilter, workspace.sales]);

  const nextHref = workspace.nextCursor
    ? `/sales/transactions?${new URLSearchParams({ cursor: workspace.nextCursor, ...(intent ? { intent } : {}), ...(statusFilter !== "ALL" ? { status: statusFilter } : {}) }).toString()}`
    : null;

  return (
    <div className={styles.page}>
      <SalesModuleNav permissions={workspace.context.permissions} />

      <header className={styles.registerHeader}>
        <div><p className={styles.scope}>{workspace.context.company_name} · {workspace.context.branch_name}</p><h1>{registerTitle}</h1><p>{registerIntro}</p></div>
        {workspace.context.permissions.includes("sales.complete") ? <Link className={styles.primaryAction} href="/sales/new">{copy("newSale")}</Link> : null}
      </header>

      {intent === "return" ? <aside className={styles.returnHint}><RotateCcw size={20} aria-hidden="true" /><p>{copy("returnHint")}</p></aside> : null}

      <section className={styles.transactionsCard} aria-labelledby="sales-register-heading">
        <div className={styles.registerTools}>
          <div><h2 id="sales-register-heading">{listTitle}</h2><p className={styles.resultsCount} role="status" aria-live="polite">{filteredSales.length} {copy("results")}</p></div>
          <div className={styles.filterGroup}>
            <label className={styles.searchField}>
              <span className="sr-only">{copy("searchTransactions")}</span>
              <Search size={18} aria-hidden="true" />
              <input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={copy("searchPlaceholder")} />
              {query ? <button type="button" onClick={() => setQuery("")} aria-label={copy("clearSearch")}><X size={16} aria-hidden="true" /></button> : null}
            </label>
            <label className={styles.filterSelect}>
              <span className="sr-only">{copy("status")}</span>
              <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}>
                <option value="ALL">{copy("allStatuses")}</option>
                <option value="POSTED">{copy("postedOnly")}</option>
                <option value="REVERSED">{copy("reversedOnly")}</option>
                <option value="EXCEPTION">{copy("exceptionsOnly")}</option>
              </select>
            </label>
          </div>
        </div>

        {filteredSales.length ? (
          <div className={styles.tableRegion} role="region" aria-labelledby="sales-register-heading" tabIndex={0}>
            <table className={styles.table}>
              <caption className="sr-only">{registerTitle}</caption>
              <thead><tr><th>{copy("type")}</th><th>{copy("number")}</th><th>{copy("customer")}</th><th className={styles.alignRight}>{copy("amount")}</th><th>{copy("status")}</th><th>{copy("date")}</th><th><span className="sr-only">{copy("openTransaction")}</span></th></tr></thead>
              <tbody>{filteredSales.map((sale) => {
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
              })}</tbody>
            </table>
          </div>
        ) : (
          <div className={styles.emptyState}><ReceiptText size={28} aria-hidden="true" /><p>{query || statusFilter !== "ALL" ? copy("noMatches") : copy("noTransactions")}</p></div>
        )}

        {nextHref ? <nav className={styles.pagination} aria-label={copy("transactions")}><Link href={nextHref}>{copy("nextPage")}<ArrowRight size={16} aria-hidden="true" /></Link></nav> : null}
      </section>
    </div>
  );
}
