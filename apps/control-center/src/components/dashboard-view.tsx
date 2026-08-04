"use client";

import Link from "next/link";
import {
  ArrowRight, BanknoteArrowUp, Box, CircleDollarSign, CircleGauge, Clock3, FileText,
  Landmark, PackageSearch, ReceiptText, ShoppingBag, ShoppingCart, Sparkles, TrendingDown,
  TrendingUp, TriangleAlert, WalletCards,
} from "lucide-react";
import type { DashboardData, QuickAction } from "@/domain/erp";
import { useLanguage } from "@/components/language-provider";
import { StatusPill } from "@/components/status-pill";

const quickActionIcons = { sale: ShoppingCart, purchase: ShoppingBag, payment: WalletCards, stock: Box };
const metricIcons = [ReceiptText, Landmark, CircleDollarSign, PackageSearch];

function RevenueChart({ data }: { data: DashboardData["revenue"] }) {
  const { l, t } = useLanguage();
  const width = 640;
  const height = 210;
  const xStep = data.length > 1 ? 560 / (data.length - 1) : 0;
  const max = Math.max(1, ...data.map((point) => point.value)) * 1.12;
  const points = data.map((point, index) => ({ x: 48 + index * xStep, y: 178 - (point.value / max) * 145, ...point }));
  const path = points.map((point, index) => `${index === 0 ? "M" : "L"} ${point.x} ${point.y}`).join(" ");
  const area = `${path} L ${points.at(-1)?.x ?? 608} 178 L 48 178 Z`;

  return (
    <div className="chart-wrap">
      <svg className="revenue-chart" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={t("revenueChartLabel")}>
        <defs><linearGradient id="chartArea" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#2f7a61" stopOpacity=".25" /><stop offset="100%" stopColor="#2f7a61" stopOpacity="0" /></linearGradient></defs>
        {[42, 87, 132, 177].map((y) => <line key={y} x1="48" x2="608" y1={y} y2={y} className="chart-grid" />)}
        <path d={area} fill="url(#chartArea)" /><path d={path} className="chart-line" />
        {points.map((point) => <g key={point.month.en}><circle cx={point.x} cy={point.y} r="5" className="chart-dot" /><text x={point.x} y="202" textAnchor="middle" className="chart-label">{l(point.month)}</text><text x={point.x} y={point.y - 14} textAnchor="middle" className="chart-value">{point.label}</text></g>)}
      </svg>
    </div>
  );
}

function QuickActionCard({ action }: { action: QuickAction }) {
  const { l } = useLanguage();
  const Icon = quickActionIcons[action.icon];
  return <Link href={action.href} className="quick-action"><span><Icon size={19} /></span><div><strong>{l(action.label)}</strong><small>{l(action.description)}</small></div><ArrowRight size={16} /></Link>;
}

