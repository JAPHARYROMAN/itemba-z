"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Check, ChevronLeft, History, Minus, PackageSearch, Plus, ReceiptText, Search, ShieldCheck, ShoppingCart, TriangleAlert } from "lucide-react";
import { useDeferredValue, useMemo, useState } from "react";
import type { CompleteSaleCommand, PaymentMethod, PublicProblem, SalesBootstrap } from "@/live-api/types";
import { compactId, formatMinorUnits, formatTimestamp } from "@/live-api/format";
import { safeIntegerSum } from "@/live-api/integer-safety";
import { cartEstimatedTotalMinor, cartLineTotalMinor } from "@/live-api/sales-math";
import {
  clearPendingCommandAfterSuccess,
  markPendingCommandRejected,
  PendingCommandConflictError,
  PendingCommandStorageError,
  reservePendingCommand,
  saleCompletionPendingScope,
  type PendingCommandSnapshot,
} from "@/live-api/pending-command";
import { usePendingCommand } from "@/live-api/use-pending-command";
import { LiveBadge } from "@/components/live-sales/live-state";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";

interface CartLine { productId: string; quantity: number }
type SaleKind = CompleteSaleCommand["kind"];
const PAYMENT_METHOD_LABELS = {
  CASH: text("Cash", "Fedha"),
  MOBILE_MONEY: text("Mobile money", "Pesa ya simu"),
  BANK_CARD: text("Bank card", "Kadi ya benki"),
  BANK_TRANSFER: text("Bank transfer", "Uhamisho wa benki"),
} satisfies Record<PaymentMethod, ReturnType<typeof text>>;
const PAYMENT_METHODS = Object.keys(PAYMENT_METHOD_LABELS) as PaymentMethod[];

function isPublicProblem(value: unknown): value is PublicProblem {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string");
}

function hasSaleId(value: unknown): value is { id: string } {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).id === "string" && String((value as Record<string, unknown>).id).length > 0);
}

function isPaymentMethod(value: string): value is PaymentMethod {
  return PAYMENT_METHODS.some((method) => method === value);
}

function isCompleteSaleCommand(value: unknown): value is CompleteSaleCommand {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Record<string, unknown>;
  if (typeof candidate.customer_id !== "string" || (candidate.kind !== "CASH" && candidate.kind !== "CREDIT") || !Array.isArray(candidate.lines) || candidate.lines.length === 0) return false;
  if (candidate.payment_method !== undefined && (typeof candidate.payment_method !== "string" || !isPaymentMethod(candidate.payment_method))) return false;
  return candidate.lines.every((line) => {
    if (!line || typeof line !== "object") return false;
    const item = line as Record<string, unknown>;
    return typeof item.product_id === "string" && typeof item.quantity === "number" && Number.isSafeInteger(item.quantity) && item.quantity > 0;
  });
}

export function LiveSaleEntry({ bootstrap, initialSourceDocumentId = "" }: { bootstrap: SalesBootstrap; initialSourceDocumentId?: string }) {
  return <ScopedLiveSaleEntry key={`${saleCompletionPendingScope(bootstrap.context)}:${initialSourceDocumentId}`} bootstrap={bootstrap} initialSourceDocumentId={initialSourceDocumentId} />;
}

