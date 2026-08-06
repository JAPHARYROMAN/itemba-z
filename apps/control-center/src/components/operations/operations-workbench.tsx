"use client";

import Link from "next/link";
import { useRef, useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import { formatMinorUnits } from "@/live-api/format";
import { minorUnitsToMajorInput, parseMajorUnitsToMinor } from "@/live-api/money-input";
import {
  clearPendingCommandAfterSuccess,
  markPendingCommandRejected,
  operationCreatePendingScope,
  operationTransitionPendingScope,
  PendingCommandConflictError,
  PendingCommandStorageError,
  reservePendingCommand,
} from "@/live-api/pending-command";
import type { CreateOperationCommand, CustomerCollection, OperationDocument, OperationDocumentType, OperationStatus, OperationsWorkspace, PaymentMethod, PublicProblem, TransitionOperationCommand } from "@/live-api/types";
import { canAccessSalesRoute } from "@/lib/sales-permissions";

type Mode = "sales" | "purchases" | "inventory";
const TYPES: Record<Mode, OperationDocumentType[]> = { sales: ["QUOTATION", "SALES_ORDER"], purchases: ["PURCHASE_REQUEST", "PURCHASE_ORDER", "GOODS_RECEIPT", "SUPPLIER_INVOICE", "SUPPLIER_PAYMENT", "PURCHASE_RETURN"], inventory: ["STOCK_TRANSFER", "STOCK_COUNT", "STOCK_ADJUSTMENT"] };
const TITLES = { sales: text("Quotes & orders", "Nukuu na oda"), purchases: text("Procure to pay", "Manunuzi hadi malipo"), inventory: text("Inventory operations", "Shughuli za bidhaa") };
const INTROS = {
  sales: text("Prepare customer quotes, approve orders, and fulfil them from one place.", "Andaa nukuu za wateja, idhinisha oda na uzitimize katika sehemu moja."),
  purchases: text("Immutable documents, approval evidence, and atomic ledger effects.", "Nyaraka zisizobadilika, ushahidi wa idhini, na athari za leja kwa pamoja."),
  inventory: text("Immutable documents, approval evidence, and atomic ledger effects.", "Nyaraka zisizobadilika, ushahidi wa idhini, na athari za leja kwa pamoja."),
};

function businessLabel(value: string): string {
  const words = value.toLocaleLowerCase().replaceAll("_", " ");
  return words.charAt(0).toLocaleUpperCase() + words.slice(1);
}

function sourceNumber(documents: OperationDocument[], sourceId?: string): string {
  if (!sourceId) return "—";
  return documents.find((document) => document.id === sourceId)?.number ?? "Linked document";
}

function isPublicProblem(value: unknown): value is PublicProblem {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string");
}

function isOperationDocument(value: unknown): value is OperationDocument {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).id === "string" && typeof (value as Record<string, unknown>).status === "string");
}

function pendingCommandProblem(error: unknown): PublicProblem {
  const detail = error instanceof PendingCommandConflictError
    ? "Another operation is awaiting an authoritative result. Retry that exact operation before starting a different one."
    : error instanceof PendingCommandStorageError
      ? error.message
      : "The operation identity could not be preserved, so nothing was sent.";
  return { type: "about:blank", title: "Operation not sent", status: 409, code: "pending_operation_conflict", detail };
}

function responseProblem(value: unknown, status: number): PublicProblem {
  return isPublicProblem(value)
    ? value
    : { type: "about:blank", title: "Operation rejected", status, code: "operation_rejected", detail: "The live ERP rejected the operation command." };
}

function nextActions(document: OperationDocument): OperationStatus[] {
  if (document.status === "DRAFT") return ["SUBMITTED"];
  if (document.status === "SUBMITTED") return ["APPROVED", "REJECTED"];
  if (document.status === "DISPATCHED" && document.type === "STOCK_TRANSFER") return ["RECEIVED"];
  if (document.status !== "APPROVED") return [];
  if (["QUOTATION", "PURCHASE_REQUEST", "PURCHASE_ORDER"].includes(document.type)) return ["CLOSED"];
  if (document.type === "SALES_ORDER") return [];
  if (document.type === "STOCK_TRANSFER") return ["DISPATCHED"];
  return ["POSTED"];
}

