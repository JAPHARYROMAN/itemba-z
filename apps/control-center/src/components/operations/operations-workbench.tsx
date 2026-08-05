"use client";

import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import type { CreateOperationCommand, CustomerCollection, OperationDocument, OperationDocumentType, OperationStatus, OperationsWorkspace, PaymentMethod, PublicProblem } from "@/live-api/types";

type Mode = "sales" | "purchases" | "inventory";
const TYPES: Record<Mode, OperationDocumentType[]> = { sales: ["QUOTATION", "SALES_ORDER"], purchases: ["PURCHASE_REQUEST", "PURCHASE_ORDER", "GOODS_RECEIPT", "SUPPLIER_INVOICE", "SUPPLIER_PAYMENT", "PURCHASE_RETURN"], inventory: ["STOCK_TRANSFER", "STOCK_COUNT", "STOCK_ADJUSTMENT"] };
const TITLES = { sales: text("Sales lifecycle", "Mzunguko wa mauzo"), purchases: text("Procure to pay", "Manunuzi hadi malipo"), inventory: text("Inventory operations", "Shughuli za bidhaa") };

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

export function OperationsWorkbench({ mode, workspace }: { mode: Mode; workspace: OperationsWorkspace }) {
  const { l } = useLanguage(); const allowed = TYPES[mode];
  const [documents, setDocuments] = useState(() => workspace.documents.filter((item) => allowed.includes(item.type)));
  const [type, setType] = useState<OperationDocumentType>(allowed[0]); const [partyId, setPartyId] = useState(""); const [sourceId, setSourceId] = useState(""); const [destinationId, setDestinationId] = useState("");
  const [productId, setProductId] = useState(workspace.products[0]?.id ?? ""); const [quantity, setQuantity] = useState(1); const [price, setPrice] = useState(workspace.products[0]?.unit_price_minor ?? 0);
  const [reason, setReason] = useState("Operational document created for approved business workflow"); const [busy, setBusy] = useState(false); const [problem, setProblem] = useState<PublicProblem | null>(null);
  const [collectionCustomer, setCollectionCustomer] = useState(""); const [invoiceSale, setInvoiceSale] = useState(""); const [collectionAmount, setCollectionAmount] = useState(0); const [collectionMethod, setCollectionMethod] = useState<PaymentMethod>("BANK_TRANSFER"); const [collection, setCollection] = useState<CustomerCollection | null>(null);
  const partyType = ["QUOTATION", "SALES_ORDER"].includes(type) ? "CUSTOMER" : ["PURCHASE_ORDER", "GOODS_RECEIPT", "SUPPLIER_INVOICE", "SUPPLIER_PAYMENT", "PURCHASE_RETURN"].includes(type) ? "SUPPLIER" : "NONE";

  async function createDocument(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setProblem(null);
    const command: CreateOperationCommand = { type, party_type: partyType, currency: workspace.context.currency, reason, lines: [{ product_id: productId, quantity, unit_price_minor: ["STOCK_TRANSFER", "STOCK_COUNT", "STOCK_ADJUSTMENT"].includes(type) ? 0 : price }] };
    if (partyType !== "NONE") command.party_id = partyId; if (sourceId) command.source_document_id = sourceId; if (type === "STOCK_TRANSFER") command.destination_warehouse_id = destinationId;
    try { const response = await fetch("/api/live/operations", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify(command) }); const payload = await response.json(); if (!response.ok) throw payload; setDocuments((current) => [payload as OperationDocument, ...current]); }
    catch (error) { setProblem(error as PublicProblem); } finally { setBusy(false); }
  }

  async function transition(document: OperationDocument, status: OperationStatus) {
    setBusy(true); setProblem(null);
    try { const response = await fetch(`/api/live/operations/${document.id}/transitions`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ status, reason: `Authorized ${status.toLowerCase()} lifecycle transition`, payment_method: document.type === "SUPPLIER_PAYMENT" && status === "POSTED" ? "BANK_TRANSFER" : undefined }) }); const payload = await response.json(); if (!response.ok) throw payload; setDocuments((current) => current.map((item) => item.id === document.id ? payload as OperationDocument : item)); }
    catch (error) { setProblem(error as PublicProblem); } finally { setBusy(false); }
  }

  async function receiveCollection(event: React.FormEvent) { event.preventDefault(); setBusy(true); setProblem(null); try { const response=await fetch(`/api/live/customers/${collectionCustomer}/collections`,{method:"POST",headers:{"Content-Type":"application/json","Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({invoice_sale_id:invoiceSale,method:collectionMethod,amount_minor:collectionAmount,currency:workspace.context.currency})}); const payload=await response.json();if(!response.ok)throw payload;setCollection(payload as CustomerCollection);}catch(error){setProblem(error as PublicProblem);}finally{setBusy(false);} }

  return <main className="page-stack operations-workbench">
    <header className="module-header"><div><p className="eyebrow">{l(text("Governed operations", "Shughuli zinazosimamiwa"))}</p><h1>{l(TITLES[mode])}</h1><p>{l(text("Immutable documents, approval evidence, and atomic ledger effects.", "Nyaraka zisizobadilika, ushahidi wa idhini, na athari za leja kwa pamoja."))}</p></div><LiveBadge context={workspace.context} /></header>
    <section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Create controlled document", "Unda hati inayodhibitiwa"))}</h2><p>{l(text("Amounts use minor currency units; prices and scope are revalidated by the server.", "Kiasi hutumia senti; bei na upeo huhakikiwa tena na seva."))}</p></div></div>
      <form className="operation-form" onSubmit={createDocument}>
        <label>{l(text("Document type", "Aina ya hati"))}<select value={type} onChange={(event) => { setType(event.target.value as OperationDocumentType); setPartyId(""); setSourceId(""); }}>{allowed.map((value) => <option key={value}>{value}</option>)}</select></label>
        {partyType === "CUSTOMER" ? <label>{l(text("Customer", "Mteja"))}<select required value={partyId} onChange={(event) => setPartyId(event.target.value)}><option value="">{l(text("Select active customer", "Chagua mteja hai"))}</option>{workspace.customers.filter((item) => item.status === "active").map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label> : null}
        {partyType === "SUPPLIER" ? <label>{l(text("Supplier", "Msambazaji"))}<select required value={partyId} onChange={(event) => setPartyId(event.target.value)}><option value="">{l(text("Select active supplier", "Chagua msambazaji hai"))}</option>{workspace.suppliers.filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label> : null}
        <label>{l(text("Source document", "Hati chanzo"))}<input value={sourceId} onChange={(event) => setSourceId(event.target.value)} placeholder={l(text("Linked upstream document UUID", "UUID ya hati ya awali"))} /></label>
        {type === "STOCK_TRANSFER" ? <label>{l(text("Destination warehouse", "Ghala lengwa"))}<input required value={destinationId} onChange={(event) => setDestinationId(event.target.value)} placeholder="UUID" /></label> : null}
        <label>{l(text("Product", "Bidhaa"))}<select value={productId} onChange={(event) => { const id = event.target.value; setProductId(id); setPrice(workspace.products.find((item) => item.id === id)?.unit_price_minor ?? 0); }}>{workspace.products.map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label>
        <label>{l(text(type === "STOCK_COUNT" ? "Counted quantity" : "Quantity", type === "STOCK_COUNT" ? "Kiasi kilichohesabiwa" : "Kiasi"))}<input type="number" value={quantity} onChange={(event) => setQuantity(Number(event.target.value))} /></label>
        {!(["STOCK_TRANSFER", "STOCK_COUNT", "STOCK_ADJUSTMENT"] as string[]).includes(type) ? <label>{l(text("Unit price (minor)", "Bei kwa kipimo (senti)"))}<input type="number" min="0" value={price} onChange={(event) => setPrice(Number(event.target.value))} /></label> : null}
        <label className="operation-reason">{l(text("Business reason", "Sababu ya biashara"))}<textarea minLength={8} required value={reason} onChange={(event) => setReason(event.target.value)} /></label>
        <button className="primary-button" disabled={busy || !productId} type="submit">{busy ? l(text("Processing…", "Inachakata…")) : l(text("Create draft", "Unda rasimu"))}</button>
      </form>{problem ? <div className="sale-problem" role="alert"><span><strong>{problem.code ?? "operation_failed"}</strong>{problem.detail ?? l(text("The command was rejected.", "Amri imekataliwa."))}</span></div> : null}
    </section>
    {mode === "sales" ? <section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Receive customer collection", "Pokea malipo ya mteja"))}</h2><p>{l(text("Allocate payment to one open credit invoice and post cash against receivables.", "Tenga malipo kwa ankara moja ya mkopo na chapisha fedha dhidi ya madeni."))}</p></div></div><form className="operation-form collection-form" onSubmit={receiveCollection}><label>{l(text("Customer", "Mteja"))}<select required value={collectionCustomer} onChange={(event)=>setCollectionCustomer(event.target.value)}><option value="">{l(text("Select customer", "Chagua mteja"))}</option>{workspace.customers.filter((item)=>!item.is_general_customer).map((item)=><option value={item.id} key={item.id}>{item.code} · {item.name}</option>)}</select></label><label>{l(text("Credit invoice sale ID", "Kitambulisho cha ankara ya mkopo"))}<input required value={invoiceSale} onChange={(event)=>setInvoiceSale(event.target.value)} placeholder="UUID" /></label><label>{l(text("Amount (minor)", "Kiasi (senti)"))}<input required min="1" type="number" value={collectionAmount} onChange={(event)=>setCollectionAmount(Number(event.target.value))} /></label><label>{l(text("Method", "Njia"))}<select value={collectionMethod} onChange={(event)=>setCollectionMethod(event.target.value as PaymentMethod)}><option>BANK_TRANSFER</option><option>CASH</option><option>MOBILE_MONEY</option><option>BANK_CARD</option></select></label><button disabled={busy} className="primary-button" type="submit">{l(text("Post collection", "Chapisha malipo"))}</button></form>{collection?<p className="collection-success" role="status">{l(text("Collection posted", "Malipo yamechapishwa"))} · {collection.id}</p>:null}</section> : null}
    <section className="card records-card"><div className="records-toolbar"><div><h2>{l(text("Lifecycle documents", "Nyaraka za mzunguko"))}</h2><span>{documents.length}</span></div></div><div className="table-scroll"><table className="data-table operations-table"><thead><tr><th>{l(text("Document", "Hati"))}</th><th>{l(text("Status", "Hali"))}</th><th>{l(text("Source", "Chanzo"))}</th><th className="align-right">{l(text("Total", "Jumla"))}</th><th>{l(text("Controlled action", "Hatua inayodhibitiwa"))}</th></tr></thead><tbody>{documents.map((document) => <tr key={document.id}><td><div className="record-primary"><span>{document.number}</span><strong>{document.type}</strong><small>{document.reason}</small></div></td><td><span className="reconciliation-status status-active">{document.status}</span></td><td className="numeric">{document.source_document_id ?? "—"}</td><td className="align-right numeric">{document.total_minor.toLocaleString()}</td><td><div className="operation-actions">{nextActions(document).map((status) => <button className="secondary-button" disabled={busy} key={status} onClick={() => transition(document, status)}>{status}</button>)}{document.type === "SALES_ORDER" && document.status === "APPROVED" ? <small>{l(text("Fulfil from New Sale using this source ID", "Timiza kupitia Mauzo Mapya kwa kutumia kitambulisho hiki"))}</small> : null}</div></td></tr>)}</tbody></table></div></section>
  </main>;
}
