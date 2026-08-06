"use client";

import Link from "next/link";
import {
  ArrowRight,
  BanknoteArrowUp,
  CircleDollarSign,
  FileCheck2,
  Landmark,
  PackageCheck,
  ReceiptText,
  Scale,
  ShieldCheck,
  ShoppingCart,
  TriangleAlert,
} from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import { formatTimestamp } from "@/live-api/format";
import type { DashboardWorkspace } from "@/live-api/types";

const metricCopy = {
  revenue: {
    label: text("Month-to-date revenue", "Mapato ya mwezi hadi sasa"),
    detail: text("Posted revenue accounts", "Akaunti za mapato zilizochapishwa"),
    icon: ReceiptText,
  },
  net_profit: {
    label: text("Month-to-date net profit", "Faida halisi ya mwezi hadi sasa"),
    detail: text("Posted revenue less expenses", "Mapato yaliyopostiwa ukiondoa gharama"),
    icon: Scale,
  },
  cash_position: {
    label: text("Cash position", "Hali ya fedha"),
    detail: text("Ledger balance in governed cash accounts", "Salio la leja katika akaunti za fedha zinazodhibitiwa"),
    icon: Landmark,
  },
  total_assets: {
    label: text("Total assets", "Jumla ya mali"),
    detail: text("Posted asset-account balance", "Salio la akaunti za mali lililopostiwa"),
    icon: CircleDollarSign,
  },
} as const;

const actions = [
  { permission: "sales.complete", href: "/sales/new", label: text("Record sale", "Rekodi mauzo"), detail: text("Post a controlled cash or credit sale", "Chapisha mauzo ya fedha au mkopo"), icon: ShoppingCart },
  { permission: "purchases.requests.manage", href: "/purchases", label: text("Start purchase", "Anza ununuzi"), detail: text("Create a governed purchase request", "Unda ombi la ununuzi linalodhibitiwa"), icon: PackageCheck },
  { permission: "finance.bank.import", href: "/finance", label: text("Import bank statement", "Ingiza taarifa ya benki"), detail: text("Begin a reconciled banking workflow", "Anza mchakato wa upatanisho wa benki"), icon: BanknoteArrowUp },
  { permission: "inventory.counts.manage", href: "/inventory", label: text("Count inventory", "Hesabu bidhaa"), detail: text("Open a controlled stock count", "Fungua hesabu ya bidhaa inayodhibitiwa"), icon: FileCheck2 },
] as const;