export function DashboardView({ data }: { data: DashboardData }) {
  const { t, l } = useLanguage();

  return (
    <div className="page-stack">
      <section className="page-heading dashboard-heading">
        <div><p className="eyebrow">{t("dashboardDate")}</p><h1>{t("greeting")}</h1><p>{t("dashboardIntro")}</p></div>
        <div className="heading-state"><span><span className="pulse-dot" />{t("liveControls")}</span><small>{t("lastSynced")}</small></div>
      </section>

      <section aria-labelledby="quick-actions-title">
        <div className="section-heading"><div><p className="eyebrow">{t("quickActions")}</p><h2 id="quick-actions-title" className="sr-only">{t("quickActions")}</h2></div></div>
        <div className="quick-actions-grid">{data.quickActions.map((action) => <QuickActionCard key={action.href} action={action} />)}</div>
      </section>

      <section className="metric-grid" aria-label={t("businessPerformanceMetrics")}>
        {data.metrics.map((metric, index) => {
          const Icon = metricIcons[index];
          const TrendIcon = metric.direction === "down" ? TrendingDown : TrendingUp;
          return <article className="metric-card" key={metric.label.en}><div className="metric-card-top"><span className="metric-icon"><Icon size={19} /></span>{metric.change ? <span className={`metric-change ${metric.direction}`}><TrendIcon size={14} />{metric.change}</span> : null}</div><p>{l(metric.label)}</p><strong>{metric.value}</strong><small>{l(metric.detail)}</small></article>;
        })}
      </section>

      <div className="dashboard-primary-grid">
        <section className="card revenue-card">
          <div className="card-heading"><div><span className="card-kicker"><CircleGauge size={15} /> {t("performance")}</span><h2>{t("revenue")}</h2><p>{t("revenueSubtitle")}</p></div><span className="period-chip">{t("periodMayAugust")}</span></div>
          <RevenueChart data={data.revenue} />
        </section>
        <section className="card sales-mix-card">
          <div className="card-heading"><div><span className="card-kicker"><Sparkles size={15} /> {t("today")}</span><h2>{t("salesMix")}</h2><p>{t("totalToday")}</p></div></div>
          <div className="donut-wrap"><div className="donut" aria-label={t("salesMixChartLabel")}><span><strong>246</strong><small>{t("salesCount")}</small></span></div></div>
          <div className="legend-list">{data.salesMix.map((item) => <div key={item.label.en}><span className="legend-color" style={{ background: item.color }} /><span><strong>{l(item.label)}</strong><small>{item.value}%</small></span><b>{item.amount}</b></div>)}</div>
        </section>
      </div>

      <div className="dashboard-secondary-grid">
        <section className="card" id="approvals">
          <div className="card-heading"><div><span className="card-kicker"><Clock3 size={15} /> {t("needsDecision")}</span><h2>{t("approvalInbox")}</h2></div><Link href="/purchases">{t("viewAll")}<ArrowRight size={15} /></Link></div>
          <div className="approval-list">{data.approvals.map((approval) => <Link href={approval.href} key={approval.id} className="approval-row"><span className={`approval-icon tone-${approval.tone}`}><FileText size={18} /></span><div className="approval-main"><span>{l(approval.type)}</span><strong>{approval.subject}</strong><small>{approval.id} · {approval.requester}</small></div><div className="approval-meta"><strong>{approval.amount}</strong><small>{l(approval.age)}</small></div><ArrowRight size={16} /></Link>)}</div>
        </section>
        <section className="card">
          <div className="card-heading"><div><span className="card-kicker"><TriangleAlert size={15} /> {t("attention")}</span><h2>{t("operationsWatch")}</h2></div></div>
          <div className="alert-list">{data.alerts.map((alert) => <Link href={alert.href} key={alert.title.en} className="alert-row"><span className={`alert-icon tone-${alert.tone}`}><TriangleAlert size={17} /></span><div><strong>{l(alert.title)}</strong><span>{l(alert.detail)}</span><small>{alert.meta}</small></div><ArrowRight size={16} /></Link>)}</div>
        </section>
      </div>

      <div className="dashboard-tertiary-grid">
        <section className="card">
          <div className="card-heading"><div><span className="card-kicker"><BanknoteArrowUp size={15} /> {t("salesLabel")}</span><h2>{t("branchPerformance")}</h2></div><Link href="/reports/RPT-SALES-01">{t("viewAll")}<ArrowRight size={15} /></Link></div>
          <div className="branch-list">{data.branches.map((branch) => <div key={branch.branch}><div><strong>{branch.branch}</strong><span><b>{branch.amount}</b><em>{branch.change}</em></span></div><div className="progress-track"><span style={{ width: `${branch.percent}%` }} /></div></div>)}</div>
        </section>
        <section className="card">
          <div className="card-heading"><div><span className="card-kicker"><Clock3 size={15} /> {t("audit")}</span><h2>{t("recentActivity")}</h2></div></div>
          <div className="activity-list">{data.activity.map((activity) => <div key={`${activity.subject}-${activity.actor}`}><span className={`activity-dot tone-${activity.tone}`} /><div><p><strong>{activity.actor}</strong> {l(activity.action)}</p><span>{activity.subject} · {l(activity.time)}</span></div></div>)}</div>
        </section>
        <section className="card readiness-card">
          <div className="card-heading"><div><span className="card-kicker"><CircleGauge size={15} /> {t("financeLabel")}</span><h2>{t("closeReadiness")}</h2></div><strong className="readiness-score">76%</strong></div>
          <div className="readiness-list">{data.closeReadiness.map((item) => <div key={item.label.en}><div><strong>{l(item.label)}</strong><StatusPill status={item.status} compact /></div><div className="progress-track"><span style={{ width: `${item.value}%` }} /></div></div>)}</div>
        </section>
      </div>
    </div>
  );
}
