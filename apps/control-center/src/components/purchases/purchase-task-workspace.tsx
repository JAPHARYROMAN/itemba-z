"use client";

import Link from "next/link";
import { useMemo, useRef, useState } from "react";
import { CheckCircle2, CircleAlert, FilePlus2, Plus, Search, Trash2 } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { PurchaseModuleNav } from "@/components/purchases/purchase-module-nav";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import { minorUnitsToMajorInput, parseMajorUnitsToMinor } from "@/live-api/money-input";
import { clearPendingCommandAfterSuccess, markPendingCommandRejected, operationCreatePendingScope, operationTransitionPendingScope, PendingCommandConflictError, reservePendingCommand } from "@/live-api/pending-command";
import type { CreateOperationCommand, OperationDocument, OperationStatus, PaymentMethod, PublicProblem, PurchaseWorkspace, TransitionOperationCommand } from "@/live-api/types";
import { text } from "@/lib/i18n";
import type { PurchaseRoute } from "@/lib/purchase-permissions";
import { canManagePurchaseType } from "@/lib/purchase-permissions";
import { eligibleSources, nextPurchaseStatuses, purchaseStatusLabel, purchaseStatusTone, purchaseTypeLabel, PURCHASE_TYPE_BY_ROUTE, sourceIsRequired, sourceTypeFor } from "@/lib/purchase-presentation";
import styles from "./purchases.module.css";

type TaskRoute = Exclude<PurchaseRoute, "overview" | "sourcing">;
type DraftLine = { key: string; productId: string; quantity: string; price: string };

const taskText: Record<TaskRoute, { title: [string, string]; intro: [string, string]; verb: [string, string] }> = {
  requests: { title: ["Purchase requests", "Maombi ya ununuzi"], intro: ["Capture an internal need before supplier commitment.", "Rekodi hitaji la ndani kabla ya kujitolea kwa msambazaji."], verb: ["Create request", "Unda ombi"] },
  orders: { title: ["Purchase orders", "Oda za ununuzi"], intro: ["Commit an approved quantity and price to an active supplier.", "Thibitisha kiasi na bei iliyoidhinishwa kwa msambazaji hai."], verb: ["Create order", "Unda oda"] },
  receipts: { title: ["Goods receipts", "Mapokezi ya bidhaa"], intro: ["Record delivered goods against an approved purchase order.", "Rekodi bidhaa zilizowasili dhidi ya oda iliyoidhinishwa."], verb: ["Record receipt", "Rekodi mapokezi"] },
  bills: { title: ["Supplier bills", "Ankara za wasambazaji"], intro: ["Match a supplier bill to goods already received.", "Linganisha ankara ya msambazaji na bidhaa zilizopokelewa."], verb: ["Record bill", "Rekodi ankara"] },
  payments: { title: ["Supplier payments", "Malipo ya wasambazaji"], intro: ["Settle a posted supplier bill through an approved channel.", "Lipa ankara ya msambazaji iliyochapishwa kupitia njia iliyoidhinishwa."], verb: ["Prepare payment", "Andaa malipo"] },
  returns: { title: ["Purchase returns", "Marejesho ya ununuzi"], intro: ["Correct a posted purchase through a linked return.", "Sahihisha ununuzi uliochapishwa kupitia marejesho yaliyounganishwa."], verb: ["Create return", "Unda marejesho"] },
};

function isProblem(value: unknown): value is PublicProblem { return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string"); }
function isDocument(value: unknown): value is OperationDocument { return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).id === "string"); }

