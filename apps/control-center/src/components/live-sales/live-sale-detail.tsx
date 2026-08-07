"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Check, Copy, FileLock2, History, Printer, ReceiptText, RotateCcw, ShieldCheck, TriangleAlert } from "lucide-react";
import { useMemo, useState } from "react";
import type { PublicProblem, ReverseSaleCommand, Sale, SaleDetailWorkspace } from "@/live-api/types";
import { compactId, formatMinorUnits, formatTimestamp } from "@/live-api/format";
import {
  clearPendingCommandAfterSuccess,
  markPendingCommandRejected,
  PendingCommandConflictError,
  PendingCommandStorageError,
  reservePendingCommand,
  saleReversalPendingScope,
  type PendingCommandSnapshot,
} from "@/live-api/pending-command";
import { usePendingCommand } from "@/live-api/use-pending-command";
import { LiveBadge } from "@/components/live-sales/live-state";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";
import { AuditDrawer } from "@/components/audit-drawer";

function isProblem(value: unknown): value is PublicProblem {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string");
}

function hasSaleId(value: unknown): value is Pick<Sale, "id"> {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).id === "string" && String((value as Record<string, unknown>).id).length > 0);
}

function isReverseSaleCommand(value: unknown): value is ReverseSaleCommand {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).reason === "string" && String((value as Record<string, unknown>).reason).trim().length >= 5);
}

export function LiveSaleDetail({ workspace }: { workspace: SaleDetailWorkspace }) {
  return <ScopedLiveSaleDetail key={saleReversalPendingScope(workspace.context, workspace.sale.id)} workspace={workspace} />;
}