export function OperationsWorkbench({ mode, workspace, view = "all" }: { mode: Mode; workspace: OperationsWorkspace; view?: "all" | "documents" | "collections" }) {
  const { l, locale } = useLanguage(); const allowed = TYPES[mode];
  const [documents, setDocuments] = useState(() => workspace.documents.filter((item) => allowed.includes(item.type)));
  const [type, setType] = useState<OperationDocumentType>(allowed[0]); const [partyId, setPartyId] = useState(""); const [sourceId, setSourceId] = useState(""); const [destinationId, setDestinationId] = useState("");
  const [productId, setProductId] = useState(workspace.products[0]?.id ?? ""); const [quantity, setQuantity] = useState(1); const [priceInput, setPriceInput] = useState(() => minorUnitsToMajorInput(workspace.products[0]?.unit_price_minor ?? 0));
  const [reason, setReason] = useState("Operational document created for approved business workflow"); const [busy, setBusy] = useState(false); const [problem, setProblem] = useState<PublicProblem | null>(null);
  const [collectionCustomer, setCollectionCustomer] = useState(""); const [invoiceSale, setInvoiceSale] = useState(""); const [collectionAmount, setCollectionAmount] = useState(0); const [collectionMethod, setCollectionMethod] = useState<PaymentMethod>("BANK_TRANSFER"); const [collection, setCollection] = useState<CustomerCollection | null>(null);
  const partyType = ["QUOTATION", "SALES_ORDER"].includes(type) ? "CUSTOMER" : ["PURCHASE_ORDER", "GOODS_RECEIPT", "SUPPLIER_INVOICE", "SUPPLIER_PAYMENT", "PURCHASE_RETURN"].includes(type) ? "SUPPLIER" : "NONE";
  const showDocuments = view !== "collections";
  const showCollections = mode === "sales" && view !== "documents";
  const canManageDocuments = mode !== "sales" || workspace.context.permissions.includes("sales.orders.manage");
  const canFulfilSalesOrder = mode === "sales" && canAccessSalesRoute(workspace.context.permissions, "new");
  const priceMinor = parseMajorUnitsToMinor(priceInput);
  const commandInFlight = useRef(false);

  async function createDocument(event: React.FormEvent) {
    event.preventDefault();
    if (commandInFlight.current) return;
    const inventoryDocument = ["STOCK_TRANSFER", "STOCK_COUNT", "STOCK_ADJUSTMENT"].includes(type);
    if (!inventoryDocument && priceMinor === null) {
      setProblem({ type: "about:blank", title: "Price invalid", status: 400, code: "operation_price_invalid", detail: "Enter a non-negative unit price with no more than two decimal places." });
      return;
    }
    const command: CreateOperationCommand = { type, party_type: partyType, currency: workspace.context.currency, reason, lines: [{ product_id: productId, quantity, unit_price_minor: inventoryDocument ? 0 : priceMinor! }] };
    if (partyType !== "NONE") command.party_id = partyId; if (sourceId) command.source_document_id = sourceId; if (type === "STOCK_TRANSFER") command.destination_warehouse_id = destinationId;
    const scope = operationCreatePendingScope(workspace.context, mode);
    let pending;
    try { pending = reservePendingCommand({ storage: window.sessionStorage, scope, fingerprint: JSON.stringify(command), payload: command }); }
    catch (error) { setProblem(pendingCommandProblem(error)); return; }
    commandInFlight.current = true; setBusy(true); setProblem(null);
    try {
      const response = await fetch("/api/live/operations", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key }, body: JSON.stringify(command) });
      const payload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) markPendingCommandRejected(window.sessionStorage, scope, pending.record.key, response.status);
        setProblem(responseProblem(payload, response.status)); return;
      }
      if (!isOperationDocument(payload)) throw new Error("Incomplete operation response");
      clearPendingCommandAfterSuccess(window.sessionStorage, scope, pending.record.key);
      setDocuments((current) => [payload, ...current]);
    } catch { setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "operation_connection_interrupted", detail: "The same operation and protected retry identity are preserved. Retry to safely retrieve or complete the authoritative result." }); }
    finally { commandInFlight.current = false; setBusy(false); }
  }

  async function transition(document: OperationDocument, status: OperationStatus) {
    if (commandInFlight.current) return;
    const command: TransitionOperationCommand = { status, reason: `Authorized ${status.toLowerCase()} lifecycle transition`, payment_method: document.type === "SUPPLIER_PAYMENT" && status === "POSTED" ? "BANK_TRANSFER" : undefined };
    const scope = operationTransitionPendingScope(workspace.context, document.id);
    let pending;
    try { pending = reservePendingCommand({ storage: window.sessionStorage, scope, fingerprint: JSON.stringify(command), payload: command }); }
    catch (error) { setProblem(pendingCommandProblem(error)); return; }
    commandInFlight.current = true; setBusy(true); setProblem(null);
    try {
      const response = await fetch(`/api/live/operations/${document.id}/transitions`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key }, body: JSON.stringify(command) });
      const payload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) markPendingCommandRejected(window.sessionStorage, scope, pending.record.key, response.status);
        setProblem(responseProblem(payload, response.status)); return;
      }
      if (!isOperationDocument(payload)) throw new Error("Incomplete operation response");
      clearPendingCommandAfterSuccess(window.sessionStorage, scope, pending.record.key);
      setDocuments((current) => current.map((item) => item.id === document.id ? payload : item));
    } catch { setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "operation_connection_interrupted", detail: "The same transition and protected retry identity are preserved. Retry to safely retrieve or complete the authoritative result." }); }
    finally { commandInFlight.current = false; setBusy(false); }
  }

  async function receiveCollection(event: React.FormEvent) { event.preventDefault(); setBusy(true); setProblem(null); try { const response=await fetch(`/api/live/customers/${collectionCustomer}/collections`,{method:"POST",headers:{"Content-Type":"application/json","Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({invoice_sale_id:invoiceSale,method:collectionMethod,amount_minor:collectionAmount,currency:workspace.context.currency})}); const payload=await response.json();if(!response.ok)throw payload;setCollection(payload as CustomerCollection);}catch(error){setProblem(error as PublicProblem);}finally{setBusy(false);} }

  return <div className="page-stack operations-workbench">
    <header className="module-header"><div><p className="eyebrow">{l(mode === "sales" ? text("Sales workspace", "Eneo la mauzo") : text("Governed operations", "Shughuli zinazosimamiwa"))}</p><h1>{l(TITLES[mode])}</h1><p>{l(INTROS[mode])}</p></div><LiveBadge context={workspace.context} /></header>
    {showDocuments && canManageDocuments ? <section id="new-document" tabIndex={-1} className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Create controlled document", "Unda hati inayodhibitiwa"))}</h2><p>{l(text("Enter normal business amounts. Prices and scope are revalidated by the server.", "Weka kiasi cha kawaida cha biashara. Bei na upeo huhakikiwa tena na seva."))}</p></div></div>
      <form className="operation-form" onSubmit={createDocument}>
        <label>{l(text("Document type", "Aina ya hati"))}<select value={type} onChange={(event) => { setType(event.target.value as OperationDocumentType); setPartyId(""); setSourceId(""); }}>{allowed.map((value) => <option key={value} value={value}>{businessLabel(value)}</option>)}</select></label>
        {partyType === "CUSTOMER" ? <label>{l(text("Customer", "Mteja"))}<select required value={partyId} onChange={(event) => setPartyId(event.target.value)}><option value="">{l(text("Select active customer", "Chagua mteja hai"))}</option>{workspace.customers.filter((item) => item.status === "active").map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label> : null}
        {partyType === "SUPPLIER" ? <label>{l(text("Supplier", "Msambazaji"))}<select required value={partyId} onChange={(event) => setPartyId(event.target.value)}><option value="">{l(text("Select active supplier", "Chagua msambazaji hai"))}</option>{workspace.suppliers.filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label> : null}
        <label>{l(text("Source document", "Hati chanzo"))}<select value={sourceId} onChange={(event) => setSourceId(event.target.value)}><option value="">{l(text("No linked document", "Hakuna hati iliyounganishwa"))}</option>{documents.map((document) => <option key={document.id} value={document.id}>{document.number} · {businessLabel(document.type)}</option>)}</select></label>
        {type === "STOCK_TRANSFER" ? <label>{l(text("Destination warehouse", "Ghala lengwa"))}<input required value={destinationId} onChange={(event) => setDestinationId(event.target.value)} placeholder="UUID" /></label> : null}
        <label>{l(text("Product", "Bidhaa"))}<select value={productId} onChange={(event) => { const id = event.target.value; setProductId(id); setPriceInput(minorUnitsToMajorInput(workspace.products.find((item) => item.id === id)?.unit_price_minor ?? 0)); }}>{workspace.products.map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label>
        <label>{l(text(type === "STOCK_COUNT" ? "Counted quantity" : "Quantity", type === "STOCK_COUNT" ? "Kiasi kilichohesabiwa" : "Kiasi"))}<input type="number" value={quantity} onChange={(event) => setQuantity(Number(event.target.value))} /></label>
        {!(["STOCK_TRANSFER", "STOCK_COUNT", "STOCK_ADJUSTMENT"] as string[]).includes(type) ? <label>{l(text("Unit price", "Bei kwa kipimo"))}<input type="text" inputMode="decimal" value={priceInput} aria-invalid={priceMinor === null} onChange={(event) => setPriceInput(event.target.value)} /></label> : null}
        <label className="operation-reason">{l(text("Business reason", "Sababu ya biashara"))}<textarea minLength={8} required value={reason} onChange={(event) => setReason(event.target.value)} /></label>
        <button className="primary-button" disabled={busy || !productId || priceMinor === null} type="submit">{busy ? l(text("Processing…", "Inachakata…")) : l(text("Create draft", "Unda rasimu"))}</button>
      </form>{problem ? <div className="sale-problem" role="alert"><span><strong>{l(text("The document could not be updated", "Hati haikuweza kusasishwa"))}</strong>{problem.detail ?? l(text("The command was rejected.", "Amri imekataliwa."))}<details><summary>{l(text("System details", "Maelezo ya mfumo"))}</summary><code>{problem.code ?? "operation_failed"}</code></details></span></div> : null}
    </section> : null}
    {showCollections ? <section id="collections" tabIndex={-1} className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Receive customer collection", "Pokea malipo ya mteja"))}</h2><p>{l(text("Allocate payment to one open credit invoice and post cash against receivables.", "Tenga malipo kwa ankara moja ya mkopo na chapisha fedha dhidi ya madeni."))}</p></div></div><form className="operation-form collection-form" onSubmit={receiveCollection}><label>{l(text("Customer", "Mteja"))}<select required value={collectionCustomer} onChange={(event)=>setCollectionCustomer(event.target.value)}><option value="">{l(text("Select customer", "Chagua mteja"))}</option>{workspace.customers.filter((item)=>!item.is_general_customer).map((item)=><option value={item.id} key={item.id}>{item.code} · {item.name}</option>)}</select></label><label>{l(text("Credit invoice sale ID", "Kitambulisho cha ankara ya mkopo"))}<input required value={invoiceSale} onChange={(event)=>setInvoiceSale(event.target.value)} placeholder="UUID" /></label><label>{l(text("Amount (minor)", "Kiasi (senti)"))}<input required min="1" type="number" value={collectionAmount} onChange={(event)=>setCollectionAmount(Number(event.target.value))} /></label><label>{l(text("Method", "Njia"))}<select value={collectionMethod} onChange={(event)=>setCollectionMethod(event.target.value as PaymentMethod)}><option>BANK_TRANSFER</option><option>CASH</option><option>MOBILE_MONEY</option><option>BANK_CARD</option></select></label><button disabled={busy} className="primary-button" type="submit">{l(text("Post collection", "Chapisha malipo"))}</button></form>{collection?<p className="collection-success" role="status">{l(text("Collection posted", "Malipo yamechapishwa"))} · {collection.id}</p>:null}</section> : null}
    {showDocuments ? <section id="documents" tabIndex={-1} className="card records-card"><div className="records-toolbar"><div><h2>{l(text("Lifecycle documents", "Nyaraka za mzunguko"))}</h2><span>{documents.length}</span></div></div><div className="table-scroll"><table className="data-table operations-table"><thead><tr><th>{l(text("Document", "Hati"))}</th><th>{l(text("Status", "Hali"))}</th><th>{l(text("Source", "Chanzo"))}</th><th className="align-right">{l(text("Total", "Jumla"))}</th><th>{l(text("Controlled action", "Hatua inayodhibitiwa"))}</th></tr></thead><tbody>{documents.map((document) => <tr key={document.id}><td><div className="record-primary"><span>{document.number}</span><strong>{businessLabel(document.type)}</strong><small>{document.reason}</small></div></td><td><span className="reconciliation-status status-active">{businessLabel(document.status)}</span></td><td>{sourceNumber(documents, document.source_document_id)}</td><td className="align-right numeric">{formatMinorUnits(document.total_minor, document.currency, locale)}</td><td><div className="operation-actions">{canManageDocuments ? nextActions(document).map((status) => <button className="secondary-button" disabled={busy} key={status} onClick={() => transition(document, status)}>{businessLabel(status)}</button>) : null}{document.type === "SALES_ORDER" && document.status === "APPROVED" && canFulfilSalesOrder ? <Link className="secondary-button" href={`/sales/new?order=${encodeURIComponent(document.id)}`}>{l(text("Fulfil order", "Timiza oda"))}</Link> : null}</div></td></tr>)}</tbody></table></div></section> : null}
  </div>;
}