export function PurchaseTaskWorkspace({ workspace, route }: { workspace: PurchaseWorkspace; route: TaskRoute }) {
  const { locale, l } = useLanguage();
  const type = PURCHASE_TYPE_BY_ROUTE[route];
  const copy = taskText[route];
  const [documents, setDocuments] = useState(workspace.documents);
  const [sourceId, setSourceId] = useState("");
  const [supplierId, setSupplierId] = useState("");
  const [reason, setReason] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("BANK_TRANSFER");
  const [lines, setLines] = useState<DraftLine[]>(() => [newLine(workspace, "line-1")]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<OperationStatus | "ALL">("ALL");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const [created, setCreated] = useState<OperationDocument | null>(null);
  const inFlight = useRef(false);
  const sources = useMemo(() => eligibleSources(documents, type), [documents, type]);
  const selectedSource = sources.find((document) => document.id === sourceId);
  const currentDocuments = useMemo(() => documents.filter((document) => document.type === type && (status === "ALL" || document.status === status) && (!query.trim() || document.number.toLocaleLowerCase("en").includes(query.trim().toLocaleLowerCase("en")) || document.reason.toLocaleLowerCase("en").includes(query.trim().toLocaleLowerCase("en")))), [documents, query, status, type]);
  const sourceRequired = sourceIsRequired(type);
  const hasSupplier = type !== "PURCHASE_REQUEST";
  const locksLines = Boolean(selectedSource) && type !== "PURCHASE_ORDER";
  const parsedLines = lines.map((line) => ({ product_id: line.productId, quantity: Number(line.quantity), unit_price_minor: parseMajorUnitsToMinor(line.price) }));
  const totalMinor = parsedLines.reduce<number | null>((sum, line) => {
    if (sum === null || line.unit_price_minor === null || !Number.isSafeInteger(line.quantity)) return null;
    const lineTotal = line.quantity * line.unit_price_minor;
    const nextTotal = sum + lineTotal;
    return Number.isSafeInteger(lineTotal) && Number.isSafeInteger(nextTotal) ? nextTotal : null;
  }, 0);
  const valid = reason.trim().length >= 8 && (!hasSupplier || Boolean(supplierId)) && (!sourceRequired || Boolean(sourceId)) && lines.length > 0 && totalMinor !== null && parsedLines.every((line) => line.product_id && Number.isSafeInteger(line.quantity) && line.quantity > 0 && line.unit_price_minor !== null && Number.isSafeInteger(line.quantity * line.unit_price_minor));
  const canManage = canManagePurchaseType(workspace.context.permissions, type);

  function selectSource(nextId: string) {
    setSourceId(nextId); setCreated(null);
    const source = sources.find((document) => document.id === nextId);
    if (!source) { if (sourceRequired) setSupplierId(""); setLines([newLine(workspace, "line-1")]); return; }
    if (source.party_id) setSupplierId(source.party_id);
    setLines(source.lines.map((line) => ({ key: crypto.randomUUID(), productId: line.product_id, quantity: String(line.quantity), price: minorUnitsToMajorInput(line.unit_price_minor || workspace.products.find((product) => product.id === line.product_id)?.unit_price_minor || 0) })));
  }

  function updateLine(key: string, patch: Partial<DraftLine>) { setLines((current) => current.map((line) => line.key === key ? { ...line, ...patch } : line)); }

  async function create(event: React.FormEvent) {
    event.preventDefault();
    if (!valid || inFlight.current) return;
    const command: CreateOperationCommand = { type, party_type: hasSupplier ? "SUPPLIER" : "NONE", currency: workspace.context.currency, reason: reason.trim(), lines: parsedLines.map((line) => ({ product_id: line.product_id, quantity: line.quantity, unit_price_minor: line.unit_price_minor! })) };
    if (hasSupplier) command.party_id = supplierId;
    if (sourceId) command.source_document_id = sourceId;
    const scope = operationCreatePendingScope(workspace.context, `purchases.${route}`);
    let pending;
    try { pending = reservePendingCommand({ storage: window.sessionStorage, scope, fingerprint: JSON.stringify(command), payload: command }); }
    catch (error) { setProblem({ type: "about:blank", title: "Document not sent", status: 409, code: "pending_purchase_document", detail: error instanceof PendingCommandConflictError ? l(text("A different document still awaits an authoritative result. Retry that exact document first.", "Hati tofauti bado inasubiri matokeo rasmi. Jaribu hati hiyo kwanza.")) : l(text("The safe retry identity could not be preserved.", "Utambulisho salama wa kujaribu tena haukuweza kuhifadhiwa.")) }); return; }
    inFlight.current = true; setBusy(true); setProblem(null); setCreated(null);
    try {
      const response = await fetch("/api/live/operations", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key }, body: JSON.stringify(command) });
      const payload: unknown = await response.json();
      if (!response.ok) { if (response.status >= 400 && response.status < 500 && response.status !== 409) markPendingCommandRejected(window.sessionStorage, scope, pending.record.key, response.status); setProblem(isProblem(payload) ? payload : { type: "about:blank", title: "Document rejected", status: response.status, code: "purchase_document_rejected", detail: l(text("The ERP rejected this document.", "ERP imekataa hati hii.")) }); return; }
      if (!isDocument(payload)) throw new Error("Incomplete document response");
      clearPendingCommandAfterSuccess(window.sessionStorage, scope, pending.record.key); setDocuments((current) => [payload, ...current]); setCreated(payload); setReason("");
    } catch { setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "purchase_connection_interrupted", detail: l(text("The result is not confirmed. Your safe retry identity is preserved.", "Matokeo hayajathibitishwa. Utambulisho salama wa kujaribu tena umehifadhiwa.")) }); }
    finally { inFlight.current = false; setBusy(false); }
  }

  async function transition(document: OperationDocument, nextStatus: OperationStatus) {
    if (inFlight.current) return;
    const command: TransitionOperationCommand = { status: nextStatus, reason: `Authorized ${purchaseStatusLabel(nextStatus, "en").toLocaleLowerCase()} purchasing decision`, payment_method: document.type === "SUPPLIER_PAYMENT" && nextStatus === "POSTED" ? paymentMethod : undefined };
    const scope = operationTransitionPendingScope(workspace.context, document.id);
    let pending;
    try { pending = reservePendingCommand({ storage: window.sessionStorage, scope, fingerprint: JSON.stringify(command), payload: command }); }
    catch { setProblem({ type: "about:blank", title: "Decision not sent", status: 409, code: "pending_purchase_transition", detail: l(text("This record has a decision awaiting confirmation. Retry the same decision first.", "Rekodi hii ina uamuzi unaosubiri uthibitisho. Jaribu uamuzi huo kwanza.")) }); return; }
    inFlight.current = true; setBusy(true); setProblem(null);
    try {
      const response = await fetch(`/api/live/operations/${encodeURIComponent(document.id)}/transitions`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key }, body: JSON.stringify(command) });
      const payload: unknown = await response.json();
      if (!response.ok) { if (response.status >= 400 && response.status < 500 && response.status !== 409) markPendingCommandRejected(window.sessionStorage, scope, pending.record.key, response.status); setProblem(isProblem(payload) ? payload : { type: "about:blank", title: "Decision rejected", status: response.status, code: "purchase_transition_rejected", detail: l(text("The ERP rejected this decision.", "ERP imekataa uamuzi huu.")) }); return; }
      if (!isDocument(payload)) throw new Error("Incomplete transition response");
      clearPendingCommandAfterSuccess(window.sessionStorage, scope, pending.record.key); setDocuments((current) => current.map((item) => item.id === payload.id ? payload : item));
    } catch { setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "purchase_transition_interrupted", detail: l(text("The decision is not confirmed. Retry the same action safely.", "Uamuzi haujathibitishwa. Jaribu hatua hiyo tena kwa usalama.")) }); }
    finally { inFlight.current = false; setBusy(false); }
  }

  return <div className={styles.page}>
    <PurchaseModuleNav permissions={workspace.context.permissions} />
    <header className={styles.registerHeader}><div><p className={styles.scope}>{workspace.context.company_name} · {workspace.context.branch_name}</p><h1>{l(text(copy.title[0], copy.title[1]))}</h1><p>{l(text(copy.intro[0], copy.intro[1]))}</p></div></header>
    {canManage ? <div className={styles.composer}><section className={`${styles.card} ${styles.formCard}`}><div className={styles.formHeader}><div><h2>{l(text(copy.verb[0], copy.verb[1]))}</h2><p>{sourceTypeFor(type) ? l(text("Choose the approved source record; supplier and lines stay linked to its evidence.", "Chagua rekodi chanzo iliyoidhinishwa; msambazaji na mistari hubaki zimeunganishwa na ushahidi wake.")) : l(text("Describe the business need and estimated products before approval.", "Eleza hitaji la biashara na bidhaa zinazokadiriwa kabla ya idhini."))}</p></div><FilePlus2 size={20} /></div><form className={styles.form} onSubmit={create}>
      <div className={styles.fieldGrid}>{sourceTypeFor(type) ? <label className={styles.wideField}><span>{l(text(`Source ${purchaseTypeLabel(sourceTypeFor(type)!, "en").toLocaleLowerCase()}`, `Chanzo: ${purchaseTypeLabel(sourceTypeFor(type)!, "sw")}`))}</span><select required={sourceRequired} value={sourceId} onChange={(event) => selectSource(event.target.value)}><option value="">{sourceRequired ? l(text("Select approved source", "Chagua chanzo kilichoidhinishwa")) : l(text("No source document", "Hakuna hati chanzo"))}</option>{sources.map((source) => <option key={source.id} value={source.id}>{source.number} · {formatMinorUnits(source.total_minor, source.currency, locale)}</option>)}</select></label> : null}{hasSupplier ? <label className={styles.wideField}><span>{l(text("Supplier", "Msambazaji"))}</span><select required disabled={Boolean(selectedSource?.party_id)} value={supplierId} onChange={(event) => setSupplierId(event.target.value)}><option value="">{l(text("Select active supplier", "Chagua msambazaji hai"))}</option>{workspace.suppliers.filter((supplier) => supplier.active).map((supplier) => <option key={supplier.id} value={supplier.id}>{supplier.code} · {supplier.name}</option>)}</select></label> : null}{route === "payments" ? <label className={styles.field}><span>{l(text("Settlement channel", "Njia ya malipo"))}</span><select value={paymentMethod} onChange={(event) => setPaymentMethod(event.target.value as PaymentMethod)}><option value="BANK_TRANSFER">{l(text("Bank transfer", "Uhamisho wa benki"))}</option><option value="MOBILE_MONEY">{l(text("Mobile money", "Pesa ya simu"))}</option><option value="BANK_CARD">{l(text("Bank card", "Kadi ya benki"))}</option><option value="CASH">{l(text("Cash", "Taslimu"))}</option></select></label> : null}</div>
      <section className={styles.lineEditor}><div className={styles.lineEditorHeader}><h3>{l(text("Products", "Bidhaa"))}</h3>{!locksLines ? <button type="button" onClick={() => setLines((current) => [...current, newLine(workspace)])}><Plus size={16} />{l(text("Add line", "Ongeza mstari"))}</button> : null}</div>{lines.map((line, index) => <div className={styles.lineRow} key={line.key}><label className={styles.field}><span>{l(text("Product", "Bidhaa"))}</span><select disabled={locksLines} value={line.productId} onChange={(event) => { const productId = event.target.value; const product = workspace.products.find((item) => item.id === productId); updateLine(line.key, { productId, price: minorUnitsToMajorInput(product?.unit_price_minor ?? 0) }); }}>{workspace.products.map((product) => <option key={product.id} value={product.id}>{product.code} · {product.name}</option>)}</select></label><label className={styles.field}><span>{l(text("Quantity", "Kiasi"))}</span><input type="number" min="1" step="1" disabled={locksLines && route === "payments"} value={line.quantity} onChange={(event) => updateLine(line.key, { quantity: event.target.value })} /></label><label className={styles.field}><span>{l(text("Unit price", "Bei kwa kipimo"))}</span><input inputMode="decimal" disabled={locksLines} value={line.price} onChange={(event) => updateLine(line.key, { price: event.target.value })} aria-invalid={parseMajorUnitsToMinor(line.price) === null} /></label><button type="button" disabled={locksLines || lines.length === 1} onClick={() => setLines((current) => current.filter((item) => item.key !== line.key))} aria-label={`${l(text("Remove product line", "Ondoa mstari wa bidhaa"))} ${index + 1}`}><Trash2 size={16} /></button></div>)}</section>
      <label className={styles.wideField}><span>{l(text("Business reason", "Sababu ya biashara"))}</span><textarea required minLength={8} maxLength={500} value={reason} onChange={(event) => setReason(event.target.value)} placeholder={l(text("Explain why this purchase is required", "Eleza kwa nini ununuzi huu unahitajika"))} /></label>
      {problem ? <div className={styles.problem} role="alert"><CircleAlert size={19} /><div><strong>{l(text("The purchase record was not updated", "Rekodi ya ununuzi haikusasishwa"))}</strong><p>{problem.detail}</p><details><summary>{l(text("System details", "Maelezo ya mfumo"))}</summary><small>{problem.code}{problem.correlation_id ? ` · ${problem.correlation_id}` : ""}</small></details></div></div> : null}
      {created ? <div className={styles.successNotice} role="status"><CheckCircle2 size={19} /><div><strong>{l(text("Draft created", "Rasimu imeundwa"))}</strong><p>{created.number} · <Link href={`/purchases/${created.id}`}>{l(text("Review document", "Kagua hati"))}</Link></p></div></div> : null}
      <button className={styles.primaryAction} disabled={!valid || busy} type="submit">{busy ? l(text("Recording…", "Inarekodi…")) : l(text(copy.verb[0], copy.verb[1]))}</button>
    </form></section><aside className={`${styles.card} ${styles.summaryCard}`}><h2>{l(text("Review", "Kagua"))}</h2><p>{l(text("The ERP validates scope, source matching, stock, payable balance, and posting configuration before committing effects.", "ERP inathibitisha upeo, ulinganifu wa chanzo, hisa, salio la deni, na usanidi wa uchapishaji kabla ya kuthibitisha athari."))}</p><dl className={styles.summary}><div><dt>{l(text("Document", "Hati"))}</dt><dd>{purchaseTypeLabel(type, locale)}</dd></div><div><dt>{l(text("Source", "Chanzo"))}</dt><dd>{selectedSource?.number ?? "—"}</dd></div><div><dt>{l(text("Products", "Bidhaa"))}</dt><dd>{lines.length}</dd></div><div><dt>{l(text("Estimated total", "Jumla inayokadiriwa"))}</dt><dd>{totalMinor === null ? l(text("Check line values", "Kagua thamani za mistari")) : formatMinorUnits(totalMinor, workspace.context.currency, locale)}</dd></div></dl></aside></div> : null}
    <section className={styles.card} id="documents"><div className={styles.toolbar}><div><h2>{l(text(`${purchaseTypeLabel(type, "en")} register`, `Rejesta ya ${purchaseTypeLabel(type, "sw").toLocaleLowerCase()}`))}</h2><p aria-live="polite">{currentDocuments.length} {l(text("records", "rekodi"))}</p></div><div className={styles.filters}><label className={styles.search}><Search size={16} /><span className="sr-only">{l(text("Search documents", "Tafuta nyaraka"))}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={l(text("Search number or reason", "Tafuta namba au sababu"))} /></label><label className={styles.select}><span className="sr-only">{l(text("Filter by status", "Chuja kwa hali"))}</span><select value={status} onChange={(event) => setStatus(event.target.value as OperationStatus | "ALL")}><option value="ALL">{l(text("All statuses", "Hali zote"))}</option><option value="DRAFT">{purchaseStatusLabel("DRAFT", locale)}</option><option value="SUBMITTED">{purchaseStatusLabel("SUBMITTED", locale)}</option><option value="APPROVED">{purchaseStatusLabel("APPROVED", locale)}</option><option value="POSTED">{purchaseStatusLabel("POSTED", locale)}</option><option value="CLOSED">{purchaseStatusLabel("CLOSED", locale)}</option><option value="REJECTED">{purchaseStatusLabel("REJECTED", locale)}</option></select></label></div></div>{currentDocuments.length ? <div className={styles.tableRegion} tabIndex={0} role="region" aria-label={l(text("Purchase documents", "Nyaraka za ununuzi"))}><table className={styles.table}><thead><tr><th>{l(text("Document", "Hati"))}</th><th>{l(text("Supplier / source", "Msambazaji / chanzo"))}</th><th>{l(text("Status", "Hali"))}</th><th className={styles.alignRight}>{l(text("Total", "Jumla"))}</th><th>{l(text("Controlled action", "Hatua inayodhibitiwa"))}</th></tr></thead><tbody>{currentDocuments.map((document) => { const supplier = workspace.suppliers.find((item) => item.id === document.party_id); const source = documents.find((item) => item.id === document.source_document_id); const tone = purchaseStatusTone(document.status); return <tr key={document.id}><td><Link className={styles.recordLink} href={`/purchases/${document.id}`}>{document.number}<small>{formatTimestamp(document.created_at, locale, workspace.context.timezone)}</small></Link></td><td data-label={l(text("Supplier / source", "Msambazaji / chanzo"))}>{supplier?.name ?? l(text("Internal request", "Ombi la ndani"))}<small className={styles.meta}>{source?.number ?? "—"}</small></td><td data-label={l(text("Status", "Hali"))}><span className={`${styles.status} ${styles[tone]}`}>{purchaseStatusLabel(document.status, locale)}</span></td><td data-label={l(text("Total", "Jumla"))} className={styles.alignRight}>{formatMinorUnits(document.total_minor, document.currency, locale)}</td><td data-label={l(text("Controlled action", "Hatua"))}><div className={styles.actions}>{canManage ? nextPurchaseStatuses(document).map((next) => <button key={next} type="button" disabled={busy} onClick={() => transition(document, next)}>{purchaseStatusLabel(next, locale)}</button>) : null}</div></td></tr>; })}</tbody></table></div> : <div className={styles.empty}><FilePlus2 size={30} /><strong>{l(text("No records match this view", "Hakuna rekodi inayolingana na mwonekano huu"))}</strong><p>{l(text("Create the first draft or adjust the search and status filter.", "Unda rasimu ya kwanza au badilisha utafutaji na kichujio cha hali."))}</p></div>}</section>
  </div>;
}

function newLine(workspace: PurchaseWorkspace, key = crypto.randomUUID()): DraftLine {
  const product = workspace.products[0];
  return { key, productId: product?.id ?? "", quantity: "1", price: minorUnitsToMajorInput(product?.unit_price_minor ?? 0) };
}