function formatDashboardMoney(amount: string, currency: string, locale: "en" | "sw"): string {
  const value = Number(amount);
  if (!Number.isFinite(value)) return `${currency} ${amount}`;
  return new Intl.NumberFormat(locale === "sw" ? "sw-TZ" : "en-TZ", {
    style: "currency",
    currency,
    currencyDisplay: "code",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

export function DashboardView({ workspace }: { workspace: DashboardWorkspace }) {
  const { l, locale } = useLanguage();
  const { context, dashboard } = workspace;
  const permissions = new Set(context.permissions);
  const visibleActions = actions.filter((action) => permissions.has(action.permission));

  return (
    <div className="page-stack">
      <section className="page-heading dashboard-heading">
        <div>
          <div className="heading-badges"><LiveBadge context={context} /></div>
          <h1>{l(text("Executive overview", "Muhtasari wa uongozi"))}</h1>
          <p>{context.company_name} · {context.branch_name} · {context.warehouse_name}</p>
        </div>
        <div className="heading-state">
          <span><span className="pulse-dot" />{l(text("Governed ledger snapshot", "Muhtasari wa leja unaodhibitiwa"))}</span>
          <small>{l(text("Generated", "Imetengenezwa"))} {formatTimestamp(dashboard.as_of, locale, context.timezone)}</small>
        </div>
      </section>

      {visibleActions.length > 0 ? (
        <section aria-labelledby="dashboard-actions-title">
          <div className="section-heading"><div><p className="eyebrow">{l(text("Authorized actions", "Hatua zilizoidhinishwa"))}</p><h2 id="dashboard-actions-title" className="sr-only">{l(text("Authorized actions", "Hatua zilizoidhinishwa"))}</h2></div></div>
          <div className="quick-actions-grid">
            {visibleActions.map((action) => {
              const Icon = action.icon;
              return <Link href={action.href} className="quick-action" key={action.href}><span><Icon size={19} /></span><div><strong>{l(action.label)}</strong><small>{l(action.detail)}</small></div><ArrowRight size={16} /></Link>;
            })}
          </div>
        </section>
      ) : null}

      <section className="metric-grid" aria-label={l(text("Governed financial metrics", "Vipimo vya fedha vinavyodhibitiwa"))}>
        {dashboard.metrics.map((metric) => {
          const copy = metricCopy[metric.key as keyof typeof metricCopy];
          const Icon = copy?.icon ?? CircleDollarSign;
          return (
            <article className="metric-card" key={metric.key}>
              <div className="metric-card-top"><span className="metric-icon"><Icon size={19} /></span><span className="reconciliation-status status-resolved">LIVE</span></div>
              <p>{copy ? l(copy.label) : metric.label}</p>
              <strong>{formatDashboardMoney(metric.value.amount, metric.value.currency, locale)}</strong>
              <small>{copy ? l(copy.detail) : l(text("Governed ERP metric", "Kipimo cha ERP kinachodhibitiwa"))}</small>
            </article>
          );
        })}
      </section>

      <div className="dashboard-secondary-grid">
        <section className="card" id="approvals">
          <div className="card-heading">
            <div><span className="card-kicker"><FileCheck2 size={15} /> {l(text("Needs a decision", "Inahitaji uamuzi"))}</span><h2>{l(text("Transactional approvals", "Idhini za miamala"))}</h2></div>
            <Link href="/purchases">{l(text("Open workflows", "Fungua michakato"))}<ArrowRight size={15} /></Link>
          </div>
          <div className="permission-empty">
            <strong>{dashboard.pending_approvals}</strong>
            <p>{l(text(
              "Submitted sales, purchase, inventory, and finance documents in this branch and warehouse scope.",
              "Nyaraka za mauzo, ununuzi, bidhaa na fedha zilizowasilishwa katika tawi na ghala hili.",
            ))}</p>
          </div>
        </section>

        <section className="card">
          <div className="card-heading"><div><span className="card-kicker"><TriangleAlert size={15} /> {l(text("Governed signals", "Ishara zinazodhibitiwa"))}</span><h2>{l(text("Operations watch", "Uangalizi wa shughuli"))}</h2></div></div>
          {dashboard.alerts.length > 0 ? (
            <div className="alert-list">
              {dashboard.alerts.map((alert) => <div className="alert-row" key={alert.id}><span className={`alert-icon tone-${alert.severity === "critical" ? "danger" : alert.severity}`}><TriangleAlert size={17} /></span><div><strong>{alert.title}</strong><span>{alert.source_type}</span><small>{alert.source_id}</small></div></div>)}
            </div>
          ) : (
            <div className="permission-empty"><ShieldCheck size={24} /><strong>{l(text("No governed alerts emitted", "Hakuna tahadhari zilizotolewa"))}</strong><p>{l(text("The API has not emitted an exception for this snapshot. This is not a simulated all-clear.", "API haijatoa hitilafu kwa muhtasari huu. Hii si hali ya majaribio."))}</p></div>
          )}
        </section>
      </div>

      <section className="card">
        <div className="card-heading"><div><span className="card-kicker"><ShieldCheck size={15} /> {l(text("Data boundary", "Mipaka ya data"))}</span><h2>{l(text("What this overview means", "Maana ya muhtasari huu"))}</h2></div><Link href="/reports">{l(text("Reconcile in reports", "Patanisha katika ripoti"))}<ArrowRight size={15} /></Link></div>
        <div className="permission-empty"><p>{l(text(
          "All monetary cards come from posted legal-company ledger entries. Trends, branch rankings, close-readiness scores, and activity feeds remain absent until governed projections are implemented.",
          "Kadi zote za fedha zinatoka kwenye rekodi za leja za kampuni zilizopostiwa. Mwenendo, viwango vya matawi, alama za utayari wa kufunga na taarifa za shughuli hazitaonyeshwa hadi makadirio yanayodhibitiwa yatakapotekelezwa.",
        ))}</p></div>
      </section>
    </div>
  );
}