function ScopedLiveSaleEntry({ bootstrap, initialSourceDocumentId }: { bootstrap: SalesBootstrap; initialSourceDocumentId: string }) {
  const { locale, l } = useLanguage();
  const router = useRouter();
  const initialOrder = bootstrap.documents.find((document) => document.id === initialSourceDocumentId && document.type === "SALES_ORDER" && document.status === "APPROVED");
  const [kind, setKind] = useState<SaleKind>("CASH");
  const [customerId, setCustomerId] = useState(initialOrder?.party_id ?? "");
  const [sourceDocumentId, setSourceDocumentId] = useState(initialOrder?.id ?? "");
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("CASH");
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [cart, setCart] = useState<CartLine[]>(() => initialOrder?.lines.map((line) => ({ productId: line.product_id, quantity: line.quantity })) ?? []);
  const [reviewing, setReviewing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const pendingScope = saleCompletionPendingScope(bootstrap.context);
  const pendingStore = usePendingCommand<unknown>(pendingScope);

  const productsById = useMemo(() => new Map(bootstrap.products.map((product) => [product.id, product])), [bootstrap.products]);
  const customersById = useMemo(() => new Map(bootstrap.customers.map((customer) => [customer.id, customer])), [bootstrap.customers]);
  const approvedOrders = useMemo(
    () => bootstrap.documents.filter((document) => document.type === "SALES_ORDER" && document.status === "APPROVED"),
    [bootstrap.documents],
  );
  const selectedOrder = approvedOrders.find((document) => document.id === sourceDocumentId);
  const selectedCustomer = bootstrap.customers.find((customer) => customer.id === customerId);
  const visibleProducts = useMemo(() => {
    const normalized = deferredQuery.trim().toLocaleLowerCase();
    if (!normalized) return bootstrap.products;
    return bootstrap.products.filter((product) => `${product.code} ${product.name}`.toLocaleLowerCase().includes(normalized));
  }, [bootstrap.products, deferredQuery]);
  const estimatedTotal = useMemo(() => cartEstimatedTotalMinor(cart, productsById), [cart, productsById]);
  const creditAllowed = Boolean(selectedCustomer && !selectedCustomer.is_general_customer && selectedCustomer.credit_enabled);
  const pendingRecovery = pendingStore.snapshot && isCompleteSaleCommand(pendingStore.snapshot.record.payload)
    ? pendingStore.snapshot as PendingCommandSnapshot<CompleteSaleCommand>
    : null;
  const pendingReferencesKnown = Boolean(pendingRecovery
    && bootstrap.customers.some((customer) => customer.id === pendingRecovery.record.payload.customer_id)
    && pendingRecovery.record.payload.lines.every((line) => bootstrap.products.some((product) => product.id === line.product_id)));
  const recoveryProblem: PublicProblem | null = pendingStore.error
    ? { type: "about:blank", title: "Recovery unavailable", status: 503, code: "pending_sale_storage_unavailable", detail: pendingStore.error.message }
    : pendingStore.snapshot && !pendingRecovery
      ? { type: "about:blank", title: "Recovery blocked", status: 409, code: "pending_sale_invalid", detail: "A preserved sale marker cannot be reconstructed safely. Do not post another sale in this tab until the live sales register is reconciled." }
      : pendingRecovery && !pendingReferencesKnown
        ? { type: "about:blank", title: "Recovery needs reconciliation", status: 409, code: "pending_sale_master_data_changed", detail: "The preserved sale refers to master data that is no longer available. Check the live sales register before continuing." }
        : null;
  const recoveryBlocked = Boolean(recoveryProblem);
  const displayedProblem = problem ?? recoveryProblem;
  const canReview = Boolean(selectedCustomer && cart.length > 0 && estimatedTotal !== null && (kind === "CASH" ? paymentMethod : creditAllowed) && !recoveryBlocked && !completed);

  function restorePendingSale(command: CompleteSaleCommand): boolean {
    const knownCustomer = bootstrap.customers.some((customer) => customer.id === command.customer_id);
    const knownProducts = command.lines.every((line) => bootstrap.products.some((product) => product.id === line.product_id));
    if (!knownCustomer || !knownProducts) return false;
    setKind(command.kind);
    setCustomerId(command.customer_id);
	setSourceDocumentId(command.source_document_id ?? "");
    setPaymentMethod(command.payment_method || "CASH");
    setCart(command.lines.map((line) => ({ productId: line.product_id, quantity: line.quantity })));
    setReviewing(true);
    setProblem(null);
    return true;
  }

  function selectKind(nextKind: SaleKind) {
    setKind(nextKind);
    setProblem(null);
    setReviewing(false);
    if (nextKind === "CREDIT" && selectedCustomer && (selectedCustomer.is_general_customer || !selectedCustomer.credit_enabled)) setCustomerId("");
  }

  function selectSourceDocument(documentId: string) {
    setSourceDocumentId(documentId);
    setReviewing(false);
    setProblem(null);
    const document = approvedOrders.find((candidate) => candidate.id === documentId);
    if (!document) {
      setCustomerId("");
      setCart([]);
      return;
    }
    setCustomerId(document.party_id ?? "");
    setCart(document.lines.map((line) => ({ productId: line.product_id, quantity: line.quantity })));
  }

  function changeQuantity(productId: string, change: number) {
    setReviewing(false);
    setProblem(null);
    setCart((current) => {
      const product = productsById.get(productId);
      if (!product) return current;
      const found = current.find((line) => line.productId === productId);
      const changedQuantity = safeIntegerSum([found?.quantity ?? 0, change]);
      if (changedQuantity === null) return current;
      const nextQuantity = Math.min(product.available_quantity, Math.max(0, changedQuantity));
      if (nextQuantity === 0) return current.filter((line) => line.productId !== productId);
      return found
        ? current.map((line) => line.productId === productId ? { ...line, quantity: nextQuantity } : line)
        : [...current, { productId, quantity: nextQuantity }];
    });
  }

  function buildCommand(): CompleteSaleCommand {
    return {
      customer_id: customerId,
	  ...(sourceDocumentId.trim() ? { source_document_id: sourceDocumentId.trim() } : {}),
      kind,
      ...(kind === "CASH" ? { payment_method: paymentMethod } : {}),
      lines: cart.map((line) => ({ product_id: line.productId, quantity: line.quantity })),
    };
  }

  async function postSale() {
    if (!canReview) return;
    const command = buildCommand();
    const fingerprint = JSON.stringify(command);
    let pending: PendingCommandSnapshot<CompleteSaleCommand>;
    try {
      pending = reservePendingCommand({ storage: window.sessionStorage, scope: pendingScope, fingerprint, payload: command });
    } catch (error) {
      const detail = error instanceof PendingCommandConflictError
        ? "A different sale is still awaiting an authoritative result. Restore that exact sale or reconcile it in the live sales register before posting another."
        : error instanceof PendingCommandStorageError
          ? error.message
          : "Safe retry information could not be preserved, so the sale was not sent. Start the sale again.";
      setProblem({ type: "about:blank", title: "Sale not sent", status: 409, code: "pending_sale_conflict", detail });
      return;
    }
    setSubmitting(true);
    setProblem(null);
    try {
      const response = await fetch("/api/live/sales", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key, Accept: "application/json" },
        body: fingerprint,
      });
      const payload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) {
          try {
            markPendingCommandRejected<CompleteSaleCommand>(window.sessionStorage, pendingScope, pending.record.key, response.status);
          } catch {
            // The preserved original key remains the safe retry identity even if metadata cannot be updated.
          }
        }
        setProblem(isPublicProblem(payload) ? payload : { type: "about:blank", title: "Sale rejected", status: response.status, code: "sale_rejected", detail: "The live ERP rejected the sale command." });
        return;
      }
      if (!hasSaleId(payload)) throw new Error("The successful sale response did not contain an ID.");
      const sale = payload;
      try {
        clearPendingCommandAfterSuccess(window.sessionStorage, pendingScope, pending.record.key);
      } catch {
        // A replay remains safe because the backend retains committed idempotency records.
      }
      setCompleted(true);
      router.push(`/sales/${sale.id}`);
      router.refresh();
    } catch {
      setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "bff_unavailable", detail: "We safely preserved this sale attempt. Retry to confirm or complete the same sale without posting it twice." });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page-stack live-sale-entry-page">
      <Link href="/sales" className="back-link"><ArrowLeft size={16} />{l(text("Back to live sales", "Rudi kwenye mauzo hai"))}</Link>
      <section className="page-heading">
        <div><div className="heading-badges"><LiveBadge context={bootstrap.context} /><span className="scope-chip">{bootstrap.context.branch_name}</span></div><h1>{l(text("Complete controlled sale", "Kamilisha mauzo yaliyodhibitiwa"))}</h1><p>{l(text("Choose the customer and quantities. The live ERP owns price, tax, stock, credit and every posting effect.", "Chagua mteja na idadi. ERP hai inadhibiti bei, kodi, bidhaa, mkopo na athari zote za uchapishaji."))}</p></div>
      </section>

      {pendingRecovery ? <aside className={`pending-command-banner ${pendingRecovery.expired ? "pending-command-stale" : ""}`} role="status"><History size={19} /><div><strong>{l(text("Unconfirmed sale recovered", "Mauzo yasiyothibitishwa yamerejeshwa"))}</strong><p>{pendingRecovery.record.outcome === "rejected" ? l(text("The ERP rejected the previous attempt without posting. Restore it before making any correction; its safe retry details will be retained.", "ERP ilikataa jaribio la awali bila kuchapisha. Irejeshe kabla ya marekebisho; maelezo salama ya kujaribu tena yatahifadhiwa.")) : l(text("The customer and quantities from this sale attempt are safely preserved in this browser tab. Restore and retry to confirm or complete the same sale without posting it twice.", "Mteja na idadi za jaribio hili la mauzo zimehifadhiwa salama kwenye kichupo hiki. Rejesha na ujaribu tena kuthibitisha au kukamilisha mauzo yale yale bila kuyachapisha mara mbili."))}</p><small>{l(text("Preserved", "Imehifadhiwa"))} · {formatTimestamp(new Date(pendingRecovery.record.createdAt).toISOString(), locale, bootstrap.context.timezone)}{pendingRecovery.expired ? ` · ${l(text("reconciliation recommended", "upatanisho unapendekezwa"))}` : ""}</small></div><div className="pending-command-actions"><button type="button" className="secondary-button" onClick={() => restorePendingSale(pendingRecovery.record.payload)}>{l(text("Restore exact sale", "Rejesha mauzo halisi"))}</button><Link className="secondary-button" href="/sales">{l(text("Check live sales", "Kagua mauzo hai"))}</Link></div></aside> : null}

      <div className="sale-entry-layout">
        <div className="sale-entry-main">
          <section className="card sale-compose-card">
            <div className="sale-step-heading"><span>1</span><div><strong>{l(text("Sale policy", "Sera ya mauzo"))}</strong><small>{l(text("Cash or online-only credit", "Fedha au mkopo wa mtandaoni tu"))}</small></div></div>
            <fieldset className="sale-kind-picker"><legend className="sr-only">{l(text("Sale kind", "Aina ya mauzo"))}</legend><button type="button" className={kind === "CASH" ? "active" : ""} aria-pressed={kind === "CASH"} onClick={() => selectKind("CASH")}><ReceiptText size={18} /><span><strong>{l(text("Cash sale", "Mauzo ya fedha"))}</strong><small>{l(text("Payment posts now", "Malipo yanachapishwa sasa"))}</small></span></button><button type="button" className={kind === "CREDIT" ? "active" : ""} aria-pressed={kind === "CREDIT"} onClick={() => selectKind("CREDIT")}><ShieldCheck size={18} /><span><strong>{l(text("Credit sale", "Mauzo ya mkopo"))}</strong><small>{l(text("Eligible accounts only", "Akaunti zinazostahili tu"))}</small></span></button></fieldset>

            <div className="sale-field-grid">
              <label><span>{l(text("Customer", "Mteja"))}</span><select disabled={Boolean(selectedOrder)} value={customerId} onChange={(event) => { setCustomerId(event.target.value); setReviewing(false); setProblem(null); }}><option value="">{l(text("Select an active customer", "Chagua mteja hai"))}</option>{bootstrap.customers.filter((customer) => customer.status === "active").map((customer) => <option key={customer.id} value={customer.id} disabled={kind === "CREDIT" && (customer.is_general_customer || !customer.credit_enabled)}>{customer.code} · {customer.name}{customer.is_general_customer ? ` · ${l(text("Cash only", "Fedha tu"))}` : ""}</option>)}</select></label>
              {kind === "CASH" ? <label><span>{l(text("Payment method", "Njia ya malipo"))}</span><select value={paymentMethod} onChange={(event) => { if (isPaymentMethod(event.target.value)) setPaymentMethod(event.target.value); setReviewing(false); }}>{PAYMENT_METHODS.map((method) => <option key={method} value={method}>{l(PAYMENT_METHOD_LABELS[method])}</option>)}</select></label> : <div className={`credit-policy ${selectedCustomer && !creditAllowed ? "credit-policy-blocked" : ""}`}><span>{l(text("Available credit", "Mkopo unaopatikana"))}</span><strong>{selectedCustomer ? formatMinorUnits(selectedCustomer.available_credit_minor, bootstrap.context.currency, locale) : "—"}</strong><small>{selectedCustomer ? (creditAllowed ? l(text("Account eligible", "Akaunti inastahili")) : l(text("Credit is not allowed", "Mkopo hauruhusiwi"))) : l(text("Select an eligible customer", "Chagua mteja anayestahili"))}</small></div>}
			  <label><span>{l(text("Approved sales order (optional)", "Oda ya mauzo iliyoidhinishwa (hiari)"))}</span><select value={sourceDocumentId} onChange={(event) => selectSourceDocument(event.target.value)}><option value="">{approvedOrders.length ? l(text("Choose an approved order", "Chagua oda iliyoidhinishwa")) : l(text("No approved orders available", "Hakuna oda zilizoidhinishwa"))}</option>{approvedOrders.map((document) => <option key={document.id} value={document.id}>{document.number} · {customersById.get(document.party_id ?? "")?.name ?? l(text("Customer", "Mteja"))}</option>)}</select>{selectedOrder ? <small>{l(text("Customer and quantities are locked to the approved order.", "Mteja na idadi zimefungwa kulingana na oda iliyoidhinishwa."))}</small> : null}</label>
            </div>
          </section>

          <section className="card sale-compose-card">
            <div className="sale-step-heading"><span>2</span><div><strong>{l(text("Products & quantities", "Bidhaa na idadi"))}</strong><small>{l(text("Availability from authenticated warehouse", "Upatikanaji kutoka ghala lililothibitishwa"))}</small></div></div>
            <label className="product-search"><Search size={17} /><span className="sr-only">{l(text("Search products", "Tafuta bidhaa"))}</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={l(text("Search code or product…", "Tafuta namba au bidhaa…"))} /></label>
            <div className="product-picker">{visibleProducts.length ? visibleProducts.map((product) => {
              const line = cart.find((item) => item.productId === product.id);
              return <article key={product.id}><div><span>{product.code} · {product.unit}</span><strong>{product.name}</strong><small>{formatMinorUnits(product.unit_price_minor, product.currency, locale)} · {product.available_quantity} {l(text("available", "zinapatikana"))}</small></div><div className="quantity-control"><button type="button" onClick={() => changeQuantity(product.id, -1)} disabled={Boolean(selectedOrder) || !line} aria-label={`${l(text("Remove one", "Ondoa moja"))} ${product.name}`}><Minus size={15} /></button><output aria-live="polite">{line?.quantity ?? 0}</output><button type="button" onClick={() => changeQuantity(product.id, 1)} disabled={Boolean(selectedOrder) || (line?.quantity ?? 0) >= product.available_quantity || product.available_quantity < 1} aria-label={`${l(text("Add one", "Ongeza moja"))} ${product.name}`}><Plus size={15} /></button></div></article>;
            }) : <div className="live-empty compact"><PackageSearch size={25} /><strong>{l(text("No live products match", "Hakuna bidhaa hai zinazolingana"))}</strong></div>}</div>
          </section>
        </div>

        <aside className="card sale-cart-card">
          <div className="sale-cart-heading"><span><ShoppingCart size={18} /></span><div><strong>{l(text("Sale summary", "Muhtasari wa mauzo"))}</strong><small>{cart.length} {l(text("product lines", "mistari ya bidhaa"))}</small></div></div>
          {cart.length ? <div className="cart-lines">{cart.map((line) => { const product = productsById.get(line.productId); if (!product) return null; const lineTotal = cartLineTotalMinor(product.unit_price_minor, line.quantity); return <div key={line.productId}><div><strong>{product.name}</strong><small>{product.code} · {line.quantity} × {formatMinorUnits(product.unit_price_minor, product.currency, locale)}</small></div><span>{lineTotal === null ? "—" : formatMinorUnits(lineTotal, product.currency, locale)}</span></div>; })}</div> : <div className="cart-empty"><ShoppingCart size={25} /><p>{l(text("Add at least one available product.", "Ongeza angalau bidhaa moja inayopatikana."))}</p></div>}
          <dl className="cart-totals"><div><dt>{l(text("Estimated product total", "Jumla ya makadirio"))}</dt><dd>{estimatedTotal === null ? l(text("Unavailable", "Haipatikani")) : formatMinorUnits(estimatedTotal, bootstrap.context.currency, locale)}</dd></div><div><dt>{l(text("Final tax & total", "Kodi na jumla ya mwisho"))}</dt><dd>{l(text("Calculated by ERP", "Inahesabiwa na ERP"))}</dd></div></dl>
          <div className="server-control-copy"><ShieldCheck size={16} /><p>{l(text("No price or tax values are sent by this browser.", "Hakuna bei au kodi inayotumwa na kivinjari hiki."))}</p></div>
          {estimatedTotal === null && cart.length ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>unsafe_cart_total</strong>{l(text("The estimated total exceeds the browser's safe integer range. Reduce quantities before posting.", "Makadirio ya jumla yamezidi kiwango salama cha kivinjari. Punguza idadi kabla ya kuchapisha."))}</span></div> : null}
          {displayedProblem ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>{displayedProblem.code}</strong>{displayedProblem.detail}{displayedProblem.correlation_id ? <small>Correlation · {compactId(displayedProblem.correlation_id)}</small> : null}</span></div> : null}
          {!reviewing ? <button type="button" className="primary-button sale-submit" disabled={!canReview} onClick={() => setReviewing(true)}><Check size={17} />{l(text("Review controlled posting", "Kagua uchapishaji"))}</button> : <div className="posting-confirmation"><p><strong>{pendingRecovery ? l(text("Retry the preserved sale?", "Jaribu tena mauzo yaliyohifadhiwa?")) : l(text("Ready to post exactly once?", "Tayari kuchapisha mara moja?"))}</strong>{l(text("Stock, payment or receivable, tax, ledger, audit and outbox effects commit atomically.", "Bidhaa, malipo au deni, kodi, daftari, ukaguzi na matukio vinahifadhiwa kwa pamoja."))}</p><div><button type="button" className="secondary-button" disabled={submitting || completed} onClick={() => setReviewing(false)}><ChevronLeft size={15} />{l(text("Edit", "Hariri"))}</button><button type="button" className="primary-button" disabled={submitting || completed || !canReview} onClick={postSale}>{submitting ? l(text("Posting…", "Inachapisha…")) : pendingRecovery ? l(text("Retry same sale", "Jaribu tena mauzo yale yale")) : l(text("Confirm & post", "Thibitisha na chapisha"))}</button></div></div>}
        </aside>
      </div>
    </div>
  );
}
