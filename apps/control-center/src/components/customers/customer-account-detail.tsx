"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, CalendarClock, CircleAlert, FileText, LockKeyhole, ShieldCheck, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import { clearPendingCommandAfterSuccess, customerCreditPolicyPendingScope, markPendingCommandRejected, PendingCommandConflictError, reservePendingCommand } from "@/live-api/pending-command";
import type { CustomerAccountWorkspace, PublicProblem, ScheduleCreditPolicyCommand } from "@/live-api/types";
import { text } from "@/lib/i18n";

function isProblem(value: unknown): value is PublicProblem { return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string"); }

export function CustomerAccountDetail({ workspace }: { workspace: CustomerAccountWorkspace }) {
  const { locale, l } = useLanguage();
  const router = useRouter();
  const { account, context } = workspace;
  const canManage = context.permissions.includes("customers.credit.manage") && !account.customer.is_general_customer;
  const [limitTzs, setLimitTzs] = useState(String(Math.trunc(account.active_policy.credit_limit_minor / 100)));
  const [terms, setTerms] = useState(String(account.active_policy.payment_terms_days));
  const [overdue, setOverdue] = useState(String(account.active_policy.max_overdue_days));
  const [enabled, setEnabled] = useState(account.active_policy.credit_enabled);
  const [risk, setRisk] = useState<ScheduleCreditPolicyCommand["risk_status"]>(account.active_policy.risk_status);
  const [reason, setReason] = useState("");
  const [effectiveFrom, setEffectiveFrom] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const limit = Number(limitTzs) * 100, termDays = Number(terms), overdueDays = Number(overdue);
  const valid = Number.isSafeInteger(limit) && limit >= 0 && Number.isInteger(termDays) && termDays >= 0 && termDays <= 365 && Number.isInteger(overdueDays) && overdueDays >= 0 && overdueDays <= 3650 && reason.trim().length >= 8 && Boolean(effectiveFrom) && (enabled || limit === 0);

  async function schedulePolicy() {
    if (!valid) return;
    const command: ScheduleCreditPolicyCommand = { credit_enabled: enabled, credit_limit_minor: limit, payment_terms_days: termDays, max_overdue_days: overdueDays, risk_status: risk, reason: reason.trim(), effective_from: new Date(effectiveFrom).toISOString() };
    const fingerprint = JSON.stringify(command);
    const scope = customerCreditPolicyPendingScope(context, account.customer.id);
    let pending;
    try { pending = reservePendingCommand({ storage: window.sessionStorage, scope, fingerprint, payload: command }); }
    catch (error) { setProblem({ type: "about:blank", title: "Command not sent", status: 409, code: "pending_credit_policy", detail: error instanceof PendingCommandConflictError ? l(text("A different policy still awaits an authoritative result.", "Sera tofauti bado inasubiri matokeo rasmi.")) : l(text("The command identity could not be preserved.", "Utambulisho wa amri haukuweza kuhifadhiwa.")) }); return; }
    setSubmitting(true); setProblem(null);
    try {
      const response = await fetch(`/api/live/customers/${encodeURIComponent(account.customer.id)}/credit-policies`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key, Accept: "application/json" }, body: fingerprint });
      const payload: unknown = await response.json();
      if (!response.ok) { if (response.status >= 400 && response.status < 500 && response.status !== 409) { try { markPendingCommandRejected(window.sessionStorage, scope, pending.record.key, response.status); } catch { /* preserve safe replay */ } } setProblem(isProblem(payload) ? payload : { type: "about:blank", title: "Policy rejected", status: response.status, code: "policy_rejected", detail: "The ERP rejected this policy." }); return; }
      try { clearPendingCommandAfterSuccess(window.sessionStorage, scope, pending.record.key); } catch { /* backend replay remains safe */ }
      setReason(""); setEffectiveFrom(""); router.refresh();
    } catch { setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "bff_unavailable", detail: l(text("The policy was not confirmed. Check the timeline before retrying.", "Sera haijathibitishwa. Angalia ratiba kabla ya kujaribu tena.")) }); }
    finally { setSubmitting(false); }
  }

  const buckets = account.aging.buckets;
  return <div className="page-stack customer-account-page">
    <Link className="back-link" href="/customers"><ArrowLeft size={15} />{l(text("Customer accounts", "Akaunti za wateja"))}</Link>
    <section className="detail-hero"><div><div className="heading-badges"><LiveBadge context={context} /><span className={`reconciliation-status status-${account.active_policy.risk_status.toLowerCase()}`}>{account.active_policy.risk_status}</span></div><h1>{account.customer.name}</h1><p>{account.customer.code} · {account.customer.active ? l(text("Active", "Hai")) : l(text("Inactive", "Haifanyi kazi"))}</p></div><div className="detail-actions"><strong>{formatMinorUnits(account.aging.calculated_exposure_minor, context.currency, locale)}</strong><small>{l(text("authoritative exposure", "mfiduo rasmi"))}</small></div></section>
    {!account.aging.reconciled ? <aside className="sale-problem"><TriangleAlert size={18} /><span><strong>{l(text("Credit is fail-closed", "Mkopo umefungwa kwa usalama"))}</strong>{l(text("Open items do not reconcile to the customer ledger.", "Madeni hayalingani na leja ya mteja."))}</span></aside> : <aside className="live-control-note"><ShieldCheck size={18} /><div><strong>{l(text("Receivables reconcile", "Madeni yanalingana"))}</strong><p>{l(text("Invoice-level exposure agrees with the append-only customer ledger.", "Mfiduo wa ankara unalingana na leja ya mteja isiyobadilika."))}</p></div></aside>}
    <div className="metric-grid compact-metrics">{[["Current", "Ya sasa", buckets.current_minor], ["1–30 days", "Siku 1–30", buckets.days_1_30_minor], ["31–60 days", "Siku 31–60", buckets.days_31_60_minor], ["61–90 days", "Siku 61–90", buckets.days_61_90_minor], ["90+ days", "Zaidi ya siku 90", buckets.days_over_90_minor]].map(([en, sw, value]) => <article className="metric-card" key={String(en)}><p>{l(text(String(en), String(sw)))}</p><strong>{formatMinorUnits(Number(value), context.currency, locale)}</strong></article>)}</div>
    <div className="detail-layout"><div className="detail-main"><section className="card"><div className="card-heading"><div><p className="eyebrow">{l(text("Open receivables", "Madeni wazi"))}</p><h2>{l(text("Invoice-level evidence", "Ushahidi wa ankara"))}</h2></div><FileText size={19} /></div><div className="table-scroll"><table className="ledger-table"><thead><tr><th>{l(text("Document", "Hati"))}</th><th>{l(text("Due", "Tarehe ya malipo"))}</th><th>{l(text("Outstanding", "Salio"))}</th></tr></thead><tbody>{account.open_items.map((item) => <tr key={item.id}><td>{item.kind}<small className="table-subline">{item.source_type}</small></td><td>{item.due_at ? formatTimestamp(item.due_at, locale, context.timezone) : "—"}</td><td>{formatMinorUnits(item.outstanding_minor, item.currency, locale)}</td></tr>)}</tbody></table>{account.open_items.length === 0 ? <div className="empty-table"><ShieldCheck size={24} /><strong>{l(text("No open receivables", "Hakuna madeni wazi"))}</strong></div> : null}</div></section></div>
      <aside className="detail-side"><section className="card"><div className="card-heading"><div><p className="eyebrow">{l(text("Effective policy", "Sera inayotumika"))}</p><h2>{account.active_policy.credit_enabled ? l(text("Credit enabled", "Mkopo umeruhusiwa")) : l(text("Cash only", "Taslimu tu"))}</h2></div><CalendarClock size={19} /></div><dl className="live-context-details"><div><dt>{l(text("Limit", "Kikomo"))}</dt><dd>{formatMinorUnits(account.active_policy.credit_limit_minor, context.currency, locale)}</dd></div><div><dt>{l(text("Terms", "Masharti"))}</dt><dd>{account.active_policy.payment_terms_days} {l(text("days", "siku"))}</dd></div><div><dt>{l(text("Overdue tolerance", "Uvumilivu wa kuchelewa"))}</dt><dd>{account.active_policy.max_overdue_days} {l(text("days", "siku"))}</dd></div><div><dt>{l(text("Effective", "Inaanza"))}</dt><dd>{formatTimestamp(account.active_policy.effective_from, locale, context.timezone)}</dd></div></dl></section>
      {canManage ? <section className="card policy-form"><div className="card-heading"><div><p className="eyebrow">{l(text("Controlled change", "Badiliko linalodhibitiwa"))}</p><h2>{l(text("Schedule policy", "Panga sera"))}</h2></div><CircleAlert size={19} /></div><div className="form-grid"><label><span>{l(text("Credit enabled", "Mkopo umeruhusiwa"))}</span><select value={enabled ? "yes" : "no"} onChange={(event) => { const next = event.target.value === "yes"; setEnabled(next); if (!next) setLimitTzs("0"); }}><option value="yes">{l(text("Yes", "Ndiyo"))}</option><option value="no">{l(text("No", "Hapana"))}</option></select></label><label><span>{l(text("Limit (TZS)", "Kikomo (TZS)"))}</span><input type="number" min="0" step="1" value={limitTzs} onChange={(event) => setLimitTzs(event.target.value)} /></label><label><span>{l(text("Payment terms", "Muda wa malipo"))}</span><input type="number" min="0" max="365" value={terms} onChange={(event) => setTerms(event.target.value)} /></label><label><span>{l(text("Overdue tolerance", "Uvumilivu wa kuchelewa"))}</span><input type="number" min="0" max="3650" value={overdue} onChange={(event) => setOverdue(event.target.value)} /></label><label><span>{l(text("Risk", "Hatari"))}</span><select value={risk} onChange={(event) => setRisk(event.target.value as typeof risk)}><option>STANDARD</option><option>WATCH</option><option>HOLD</option></select></label><label><span>{l(text("Effective from", "Inaanza"))}</span><input type="datetime-local" value={effectiveFrom} onChange={(event) => setEffectiveFrom(event.target.value)} /></label><label className="field-wide"><span>{l(text("Approval reason", "Sababu ya idhini"))}</span><textarea rows={3} maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} /></label></div><button className="primary-button" disabled={!valid || submitting} onClick={schedulePolicy}>{submitting ? l(text("Recording…", "Inarekodi…")) : l(text("Schedule immutable policy", "Panga sera isiyobadilika"))}</button>{problem ? <div className="sale-problem"><TriangleAlert size={16} /><span><strong>{problem.code}</strong>{problem.detail}</span></div> : null}</section> : <section className="permission-empty"><LockKeyhole size={22} /><strong>{l(text("Credit management permission required", "Ruhusa ya kusimamia mikopo inahitajika"))}</strong></section>}</aside></div>
  </div>;
}
