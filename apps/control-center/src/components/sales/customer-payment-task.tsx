"use client";

import Link from "next/link";
import {
  ArrowLeft,
  ArrowRight,
  Banknote,
  Check,
  CheckCircle2,
  CreditCard,
  FileText,
  HandCoins,
  Landmark,
  Loader2,
  ShieldCheck,
  Smartphone,
  TriangleAlert,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { loadCustomerPaymentAccount } from "@/app/sales/payments/actions";
import { useLanguage } from "@/components/language-provider";
import { SalesModuleNav } from "@/components/sales/sales-module-nav";
import { formatMinorUnits, formatTimestamp } from "@/live-api/format";
import {
  clearPendingCommandAfterSuccess,
  markPendingCommandRejected,
  PendingCommandConflictError,
  PendingCommandStorageError,
  reservePendingCommand,
  type PendingCommandSnapshot,
} from "@/live-api/pending-command";
import type {
  CustomerAccountDetail,
  CustomerCollection,
  PaymentMethod,
  PublicProblem,
  ReceiveCustomerCollectionCommand,
  SalesRegisterWorkspace,
} from "@/live-api/types";
import { usePendingCommand } from "@/live-api/use-pending-command";
import { text } from "@/lib/i18n";
import { saleBusinessReference } from "@/lib/sales-presentation";
import styles from "./customer-payment.module.css";

type PaymentWorkspace = SalesRegisterWorkspace;
type ReceivableItem = CustomerAccountDetail["open_items"][number];
type Step = 1 | 2 | 3;

interface CustomerPaymentPayload {
  customer_id: string;
  command: ReceiveCustomerCollectionCommand;
}

const PAYMENT_METHODS: Array<{
  value: PaymentMethod;
  label: ReturnType<typeof text>;
  detail: ReturnType<typeof text>;
  icon: LucideIcon;
}> = [
  { value: "MOBILE_MONEY", label: text("Mobile money", "Pesa kwa simu"), detail: text("M-Pesa, Airtel Money or another mobile wallet", "M-Pesa, Airtel Money au pochi nyingine ya simu"), icon: Smartphone },
  { value: "BANK_TRANSFER", label: text("Bank transfer", "Hamisho la benki"), detail: text("Payment received in the company bank account", "Malipo yamepokelewa katika akaunti ya benki ya kampuni"), icon: Landmark },
  { value: "CASH", label: text("Cash", "Fedha taslimu"), detail: text("Physical cash received and counted", "Fedha taslimu zimepokelewa na kuhesabiwa"), icon: Banknote },
  { value: "BANK_CARD", label: text("Bank card", "Kadi ya benki"), detail: text("Debit or credit card payment", "Malipo ya kadi ya benki"), icon: CreditCard },
];

function customerPaymentPendingScope(context: PaymentWorkspace["context"]): string {
  return [
    "customers:collection",
    `actor=${encodeURIComponent(context.actor_id)}`,
    `tenant=${encodeURIComponent(context.tenant_id)}`,
    `company=${encodeURIComponent(context.company_id)}`,
    `branch=${encodeURIComponent(context.branch_id)}`,
    `warehouse=${encodeURIComponent(context.warehouse_id)}`,
  ].join(":");
}

export function parseTzsMajorUnits(value: string): number | null {
  const normalized = value.trim();
  const match = /^(0|[1-9][0-9]*)(?:\.([0-9]{1,2}))?$/.exec(normalized);
  if (!match) return null;
  const whole = BigInt(match[1]);
  const fraction = BigInt((match[2] ?? "").padEnd(2, "0") || "0");
  const minor = whole * BigInt(100) + fraction;
  if (minor <= BigInt(0) || minor > BigInt(Number.MAX_SAFE_INTEGER)) return null;
  return Number(minor);
}

function minorToMajorInput(value: number): string {
  const minor = BigInt(value);
  const whole = minor / BigInt(100);
  const fraction = (minor % BigInt(100)).toString().padStart(2, "0");
  return `${whole}.${fraction}`;
}

function isPublicProblem(value: unknown): value is PublicProblem {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.code === "string" && typeof candidate.detail === "string";
}

function isCustomerCollection(value: unknown): value is CustomerCollection {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.id === "string"
    && typeof candidate.customer_id === "string"
    && typeof candidate.invoice_sale_id === "string"
    && typeof candidate.occurred_at === "string"
    && typeof candidate.amount_minor === "number"
    && Number.isSafeInteger(candidate.amount_minor)
    && candidate.amount_minor > 0;
}

function isCustomerPaymentPayload(value: unknown): value is CustomerPaymentPayload {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Record<string, unknown>;
  if (typeof candidate.customer_id !== "string" || !candidate.command || typeof candidate.command !== "object") return false;
  const command = candidate.command as Record<string, unknown>;
  return typeof command.invoice_sale_id === "string"
    && PAYMENT_METHODS.some(({ value: method }) => method === command.method)
    && typeof command.amount_minor === "number"
    && Number.isSafeInteger(command.amount_minor)
    && command.amount_minor > 0
    && command.currency === "TZS";
}

function formatDate(value: string, locale: "en" | "sw", timeZone: string): string {
  return new Intl.DateTimeFormat(locale === "sw" ? "sw-TZ" : "en-TZ", {
    dateStyle: "medium",
    timeZone,
  }).format(new Date(value));
}

export function CustomerPaymentTask({ workspace }: { workspace: PaymentWorkspace }) {
  const { locale, l } = useLanguage();
  const [step, setStep] = useState<Step>(1);
  const [customerId, setCustomerId] = useState("");
  const [account, setAccount] = useState<CustomerAccountDetail | null>(null);
  const [loadingAccount, setLoadingAccount] = useState(false);
  const [invoiceSaleId, setInvoiceSaleId] = useState("");
  const [amount, setAmount] = useState("");
  const [amountTouched, setAmountTouched] = useState(false);
  const [method, setMethod] = useState<PaymentMethod | "">("");
  const [submitting, setSubmitting] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const [collection, setCollection] = useState<CustomerCollection | null>(null);
  const accountRequest = useRef(0);
  const pendingScope = customerPaymentPendingScope(workspace.context);
  const pendingStore = usePendingCommand<unknown>(pendingScope);

  const eligibleCustomers = useMemo(
    () => workspace.customers
      .filter((customer) => customer.status === "active" && !customer.is_general_customer)
      .sort((left, right) => left.name.localeCompare(right.name)),
    [workspace.customers],
  );
  const invoices = useMemo(
    () => (account?.open_items ?? [])
      .filter((item) => item.kind === "INVOICE" && item.outstanding_minor > 0 && item.currency === workspace.context.currency)
      .sort((left, right) => Date.parse(left.due_at ?? left.document_at) - Date.parse(right.due_at ?? right.document_at)),
    [account, workspace.context.currency],
  );
  const salesById = useMemo(() => new Map(workspace.sales.map((sale) => [sale.id, sale])), [workspace.sales]);
  const selectedCustomer = eligibleCustomers.find((customer) => customer.id === customerId);
  const selectedInvoice = invoices.find((invoice) => invoice.source_id === invoiceSaleId);
  const amountMinor = parseTzsMajorUnits(amount);
  const amountInvalid = amountMinor === null || !selectedInvoice || amountMinor > selectedInvoice.outstanding_minor;
  const selectedMethod = PAYMENT_METHODS.find((item) => item.value === method);
  const pendingRecovery = pendingStore.snapshot && isCustomerPaymentPayload(pendingStore.snapshot.record.payload)
    ? pendingStore.snapshot as PendingCommandSnapshot<CustomerPaymentPayload>
    : null;
  const recoveryProblem: PublicProblem | null = pendingStore.error
    ? { type: "about:blank", title: "Recovery unavailable", status: 503, code: "pending_payment_storage_unavailable", detail: pendingStore.error.message }
    : pendingStore.snapshot && !pendingRecovery
      ? { type: "about:blank", title: "Recovery blocked", status: 409, code: "pending_payment_invalid", detail: "A preserved payment cannot be reconstructed safely. Reconcile the customer account before posting another payment in this tab." }
      : null;
  const displayedProblem = problem ?? recoveryProblem;
  const canPost = Boolean(selectedCustomer && selectedInvoice && amountMinor && method && !recoveryProblem && !collection);

  function invoiceReference(item: ReceivableItem): string {
    const sale = salesById.get(item.source_id);
    return sale ? saleBusinessReference(sale) : l(text("Credit invoice", "Ankara ya mkopo"));
  }

  async function chooseCustomer(nextCustomerId: string, restore?: CustomerPaymentPayload) {
    const requestId = accountRequest.current + 1;
    accountRequest.current = requestId;
    setCustomerId(nextCustomerId);
    setAccount(null);
    setInvoiceSaleId("");
    setAmount("");
    setAmountTouched(false);
    setMethod("");
    setStep(1);
    setProblem(null);
    setCollection(null);
    setLoadingAccount(Boolean(nextCustomerId));
    if (!nextCustomerId) return;
    if (!eligibleCustomers.some((customer) => customer.id === nextCustomerId)) {
      setLoadingAccount(false);
      setProblem({ type: "about:blank", title: "Customer unavailable", status: 409, code: "customer_payment_customer_changed", detail: "This named customer is no longer available in the current company scope." });
      return;
    }

    try {
      const result = await loadCustomerPaymentAccount(nextCustomerId);
      if (requestId !== accountRequest.current) return;
      if (!result.ok) {
        setProblem(result.problem);
        return;
      }
      setAccount(result.account);
      if (restore) {
        const restorableInvoice = result.account.open_items.some((item) => item.kind === "INVOICE"
          && item.source_id === restore.command.invoice_sale_id
          && item.currency === workspace.context.currency
          && item.outstanding_minor >= restore.command.amount_minor);
        if (!restorableInvoice) {
          setProblem({ type: "about:blank", title: "Payment needs reconciliation", status: 409, code: "pending_payment_invoice_changed", detail: "The preserved invoice is no longer open for the same amount. Check the customer account before retrying." });
          return;
        }
        setInvoiceSaleId(restore.command.invoice_sale_id);
        setAmount(minorToMajorInput(restore.command.amount_minor));
        setMethod(restore.command.method);
        setStep(3);
      }
    } catch {
      if (requestId === accountRequest.current) {
        setProblem({ type: "about:blank", title: "Customer account unavailable", status: 503, code: "customer_payment_account_unavailable", detail: "The customer account could not be loaded. Check the connection and try again." });
      }
    } finally {
      if (requestId === accountRequest.current) setLoadingAccount(false);
    }
  }

  function continueToPayment() {
    if (!selectedInvoice) return;
    setProblem(null);
    setStep(2);
  }

  function continueToReview() {
    setAmountTouched(true);
    if (amountInvalid || !method) return;
    setProblem(null);
    setStep(3);
  }

  async function submitPayment(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canPost || amountMinor === null || !selectedInvoice || !method) return;

    const command: ReceiveCustomerCollectionCommand = {
      invoice_sale_id: selectedInvoice.source_id,
      method,
      amount_minor: amountMinor,
      currency: workspace.context.currency,
    };
    const payload: CustomerPaymentPayload = { customer_id: customerId, command };
    const fingerprint = JSON.stringify(payload);
    let pending: PendingCommandSnapshot<CustomerPaymentPayload>;
    try {
      pending = reservePendingCommand({ storage: window.sessionStorage, scope: pendingScope, fingerprint, payload });
    } catch (error) {
      const detail = error instanceof PendingCommandConflictError
        ? "Another payment is awaiting an authoritative result. Restore that exact payment before starting a different one."
        : error instanceof PendingCommandStorageError
          ? error.message
          : "The payment identity could not be preserved, so nothing was sent.";
      setProblem({ type: "about:blank", title: "Payment not sent", status: 409, code: "pending_payment_conflict", detail });
      return;
    }

    setSubmitting(true);
    setProblem(null);
    try {
      const response = await fetch(`/api/live/customers/${encodeURIComponent(customerId)}/collections`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": pending.record.key,
          Accept: "application/json",
        },
        body: JSON.stringify(command),
      });
      const responsePayload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) {
          try {
            markPendingCommandRejected<CustomerPaymentPayload>(window.sessionStorage, pendingScope, pending.record.key, response.status);
          } catch {
            // The original command identity remains the safest retry key.
          }
        }
        setProblem(isPublicProblem(responsePayload)
          ? responsePayload
          : { type: "about:blank", title: "Payment rejected", status: response.status, code: "customer_payment_rejected", detail: "The live ERP rejected this customer payment." });
        return;
      }
      if (!isCustomerCollection(responsePayload)) throw new Error("The successful response was incomplete.");
      try {
        clearPendingCommandAfterSuccess(window.sessionStorage, pendingScope, pending.record.key);
      } catch {
        // The backend retains the committed idempotency result if the marker cannot be cleared.
      }
      setCollection(responsePayload);
    } catch {
      setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "customer_payment_connection_interrupted", detail: "We safely preserved this payment attempt. Retry to confirm or complete the same payment without posting it twice." });
    } finally {
      setSubmitting(false);
    }
  }

  function resetTask() {
    accountRequest.current += 1;
    setStep(1);
    setCustomerId("");
    setAccount(null);
    setInvoiceSaleId("");
    setAmount("");
    setAmountTouched(false);
    setMethod("");
    setProblem(null);
    setCollection(null);
  }

  if (!workspace.context.permissions.includes("customers.collections.post")) {
    return (
      <div className={styles.page}>
        <SalesModuleNav permissions={workspace.context.permissions} />
        <section className={styles.blocked}>
          <ShieldCheck size={28} aria-hidden="true" />
          <h1>{l(text("Payment access required", "Ruhusa ya malipo inahitajika"))}</h1>
          <p>{l(text("Your current role cannot post customer payments.", "Jukumu lako la sasa haliwezi kuchapisha malipo ya wateja."))}</p>
          <Link href="/sales">{l(text("Back to Sales", "Rudi kwenye Mauzo"))}</Link>
        </section>
      </div>
    );
  }

  if (collection && selectedCustomer && selectedInvoice && selectedMethod) {
    return (
      <div className={styles.page}>
        <SalesModuleNav permissions={workspace.context.permissions} />
        <section className={styles.success} role="status">
          <span className={styles.successIcon}><CheckCircle2 size={34} aria-hidden="true" /></span>
          <p className={styles.eyebrow}>{workspace.context.branch_name}</p>
          <h1>{l(text("Payment posted", "Malipo yamechapishwa"))}</h1>
          <p>{l(text("The payment is allocated to the selected invoice and posted to the customer account.", "Malipo yametengwa kwa ankara iliyochaguliwa na kuchapishwa kwenye akaunti ya mteja."))}</p>
          <dl className={styles.successSummary}>
            <div><dt>{l(text("Customer", "Mteja"))}</dt><dd>{selectedCustomer.name}</dd></div>
            <div><dt>{l(text("Invoice", "Ankara"))}</dt><dd>{invoiceReference(selectedInvoice)}</dd></div>
            <div><dt>{l(text("Amount", "Kiasi"))}</dt><dd>{formatMinorUnits(collection.amount_minor, collection.currency, locale)}</dd></div>
            <div><dt>{l(text("Payment method", "Njia ya malipo"))}</dt><dd>{l(selectedMethod.label)}</dd></div>
            <div><dt>{l(text("Posted", "Yamechapishwa"))}</dt><dd>{formatTimestamp(collection.occurred_at, locale, workspace.context.timezone)}</dd></div>
          </dl>
          <div className={styles.successActions}>
            <button type="button" className={styles.primaryButton} onClick={resetTask}>{l(text("Record another payment", "Rekodi malipo mengine"))}</button>
            <Link className={styles.secondaryButton} href={`/customers/${selectedCustomer.id}`}>{l(text("View customer account", "Angalia akaunti ya mteja"))}</Link>
          </div>
        </section>
      </div>
    );
  }

  return (
    <div className={styles.page}>
      <SalesModuleNav permissions={workspace.context.permissions} />

      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>{workspace.context.company_name} · {workspace.context.branch_name}</p>
          <h1>{l(text("Record customer payment", "Rekodi malipo ya mteja"))}</h1>
          <p>{l(text("Choose an open invoice, enter the amount received, then review once before posting.", "Chagua ankara iliyo wazi, weka kiasi kilichopokelewa, kisha kagua mara moja kabla ya kuchapisha."))}</p>
        </div>
        <Link className={styles.backLink} href="/sales"><ArrowLeft size={17} aria-hidden="true" />{l(text("Back to Sales", "Rudi kwenye Mauzo"))}</Link>
      </header>

      <ol className={styles.progress} aria-label={l(text("Payment progress", "Hatua za malipo"))}>
        {[
          text("Invoice", "Ankara"),
          text("Payment", "Malipo"),
          text("Review", "Kagua"),
        ].map((label, index) => {
          const itemStep = (index + 1) as Step;
          const completed = itemStep < step;
          return (
            <li key={label.en} className={itemStep === step ? styles.currentStep : completed ? styles.completedStep : undefined} aria-current={itemStep === step ? "step" : undefined}>
              <span>{completed ? <Check size={16} aria-hidden="true" /> : itemStep}</span>
              <strong>{l(label)}</strong>
            </li>
          );
        })}
      </ol>

      {pendingRecovery ? (
        <aside className={styles.recovery} role="status">
          <ShieldCheck size={21} aria-hidden="true" />
          <div>
            <strong>{l(text("Unconfirmed payment preserved", "Malipo ambayo hayajathibitishwa yamehifadhiwa"))}</strong>
            <p>{l(text("Restore the exact customer, invoice, amount and payment method to retry with the same protected identity.", "Rejesha mteja, ankara, kiasi na njia ileile ya malipo ili ujaribu tena kwa utambulisho uleule salama."))}</p>
          </div>
          <button type="button" className={styles.secondaryButton} onClick={() => void chooseCustomer(pendingRecovery.record.payload.customer_id, pendingRecovery.record.payload)} disabled={loadingAccount}>
            {loadingAccount ? l(text("Restoring…", "Inarejesha…")) : l(text("Restore payment", "Rejesha malipo"))}
          </button>
        </aside>
      ) : null}

      {displayedProblem ? (
        <div className={styles.problem} role="alert">
          <TriangleAlert size={20} aria-hidden="true" />
          <div><strong>{l(text("Payment not completed", "Malipo hayajakamilika"))}</strong><p>{displayedProblem.detail}</p></div>
        </div>
      ) : null}

      <form className={styles.taskShell} onSubmit={submitPayment}>
        <div className={styles.taskCard}>
          {step === 1 ? (
            <section aria-labelledby="payment-invoice-step">
              <div className={styles.sectionHeading}>
                <span><FileText size={21} aria-hidden="true" /></span>
                <div><h2 id="payment-invoice-step">{l(text("Choose the invoice", "Chagua ankara"))}</h2><p>{l(text("Only named customer accounts with an outstanding credit invoice are available.", "Akaunti za wateja wenye majina na ankara ya mkopo yenye salio pekee ndizo zinapatikana."))}</p></div>
              </div>

              <label className={styles.field}>
                <span>{l(text("Customer account", "Akaunti ya mteja"))}</span>
                <select value={customerId} onChange={(event) => void chooseCustomer(event.target.value)} disabled={loadingAccount}>
                  <option value="">{l(text("Select a named customer", "Chagua mteja mwenye jina"))}</option>
                  {eligibleCustomers.map((customer) => <option key={customer.id} value={customer.id}>{customer.code} · {customer.name}</option>)}
                </select>
              </label>

              {loadingAccount ? (
                <div className={styles.loading} role="status"><span className={styles.spin}><Loader2 size={22} aria-hidden="true" /></span><span>{l(text("Loading open invoices…", "Inapakia ankara zilizo wazi…"))}</span></div>
              ) : account && invoices.length ? (
                <fieldset className={styles.invoiceList}>
                  <legend>{l(text("Open invoices", "Ankara zilizo wazi"))}</legend>
                  {invoices.map((invoice) => {
                    const checked = invoice.source_id === invoiceSaleId;
                    return (
                      <label key={invoice.id} className={checked ? styles.choiceSelected : undefined}>
                        <input type="radio" name="invoice" value={invoice.source_id} checked={checked} onChange={() => { setInvoiceSaleId(invoice.source_id); setProblem(null); }} />
                        <span className={styles.choiceMarker}><Check size={15} aria-hidden="true" /></span>
                        <span className={styles.invoiceIdentity}>
                          <strong>{invoiceReference(invoice)}</strong>
                          <small>{l(text("Issued", "Imetolewa"))} {formatDate(invoice.document_at, locale, workspace.context.timezone)}{invoice.due_at ? ` · ${l(text("Due", "Mwisho"))} ${formatDate(invoice.due_at, locale, workspace.context.timezone)}` : ""}</small>
                        </span>
                        <span className={styles.invoiceAmount}><small>{l(text("Outstanding", "Salio"))}</small><strong>{formatMinorUnits(invoice.outstanding_minor, invoice.currency, locale)}</strong></span>
                      </label>
                    );
                  })}
                </fieldset>
              ) : account ? (
                <div className={styles.emptyState}><CheckCircle2 size={26} aria-hidden="true" /><strong>{l(text("No open invoices", "Hakuna ankara zilizo wazi"))}</strong><p>{l(text("This customer has no outstanding invoice available for payment.", "Mteja huyu hana ankara yenye salio inayopatikana kwa malipo."))}</p></div>
              ) : null}

              <div className={styles.stepActions}>
                <span />
                <button type="button" className={styles.primaryButton} onClick={continueToPayment} disabled={!selectedInvoice}>{l(text("Continue to payment", "Endelea kwenye malipo"))}<ArrowRight size={17} aria-hidden="true" /></button>
              </div>
            </section>
          ) : null}

          {step === 2 && selectedInvoice ? (
            <section aria-labelledby="payment-details-step">
              <div className={styles.sectionHeading}>
                <span><HandCoins size={21} aria-hidden="true" /></span>
                <div><h2 id="payment-details-step">{l(text("Enter payment details", "Weka maelezo ya malipo"))}</h2><p>{l(text("Record the amount actually received and how it was paid.", "Rekodi kiasi kilichopokelewa na jinsi kilivyolipwa."))}</p></div>
              </div>

              <div className={styles.amountPanel}>
                <label className={styles.field}>
                  <span>{l(text("Amount received", "Kiasi kilichopokelewa"))}</span>
                  <span className={styles.moneyInput}><b>TZS</b><input type="text" inputMode="decimal" autoComplete="off" value={amount} onChange={(event) => { setAmount(event.target.value); setProblem(null); }} onBlur={() => setAmountTouched(true)} placeholder="0.00" aria-describedby={amountTouched && amountInvalid ? "payment-amount-help payment-amount-error" : "payment-amount-help"} /></span>
                  <small id="payment-amount-help">{l(text("Maximum", "Kiwango cha juu"))}: {formatMinorUnits(selectedInvoice.outstanding_minor, selectedInvoice.currency, locale)}</small>
                  {amountTouched && amountInvalid ? <small id="payment-amount-error" className={styles.fieldError}>{amountMinor && amountMinor > selectedInvoice.outstanding_minor ? l(text("The amount cannot exceed the invoice balance.", "Kiasi hakiwezi kuzidi salio la ankara.")) : l(text("Enter a valid amount greater than zero, with up to two decimal places.", "Weka kiasi halali zaidi ya sifuri, chenye hadi nafasi mbili za desimali."))}</small> : null}
                </label>
              </div>

              <fieldset className={styles.methodList}>
                <legend>{l(text("Payment method", "Njia ya malipo"))}</legend>
                {PAYMENT_METHODS.map((option) => {
                  const Icon = option.icon;
                  const checked = option.value === method;
                  return (
                    <label key={option.value} className={checked ? styles.choiceSelected : undefined}>
                      <input type="radio" name="payment-method" value={option.value} checked={checked} onChange={() => { setMethod(option.value); setProblem(null); }} />
                      <span className={styles.methodIcon}><Icon size={20} aria-hidden="true" /></span>
                      <span><strong>{l(option.label)}</strong><small>{l(option.detail)}</small></span>
                      <span className={styles.choiceMarker}><Check size={15} aria-hidden="true" /></span>
                    </label>
                  );
                })}
              </fieldset>

              <div className={styles.stepActions}>
                <button type="button" className={styles.secondaryButton} onClick={() => setStep(1)}><ArrowLeft size={17} aria-hidden="true" />{l(text("Back", "Rudi"))}</button>
                <button type="button" className={styles.primaryButton} onClick={continueToReview} disabled={amountInvalid || !method}>{l(text("Review payment", "Kagua malipo"))}<ArrowRight size={17} aria-hidden="true" /></button>
              </div>
            </section>
          ) : null}

          {step === 3 && selectedCustomer && selectedInvoice && selectedMethod && amountMinor ? (
            <section aria-labelledby="payment-review-step">
              <div className={styles.sectionHeading}>
                <span><ShieldCheck size={21} aria-hidden="true" /></span>
                <div><h2 id="payment-review-step">{l(text("Review before posting", "Kagua kabla ya kuchapisha"))}</h2><p>{l(text("Confirm the customer, invoice and amount. Posting updates the authoritative receivables ledger.", "Thibitisha mteja, ankara na kiasi. Uchapishaji unasasisha leja rasmi ya madeni yanayopokelewa."))}</p></div>
              </div>

              <dl className={styles.reviewList}>
                <div><dt>{l(text("Customer", "Mteja"))}</dt><dd><strong>{selectedCustomer.name}</strong><small>{selectedCustomer.code}</small></dd></div>
                <div><dt>{l(text("Invoice", "Ankara"))}</dt><dd><strong>{invoiceReference(selectedInvoice)}</strong><small>{formatDate(selectedInvoice.document_at, locale, workspace.context.timezone)}</small></dd></div>
                <div><dt>{l(text("Payment method", "Njia ya malipo"))}</dt><dd><strong>{l(selectedMethod.label)}</strong></dd></div>
                <div className={styles.reviewAmount}><dt>{l(text("Amount to post", "Kiasi cha kuchapisha"))}</dt><dd>{formatMinorUnits(amountMinor, workspace.context.currency, locale)}</dd></div>
                <div><dt>{l(text("Invoice balance after payment", "Salio la ankara baada ya malipo"))}</dt><dd><strong>{formatMinorUnits(selectedInvoice.outstanding_minor - amountMinor, selectedInvoice.currency, locale)}</strong></dd></div>
              </dl>

              <div className={styles.controlNote}><ShieldCheck size={18} aria-hidden="true" /><p>{l(text("The ERP will allocate this payment to one invoice and update receivables as one protected posting. No balance is edited directly.", "ERP itatenga malipo haya kwa ankara moja na kusasisha madeni yanayopokelewa katika uchapishaji mmoja salama. Hakuna salio litakalohaririwa moja kwa moja."))}</p></div>

              <div className={styles.stepActions}>
                <button type="button" className={styles.secondaryButton} onClick={() => setStep(2)} disabled={submitting}><ArrowLeft size={17} aria-hidden="true" />{l(text("Edit payment", "Hariri malipo"))}</button>
                <button type="submit" className={styles.primaryButton} disabled={submitting || !canPost}>{submitting ? <span className={styles.spin}><Loader2 size={17} aria-hidden="true" /></span> : <HandCoins size={17} aria-hidden="true" />}{submitting ? l(text("Posting payment…", "Inachapisha malipo…")) : pendingRecovery ? l(text("Retry same payment", "Jaribu malipo yale yale")) : l(text("Post payment", "Chapisha malipo"))}</button>
              </div>
            </section>
          ) : null}
        </div>

        <aside className={styles.contextCard}>
          <span className={styles.contextIcon}><ShieldCheck size={20} aria-hidden="true" /></span>
          <h2>{l(text("Posting context", "Muktadha wa uchapishaji"))}</h2>
          <dl>
            <div><dt>{l(text("Company", "Kampuni"))}</dt><dd>{workspace.context.company_name}</dd></div>
            <div><dt>{l(text("Branch", "Tawi"))}</dt><dd>{workspace.context.branch_name}</dd></div>
            <div><dt>{l(text("Currency", "Sarafu"))}</dt><dd>{workspace.context.currency}</dd></div>
          </dl>
          <p>{l(text("Customer, scope, invoice balance and posting permissions are revalidated by the live ERP.", "Mteja, upeo, salio la ankara na ruhusa za uchapishaji zinathibitishwa tena na ERP hai."))}</p>
        </aside>
      </form>
    </div>
  );
}
