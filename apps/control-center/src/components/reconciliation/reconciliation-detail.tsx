"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, CheckCircle2, ClipboardCheck, FileJson2, FileLock2, ShieldCheck, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { LiveBadge } from "@/components/live-sales/live-state";
import { useLanguage } from "@/components/language-provider";
import { compactId, formatTimestamp } from "@/live-api/format";
import {
  clearPendingCommandAfterSuccess, markPendingCommandRejected, PendingCommandConflictError,
  PendingCommandStorageError, reconciliationResolutionPendingScope, reservePendingCommand,
} from "@/live-api/pending-command";
import type { MobileReconciliationCase, PublicProblem, ReconciliationDetailWorkspace, ResolveMobileReconciliationCommand } from "@/live-api/types";
import { text } from "@/lib/i18n";

function isProblem(value: unknown): value is PublicProblem {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string");
}

function isCase(value: unknown): value is MobileReconciliationCase {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).id === "string");
}

export function ReconciliationDetail({ workspace }: { workspace: ReconciliationDetailWorkspace }) {
  const { locale, l } = useLanguage();
  const router = useRouter();
  const item = workspace.reconciliationCase;
  const canResolve = item.status === "OPEN" && workspace.context.permissions.includes("mobile.reconciliation.resolve");
  const [action, setAction] = useState<ResolveMobileReconciliationCommand["action"]>("CASH_REFUNDED");
  const [reason, setReason] = useState("");
  const [externalReference, setExternalReference] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const pendingScope = reconciliationResolutionPendingScope(workspace.context, item.id);
  const valid = reason.trim().length >= 8 && confirmed && (action !== "POSTED_EXTERNALLY" || externalReference.trim().length > 0);

  async function resolveCase() {
    if (!canResolve || !valid || submitting) return;
    const command: ResolveMobileReconciliationCommand = {
      action,
      reason: reason.trim(),
      ...(action === "POSTED_EXTERNALLY" ? { external_reference: externalReference.trim() } : {}),
    };
    const fingerprint = JSON.stringify(command);
    let pending;
    try {
      pending = reservePendingCommand({ storage: window.sessionStorage, scope: pendingScope, fingerprint, payload: command });
    } catch (error) {
      const detail = error instanceof PendingCommandConflictError
        ? l(text("A different resolution is awaiting an authoritative result. Restore that exact command before changing it.", "Uamuzi tofauti unasubiri matokeo rasmi. Rejesha amri hiyo kamili kabla ya kuibadilisha."))
        : error instanceof PendingCommandStorageError ? error.message : l(text("The command identity could not be preserved.", "Utambulisho wa amri haukuweza kuhifadhiwa."));
      setProblem({ type: "about:blank", title: "Resolution not sent", status: 409, code: "pending_resolution_conflict", detail });
      return;
    }
    setSubmitting(true);
    setProblem(null);
    try {
      const response = await fetch(`/api/live/mobile/reconciliation-cases/${encodeURIComponent(item.id)}/resolutions`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key, Accept: "application/json" },
        body: fingerprint,
      });
      const payload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) {
          try { markPendingCommandRejected(window.sessionStorage, pendingScope, pending.record.key, response.status); } catch { /* retry retains the original key */ }
        }
        setProblem(isProblem(payload) ? payload : { type: "about:blank", title: "Resolution rejected", status: response.status, code: "resolution_rejected", detail: "The live ERP rejected the resolution." });
        return;
      }
      if (!isCase(payload)) throw new Error("Resolution response did not contain a case ID.");
      try { clearPendingCommandAfterSuccess(window.sessionStorage, pendingScope, pending.record.key); } catch { /* backend replay remains safe */ }
      router.refresh();
    } catch {
      setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "bff_unavailable", detail: l(text("Retry is safe: the exact disposition retains its idempotency key.", "Kujaribu tena ni salama: uamuzi kamili unahifadhi ufunguo wake wa kutorudia.")) });
    } finally {
      setSubmitting(false);
    }
  }

  return <div className="page-stack reconciliation-detail-page">
    <Link className="back-link" href="/reconciliation"><ArrowLeft size={16} />{l(text("Back to reconciliation", "Rudi kwenye upatanisho"))}</Link>
    <section className="detail-hero live-detail-hero"><div><div className="heading-badges"><LiveBadge context={workspace.context} /><span className={`reconciliation-status status-${item.status.toLowerCase()}`}>{item.status}</span><span className="scope-chip">{compactId(item.device_id)}</span></div><p className="eyebrow">{compactId(item.id)}</p><h1>{l(text("Offline command evidence", "Ushahidi wa amri nje ya mtandao"))}</h1><p>{l(text("Retained without business posting effects", "Imehifadhiwa bila athari za uchapishaji wa biashara"))} · {formatTimestamp(item.created_at, locale, workspace.context.timezone)}</p></div></section>
    <aside className="immutable-banner"><FileLock2 size={19} /><div><strong>{l(text("Append-only evidence", "Ushahidi usiobadilika"))}</strong><p>{l(text("The original command and any resolution remain immutable. This workflow records a disposition; it does not create or alter a sale.", "Amri asili na uamuzi wowote hubaki bila kubadilishwa. Mchakato huu unarekodi uamuzi; hautengenezi wala kubadilisha mauzo."))}</p></div></aside>

    <div className="reconciliation-detail-grid">
      <section className="card evidence-card"><div className="card-heading"><div><p className="eyebrow">{l(text("Provenance", "Chanzo"))}</p><h2>{l(text("Captured command", "Amri iliyohifadhiwa"))}</h2></div><FileJson2 size={20} /></div><dl className="reference-list"><div><dt>{l(text("Device", "Kifaa"))}</dt><dd>{item.device_id}</dd></div><div><dt>{l(text("Client transaction", "Muamala wa mteja"))}</dt><dd>{item.client_transaction_id}</dd></div><div><dt>{l(text("Client timestamp", "Muda wa mteja"))}</dt><dd>{formatTimestamp(item.client_timestamp, locale, workspace.context.timezone)}</dd></div><div><dt>{l(text("Application / publication", "Programu / chapisho"))}</dt><dd>{item.app_version} · MD {item.master_data_version} · PRICE {item.price_version}<br />{item.catalog_snapshot_token}</dd></div><div><dt>{l(text("Failure", "Hitilafu"))}</dt><dd>{item.failure_code}</dd></div><div><dt>{l(text("Correlation", "Uhusiano"))}</dt><dd>{item.correlation_id}</dd></div></dl><details className="command-evidence"><summary>{l(text("View exact command JSON", "Tazama JSON kamili ya amri"))}</summary><pre>{JSON.stringify(item.command, null, 2)}</pre></details></section>

      <section className="card disposition-card"><div className="card-heading"><div><p className="eyebrow">{l(text("Controlled disposition", "Uamuzi unaodhibitiwa"))}</p><h2>{item.resolution ? l(text("Resolution recorded", "Uamuzi umerekodiwa")) : l(text("Record resolution", "Rekodi uamuzi"))}</h2></div><ClipboardCheck size={20} /></div>
        {item.resolution ? <div className="resolution-summary"><CheckCircle2 size={28} /><strong>{item.resolution.action}</strong><p>{item.resolution.reason}</p>{item.resolution.external_reference ? <code>{item.resolution.external_reference}</code> : null}<small>{formatTimestamp(item.resolution.resolved_at, locale, workspace.context.timezone)} · {compactId(item.resolution.resolved_by)}</small></div> : canResolve ? <div className="resolution-form"><fieldset><legend>{l(text("Verified outcome", "Matokeo yaliyothibitishwa"))}</legend>{(["CASH_REFUNDED", "POSTED_EXTERNALLY", "DUPLICATE_CONFIRMED"] as const).map((value) => <label key={value} className={action === value ? "active" : ""}><input type="radio" name="action" value={value} checked={action === value} onChange={() => { setAction(value); setConfirmed(false); setProblem(null); }} /><span><strong>{value === "CASH_REFUNDED" ? l(text("Cash refunded", "Fedha zimerejeshwa")) : value === "POSTED_EXTERNALLY" ? l(text("Posted externally", "Imechapishwa nje")) : l(text("Duplicate confirmed", "Nakala imethibitishwa"))}</strong><small>{value === "CASH_REFUNDED" ? l(text("Customer was made whole without ERP posting.", "Mteja amerudishiwa bila kuchapisha ERP.")) : value === "POSTED_EXTERNALLY" ? l(text("A controlled external record already owns the effects.", "Rekodi ya nje inayodhibitiwa tayari ina athari.")) : l(text("An existing ERP transaction already represents it.", "Muamala uliopo wa ERP tayari unaiwakilisha."))}</small></span></label>)}</fieldset>{action === "POSTED_EXTERNALLY" ? <label><span>{l(text("External reference", "Rejea ya nje"))}</span><input value={externalReference} onChange={(event) => { setExternalReference(event.target.value); setConfirmed(false); }} maxLength={200} /></label> : null}<label><span>{l(text("Verified reason", "Sababu iliyothibitishwa"))}</span><textarea value={reason} onChange={(event) => { setReason(event.target.value); setConfirmed(false); }} minLength={8} maxLength={500} rows={4} /></label><label className="confirmation-check"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} /><span>{l(text("I confirm the evidence and understand this action does not post stock or accounting.", "Ninathibitisha ushahidi na ninaelewa hatua hii haichapishi bidhaa wala uhasibu."))}</span></label>{problem ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>{problem.code}</strong>{problem.detail}</span></div> : null}<button type="button" className="primary-button resolution-submit" disabled={!valid || submitting} onClick={resolveCase}><ShieldCheck size={17} />{submitting ? l(text("Recording…", "Inarekodi…")) : l(text("Record immutable resolution", "Rekodi uamuzi usiobadilika"))}</button></div> : <div className="permission-empty"><ShieldCheck size={25} /><strong>{l(text("Resolution permission required", "Ruhusa ya uamuzi inahitajika"))}</strong><p>{l(text("You may inspect this case, but the authenticated role cannot record its disposition.", "Unaweza kukagua kesi hii, lakini jukumu lililothibitishwa haliwezi kurekodi uamuzi wake."))}</p></div>}
      </section>
    </div>
  </div>;
}
