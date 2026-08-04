"use client";

import Link from "next/link";
import { ArrowLeft, Check, ChevronDown, Copy, Ellipsis, FileText, History, Printer, RotateCcw, ShieldCheck } from "lucide-react";
import type { ModuleData, ModuleRecord } from "@/domain/erp";
import { useLanguage } from "@/components/language-provider";
import { StatusPill } from "@/components/status-pill";

export function RecordDetailView({ module, record }: { module: ModuleData; record: ModuleRecord }) {
  const { t, l } = useLanguage();

  async function copyLink() {
    await navigator.clipboard.writeText(window.location.href);
  }

  return (
    <div className="page-stack detail-page">
      <Link className="back-link" href={`/${module.key}`}><ArrowLeft size={16} />{t("back")} · {l(module.title)}</Link>
      <section className="detail-hero">
        <div><p className="eyebrow">{record.id}</p><h1>{l(record.primary)}</h1><p>{record.secondary} · {t("updated")} {l(record.updated)}</p></div>
        <div className="detail-actions"><StatusPill status={record.status} /><button className="secondary-button" type="button" onClick={() => window.print()}><Printer size={17} />{t("print")}</button><details className="more-menu"><summary className="secondary-button"><Ellipsis size={18} />{t("more")}<ChevronDown size={14} /></summary><div><button type="button" onClick={copyLink}><Copy size={16} />{t("copyLink")}</button><button type="button"><RotateCcw size={16} />{t("reverse")}</button></div></details></div>
      </section>

      <div className="detail-layout">
        <div className="detail-main">
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><FileText size={15} /> {t("record")}</span><h2>{t("recordOverview")}</h2></div></div><dl className="detail-grid">{record.details.map((field) => <div key={field.label.en}><dt>{l(field.label)}</dt><dd>{field.value}</dd></div>)}</dl></section>
          {record.ledgerImpact?.length ? <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><ShieldCheck size={15} /> {t("balanced")}</span><h2>{t("accountingImpact")}</h2></div></div><div className="table-scroll"><table className="ledger-table"><thead><tr><th>{t("account")}</th><th>{t("debit")}</th><th>{t("credit")}</th></tr></thead><tbody>{record.ledgerImpact.map((line) => <tr key={line.account}><td>{line.account}</td><td>{line.debit}</td><td>{line.credit}</td></tr>)}</tbody></table></div><div className="balanced-banner"><Check size={16} />{t("balancedNotice")}</div></section> : null}
        </div>
        <aside className="detail-side">
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><ShieldCheck size={15} /> {t("policy")}</span><h2>{t("controlChecks")}</h2></div></div><div className="control-list">{record.controls.map((control) => <div key={control.label.en}><span className={`control-icon status-${control.status.tone}`}><Check size={15} /></span><div><strong>{l(control.label)}</strong><p>{l(control.detail)}</p></div><StatusPill status={control.status} compact /></div>)}</div></section>
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><History size={15} /> {t("immutable")}</span><h2>{t("auditTrail")}</h2></div></div><div className="timeline">{record.timeline.map((event, index) => <div key={`${event.title.en}-${index}`}><span className={`timeline-dot tone-${event.tone}`} /><div><strong>{l(event.title)}</strong><p>{event.detail}</p><small>{l(event.time)}</small></div></div>)}</div></section>
        </aside>
      </div>
    </div>
  );
}