function ScopedLiveSaleDetail({ workspace }: { workspace: SaleDetailWorkspace }) {
  const { locale, l } = useLanguage();
  const router = useRouter();
  const { sale } = workspace;
  const [reversalOpen, setReversalOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const products = useMemo(() => new Map(workspace.products.map((product) => [product.id, product])), [workspace.products]);
  const customer = workspace.customers.find((item) => item.id === sale.customer_id);
  const reversible = sale.record_type === "SALE" && sale.status === "POSTED";
  const pendingScope = saleReversalPendingScope(workspace.context, sale.id);
  const pendingStore = usePendingCommand<unknown>(pendingScope);
  const pendingRecovery = pendingStore.snapshot && isReverseSaleCommand(pendingStore.snapshot.record.payload)
    ? pendingStore.snapshot as PendingCommandSnapshot<ReverseSaleCommand>
    : null;
  const recoveryProblem: PublicProblem | null = pendingStore.error
    ? { type: "about:blank", title: "Recovery unavailable", status: 503, code: "pending_reversal_storage_unavailable", detail: pendingStore.error.message }
    : pendingStore.snapshot && !pendingRecovery
      ? { type: "about:blank", title: "Recovery blocked", status: 409, code: "pending_reversal_invalid", detail: "A preserved reversal marker cannot be reconstructed safely. Reconcile the live sale before taking another action." }
      : null;
  const recoveryBlocked = Boolean(recoveryProblem);
  const displayedProblem = problem ?? recoveryProblem;
  const fiscalCopy = sale.fiscal_status === "FISCALIZED"
    ? text("Fiscal workflow marked fiscalized", "Mchakato wa fiskali umewekwa kuwa umekamilika")
    : sale.fiscal_status === "PENDING"
      ? text("Fiscalization pending", "Ufiskalishaji unasubiri")
      : sale.fiscal_status === "FAILED"
        ? text("Fiscalization failed", "Ufiskalishaji umeshindwa")
        : text("Fiscal integration not configured", "Muunganisho wa fiskali haujawekwa");

  async function copyLink() {
    await navigator.clipboard.writeText(window.location.href);
  }

  async function submitReversal() {
    const trimmedReason = reason.trim();
    if (!reversible || trimmedReason.length < 5 || !confirmed || recoveryBlocked || completed) return;
    const command: ReverseSaleCommand = { reason: trimmedReason };
    const fingerprint = JSON.stringify(command);
    let pending: PendingCommandSnapshot<ReverseSaleCommand>;
    try {
      pending = reservePendingCommand({ storage: window.sessionStorage, scope: pendingScope, fingerprint, payload: command });
    } catch (error) {
      const detail = error instanceof PendingCommandConflictError
        ? "A different reversal is still awaiting an authoritative result. Restore its exact reason before retrying."
        : error instanceof PendingCommandStorageError
          ? error.message
          : "Safe retry information could not be preserved, so the reversal was not sent. Start the correction again.";
      setProblem({ type: "about:blank", title: "Reversal not sent", status: 409, code: "pending_reversal_conflict", detail });
      return;
    }
    setSubmitting(true);
    setProblem(null);
    try {
      const response = await fetch(`/api/live/sales/${encodeURIComponent(sale.id)}/reversals`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key, Accept: "application/json" },
        body: fingerprint,
      });
      const payload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) {
          try {
            markPendingCommandRejected<ReverseSaleCommand>(window.sessionStorage, pendingScope, pending.record.key, response.status);
          } catch {
            // The preserved original key remains the safe retry identity even if metadata cannot be updated.
          }
        }
        setProblem(isProblem(payload) ? payload : { type: "about:blank", title: "Reversal rejected", status: response.status, code: "reversal_rejected", detail: "The live ERP rejected the reversal command." });
        return;
      }
      if (!hasSaleId(payload)) throw new Error("The successful reversal response did not contain an ID.");
      const reversal = payload;
      try {
        clearPendingCommandAfterSuccess(window.sessionStorage, pendingScope, pending.record.key);
      } catch {
        // A replay remains safe because the backend retains committed idempotency records.
      }
      setCompleted(true);
      router.push(`/sales/${reversal.id}`);
      router.refresh();
    } catch {
      setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "bff_unavailable", detail: "We safely preserved this correction attempt. Retry to confirm or complete the same correction without posting it twice." });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page-stack live-sale-detail-page">
      <Link className="back-link" href="/sales"><ArrowLeft size={16} />{l(text("Back to live sales", "Rudi kwenye mauzo hai"))}</Link>
      <section className="detail-hero live-detail-hero">
        <div><div className="heading-badges"><LiveBadge context={workspace.context} /><span className={`live-sale-status status-${sale.status.toLowerCase()}`}>{sale.status}</span><span className="scope-chip">{sale.record_type}</span></div><p className="eyebrow" title={sale.id}>{compactId(sale.id)}</p><h1>{sale.record_type === "REVERSAL" ? l(text("Linked sale reversal", "Ubatilisho wa mauzo uliounganishwa")) : l(text("Immutable posted sale", "Mauzo yaliyochapishwa yasiyobadilika"))}</h1><p>{customer?.name ?? compactId(sale.customer_id)} · {formatTimestamp(sale.created_at, locale, workspace.context.timezone)}</p></div>
        <div className="detail-actions"><AuditDrawer entityType="sale" entityId={sale.id} context={workspace.context} /><button type="button" className="secondary-button" onClick={() => window.print()}><Printer size={17} />{l(text("Print", "Chapisha"))}</button><button type="button" className="secondary-button" onClick={copyLink}><Copy size={17} />{l(text("Copy link", "Nakili kiungo"))}</button>{reversible ? <button type="button" className="danger-button" onClick={() => setReversalOpen(true)}><RotateCcw size={17} />{l(text("Reverse sale", "Batilisha mauzo"))}</button> : null}</div>
      </section>

      <aside className="immutable-banner"><FileLock2 size={19} /><div><strong>{l(text("Posted record — editing is disabled", "Rekodi imechapishwa — kuhariri kumezuiwa"))}</strong><p>{l(text("Corrections use a reasoned, linked reversal. The original sale, correlation and posting values remain intact.", "Marekebisho hutumia ubatilisho wenye sababu na kiungo. Mauzo asili, uhusiano na thamani zake hubaki salama."))}</p></div></aside>

      {pendingRecovery ? <aside className={`pending-command-banner ${pendingRecovery.expired ? "pending-command-stale" : ""}`} role="status"><History size={19} /><div><strong>{l(text("Unconfirmed reversal recovered", "Ubatilisho usiothibitishwa umerejeshwa"))}</strong><p>{pendingRecovery.record.outcome === "rejected" ? l(text("The ERP rejected the previous attempt without posting. Restore it before correcting the reason; its safe retry details will be retained.", "ERP ilikataa jaribio la awali bila kuchapisha. Irejeshe kabla ya kurekebisha sababu; maelezo salama ya kujaribu tena yatahifadhiwa.")) : l(text("The reason from this correction attempt is safely preserved. Restore, confirm and retry to complete the same linked correction without posting it twice.", "Sababu ya jaribio hili la marekebisho imehifadhiwa salama. Rejesha, thibitisha na ujaribu tena kukamilisha marekebisho yale yale bila kuyachapisha mara mbili."))}</p><small>{l(text("Preserved", "Imehifadhiwa"))} · {formatTimestamp(new Date(pendingRecovery.record.createdAt).toISOString(), locale, workspace.context.timezone)}{pendingRecovery.expired ? ` · ${l(text("reconciliation recommended", "upatanisho unapendekezwa"))}` : ""}</small></div><div className="pending-command-actions"><button type="button" className="secondary-button" onClick={() => { setReason(pendingRecovery.record.payload.reason); setConfirmed(false); setReversalOpen(true); setProblem(null); }}>{l(text("Restore exact reason", "Rejesha sababu halisi"))}</button><Link className="secondary-button" href="/sales">{l(text("Check live sales", "Kagua mauzo hai"))}</Link></div></aside> : null}

      {displayedProblem && !reversalOpen ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>{displayedProblem.code}</strong>{displayedProblem.detail}{displayedProblem.correlation_id ? <small>Correlation · {compactId(displayedProblem.correlation_id)}</small> : null}</span></div> : null}

      {reversalOpen ? <section className="reversal-panel" aria-labelledby="reversal-title"><div className="reversal-warning"><TriangleAlert size={21} /><div><h2 id="reversal-title">{l(text("Confirm linked reversal", "Thibitisha ubatilisho uliounganishwa"))}</h2><p>{l(text("This posts opposite stock, customer, payment and accounting effects. It does not delete or edit the original sale.", "Hii huchapisha athari kinyume za bidhaa, mteja, malipo na uhasibu. Haifuti wala kuhariri mauzo asili."))}</p></div></div><label><span>{l(text("Reason for reversal", "Sababu ya ubatilisho"))}</span><textarea value={reason} onChange={(event) => { setReason(event.target.value); setProblem(null); }} rows={3} maxLength={500} placeholder={l(text("Describe the verified business reason…", "Eleza sababu ya biashara iliyothibitishwa…"))} /></label><label className="confirmation-check"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} /><span>{l(text("I confirm that this sale must be reversed and the original must remain immutable.", "Ninathibitisha mauzo haya yabatilishwe na rekodi asili ibaki bila kubadilishwa."))}</span></label>{displayedProblem ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>{displayedProblem.code}</strong>{displayedProblem.detail}{displayedProblem.correlation_id ? <small>Correlation · {compactId(displayedProblem.correlation_id)}</small> : null}</span></div> : null}<div className="reversal-actions"><button type="button" className="secondary-button" disabled={submitting || completed} onClick={() => { setReversalOpen(false); setProblem(null); }}>{l(text("Keep original", "Baki na rekodi asili"))}</button><button type="button" className="danger-button" disabled={submitting || completed || recoveryBlocked || reason.trim().length < 5 || !confirmed} onClick={submitReversal}>{submitting ? l(text("Posting reversal…", "Inachapisha ubatilisho…")) : pendingRecovery ? l(text("Retry same reversal", "Jaribu ubatilisho uleule")) : l(text("Post linked reversal", "Chapisha ubatilisho"))}</button></div></section> : null}

      <div className="detail-layout">
        <div className="detail-main">
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><ReceiptText size={15} /> {l(text("Authoritative values", "Thamani rasmi"))}</span><h2>{l(text("Sale totals", "Jumla ya mauzo"))}</h2></div></div><dl className="live-total-grid"><div><dt>{l(text("Subtotal", "Jumla ndogo"))}</dt><dd>{formatMinorUnits(sale.subtotal_minor, sale.currency, locale)}</dd></div><div><dt>{l(text("Tax", "Kodi"))}</dt><dd>{formatMinorUnits(sale.tax_minor, sale.currency, locale)}</dd></div><div className="total-emphasis"><dt>{l(text("Posted total", "Jumla iliyochapishwa"))}</dt><dd>{formatMinorUnits(sale.total_minor, sale.currency, locale)}</dd></div></dl></section>
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><ReceiptText size={15} /> {sale.lines.length} {l(text("lines", "mistari"))}</span><h2>{l(text("Posted products", "Bidhaa zilizochapishwa"))}</h2></div></div><div className="table-scroll"><table className="ledger-table sale-lines-table"><thead><tr><th>{l(text("Product", "Bidhaa"))}</th><th>{l(text("Qty", "Idadi"))}</th><th>{l(text("Unit price", "Bei ya kipimo"))}</th><th>{l(text("Tax", "Kodi"))}</th><th>{l(text("Total", "Jumla"))}</th></tr></thead><tbody>{sale.lines.map((line) => { const product = products.get(line.product_id); return <tr key={line.id}><td><strong>{product?.name ?? compactId(line.product_id)}</strong><small className="table-subline">{product?.code ?? compactId(line.product_id)}</small></td><td>{line.quantity}</td><td>{formatMinorUnits(line.unit_price_minor, sale.currency, locale)}</td><td>{formatMinorUnits(line.tax_minor, sale.currency, locale)}</td><td>{formatMinorUnits(line.total_minor, sale.currency, locale)}</td></tr>; })}</tbody></table></div></section>
        </div>
        <aside className="detail-side">
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><ShieldCheck size={15} /> {l(text("Scope", "Upeo"))}</span><h2>{l(text("Posting context", "Muktadha wa uchapishaji"))}</h2></div></div><dl className="live-context-details"><div><dt>{l(text("Company", "Kampuni"))}</dt><dd>{workspace.context.company_name}</dd></div><div><dt>{l(text("Branch", "Tawi"))}</dt><dd>{workspace.context.branch_name}</dd></div><div><dt>{l(text("Warehouse", "Ghala"))}</dt><dd title={sale.scope.warehouse_id}>{workspace.context.warehouse_name}</dd></div><div><dt>{l(text("Customer", "Mteja"))}</dt><dd>{customer?.name ?? compactId(sale.customer_id)}</dd></div><div><dt>{l(text("Sale kind", "Aina ya mauzo"))}</dt><dd>{sale.kind}</dd></div><div><dt>{l(text("Payment", "Malipo"))}</dt><dd>{sale.payment_method || (sale.kind === "CREDIT" ? l(text("Receivable", "Deni la kupokea")) : "—")}</dd></div></dl></section>
          <section className="card detail-card fiscal-truth-card"><div className="card-heading"><div><span className="card-kicker"><ReceiptText size={15} /> {l(text("Receipt status", "Hali ya risiti"))}</span><h2>{l(text("Internal receipt & fiscal workflow", "Risiti ya ndani na mchakato wa fiskali"))}</h2></div><span className={`fiscal-status fiscal-${sale.fiscal_status.toLowerCase().replaceAll("_", "-")}`}>{l(fiscalCopy)}</span></div><dl className="reference-list"><div><dt>{l(text("Internal receipt reference", "Rejea ya risiti ya ndani"))}</dt><dd title={sale.receipt_reference}>{sale.receipt_reference}</dd></div><div><dt>{l(text("Fiscal status", "Hali ya fiskali"))}</dt><dd>{sale.fiscal_status}</dd></div></dl><div className="fiscal-truth-note"><TriangleAlert size={16} /><p>{sale.fiscal_status === "FISCALIZED" ? l(text("The API marks this workflow fiscalized. The reference shown above is still an internal receipt reference, not a TRA fiscal receipt number.", "API inaonyesha mchakato huu umefiskalishwa. Rejea hapo juu bado ni ya risiti ya ndani, si namba ya risiti ya TRA.")) : l(text("No TRA fiscal receipt number is available on this record. Never present the internal reference as a TRA fiscal receipt.", "Hakuna namba ya risiti ya TRA kwenye rekodi hii. Usitumie rejea ya ndani kama risiti ya TRA."))}</p></div></section>
          <section className="card detail-card"><div className="card-heading"><div><span className="card-kicker"><History size={15} /> {l(text("Trace", "Ufuatiliaji"))}</span><h2>{l(text("Immutable references", "Rejea zisizobadilika"))}</h2></div></div><dl className="reference-list"><div><dt>Sale ID</dt><dd title={sale.id}>{sale.id}</dd></div><div><dt>Correlation ID</dt><dd title={sale.correlation_id}>{sale.correlation_id}</dd></div><div><dt>{l(text("Document time", "Muda wa hati"))}</dt><dd>{formatTimestamp(sale.document_at, locale, workspace.context.timezone)}</dd></div><div><dt>{l(text("Server receipt", "Mapokezi ya seva"))}</dt><dd>{formatTimestamp(sale.received_at, locale, workspace.context.timezone)}</dd></div><div><dt>{l(text("Accounting time", "Muda wa uhasibu"))}</dt><dd>{formatTimestamp(sale.accounting_at, locale, workspace.context.timezone)}<small>{sale.accounting_time_basis}</small></dd></div><div><dt>{l(text("Created by", "Imeundwa na"))}</dt><dd title={sale.created_by}>{sale.created_by}</dd></div>{sale.reversal_of ? <div><dt>{l(text("Reversal of", "Ubatilisho wa"))}</dt><dd><Link href={`/sales/${sale.reversal_of}`}>{compactId(sale.reversal_of)}</Link></dd></div> : null}{sale.reversal_reason ? <div><dt>{l(text("Reversal reason", "Sababu ya ubatilisho"))}</dt><dd>{sale.reversal_reason}</dd></div> : null}</dl><div className="balanced-banner"><Check size={16} />{l(text("Read-only values returned by the live ERP", "Thamani za kusoma tu kutoka ERP hai"))}</div></section>
        </aside>
      </div>
    </div>
  );
}
