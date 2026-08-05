"use client";

import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";
import {
  compactId,
  formatMinorUnits,
  formatTimestamp,
} from "@/live-api/format";
import type {
  CreateFinancialDocumentCommand,
  FinanceControlWorkspace,
  FinancialDocument,
  FiscalPeriodActionRequest,
  PublicProblem,
} from "@/live-api/types";

function problemFrom(value: unknown): PublicProblem {
  if (
    value &&
    typeof value === "object" &&
    "code" in value &&
    "detail" in value
  )
    return value as PublicProblem;
  if (value instanceof Error)
    return {
      type: "about:blank",
      title: "Request failed",
      status: 400,
      code: "finance_input_invalid",
      detail: value.message,
    };
  return {
    type: "about:blank",
    title: "Request failed",
    status: 500,
    code: "finance_command_failed",
    detail: "The finance command could not be completed.",
  };
}
async function payload(response: Response): Promise<unknown> {
  const value: unknown = await response.json();
  if (!response.ok) throw value;
  return value;
}
function parseJournalRows(value: string) {
  const lines = value
    .split(/\r?\n/)
    .map((row) => row.trim())
    .filter(Boolean)
    .map((row, index) => {
      const [account_id = "", debit = "0", credit = "0", memo = ""] = row
        .split("|")
        .map((part) => part.trim());
      const debit_minor = Number(debit),
        credit_minor = Number(credit);
      if (
        !account_id ||
        !Number.isSafeInteger(debit_minor) ||
        !Number.isSafeInteger(credit_minor) ||
        debit_minor > 0 === credit_minor > 0
      )
        throw new Error(`Invalid journal line ${index + 1}`);
      return { account_id, debit_minor, credit_minor, memo };
    });
  if (lines.length < 2)
    throw new Error("At least two journal lines are required");
  return lines;
}

export function FinanceControls({
  workspace,
}: {
  workspace: FinanceControlWorkspace;
}) {
  const { l, locale } = useLanguage();
  const [documents, setDocuments] = useState(workspace.documents);
  const [periods, setPeriods] = useState(workspace.periods);
  const [kind, setKind] =
    useState<CreateFinancialDocumentCommand["type"]>("CASH_TRANSFER");
  const [accountingDate, setAccountingDate] = useState("");
  const [reason, setReason] = useState("");
  const [fromAccount, setFromAccount] = useState(
    workspace.accounts[0]?.id ?? "",
  );
  const [toAccount, setToAccount] = useState(workspace.accounts[1]?.id ?? "");
  const [amount, setAmount] = useState(0);
  const [offsetAccount, setOffsetAccount] = useState("");
  const [journalRows, setJournalRows] = useState("");
  const [reversesId, setReversesId] = useState("");
  const [periodActions, setPeriodActions] = useState(workspace.periodActions);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const canManage =
    workspace.context.permissions.includes("finance.journals.manage") ||
    workspace.context.permissions.includes("finance.cash.transfer") ||
    workspace.context.permissions.includes("finance.bank.adjust");
  const canPost = workspace.context.permissions.includes(
    "finance.journals.post",
  );
  function replaceDocument(value: FinancialDocument) {
    setDocuments((current) =>
      current.map((item) => (item.id === value.id ? value : item)),
    );
  }
  async function create(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setProblem(null);
    try {
      if (!accountingDate || reason.trim().length < 8)
        throw new Error(
          "Accounting date and a reason of at least 8 characters are required",
        );
      const lines: CreateFinancialDocumentCommand["lines"] = [];
      const command: CreateFinancialDocumentCommand = {
        type: kind,
        currency: "TZS",
        accounting_at: `${accountingDate}T12:00:00+03:00`,
        reason,
        lines,
      };
      if (kind === "MANUAL_JOURNAL")
        command.lines = parseJournalRows(journalRows);
      if (kind === "CASH_TRANSFER") {
        if (amount <= 0 || fromAccount === toAccount)
          throw new Error(
            "Choose two different accounts and a positive amount",
          );
        command.from_account_id = fromAccount;
        command.to_account_id = toAccount;
        command.lines = [
          {
            account_id: "",
            debit_minor: amount,
            credit_minor: 0,
            memo: reason,
          },
        ];
      }
      if (kind === "BANK_ADJUSTMENT") {
        if (amount === 0 || !offsetAccount)
          throw new Error("Signed amount and offset GL account are required");
        command.from_account_id = fromAccount;
        command.lines = [
          {
            account_id: offsetAccount,
            debit_minor: amount > 0 ? amount : 0,
            credit_minor: amount < 0 ? -amount : 0,
            memo: reason,
          },
        ];
      }
      if (kind === "REVERSAL") {
        command.reverses_document_id = reversesId;
        command.lines = [];
      }
      const created = (await payload(
        await fetch("/api/live/finance/documents", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "Idempotency-Key": crypto.randomUUID(),
          },
          body: JSON.stringify(command),
        }),
      )) as FinancialDocument;
      setDocuments((current) => [created, ...current]);
      setReason("");
      setAmount(0);
      setJournalRows("");
    } catch (error) {
      setProblem(problemFrom(error));
    } finally {
      setBusy(false);
    }
  }
  async function transition(
    document: FinancialDocument,
    status: "SUBMITTED" | "POSTED" | "REJECTED",
  ) {
    setBusy(true);
    setProblem(null);
    try {
      const updated = (await payload(
        await fetch(`/api/live/finance/documents/${document.id}/transitions`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "Idempotency-Key": crypto.randomUUID(),
          },
          body: JSON.stringify({
            status,
            reason:
              status === "SUBMITTED"
                ? "Prepared financial evidence reviewed for approval"
                : status === "POSTED"
                  ? "Independent posting review completed successfully"
                  : "Independent review rejected the submitted evidence",
          }),
        }),
      )) as FinancialDocument;
      replaceDocument(updated);
    } catch (error) {
      setProblem(problemFrom(error));
    } finally {
      setBusy(false);
    }
  }
  async function requestPeriod(periodId: string, action: "CLOSE" | "REOPEN") {
    setBusy(true);
    setProblem(null);
    try {
      const result = (await payload(
        await fetch(`/api/live/finance/fiscal-periods/${periodId}/actions`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "Idempotency-Key": crypto.randomUUID(),
          },
          body: JSON.stringify({
            action,
            reason:
              action === "CLOSE"
                ? "All period reconciliations prepared for independent close"
                : "Approved correction requires controlled period reopening",
          }),
        }),
      )) as FiscalPeriodActionRequest;
      setPeriodActions((current) => [result, ...current]);
    } catch (error) {
      setProblem(problemFrom(error));
    } finally {
      setBusy(false);
    }
  }
  async function approvePeriod(actionId: string) {
    setBusy(true);
    setProblem(null);
    try {
      const result = (await payload(
        await fetch(
          `/api/live/finance/fiscal-period-actions/${actionId}/approval`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              "Idempotency-Key": crypto.randomUUID(),
            },
            body: JSON.stringify({
              reason:
                "Independent close evidence and blocking controls verified",
            }),
          },
        ),
      )) as FiscalPeriodActionRequest;
      setPeriodActions((current) =>
        current.map((item) => (item.id === result.id ? result : item)),
      );
      setPeriods((current) =>
        current.map((period) =>
          period.id === result.period_id
            ? { ...period, open: result.action === "REOPEN" }
            : period,
        ),
      );
    } catch (error) {
      setProblem(problemFrom(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="page-stack finance-controls">
      <header className="module-header">
        <div>
          <p className="eyebrow">
            {l(text("Governed finance", "Fedha zinazodhibitiwa"))}
          </p>
          <h1>
            {l(text("Journals & fiscal close", "Majarida na kufunga kipindi"))}
          </h1>
          <p>
            {l(
              text(
                "Prepare, independently post, reverse, and close with exact ledger evidence.",
                "Andaa, chapisha kwa uhuru, geuza, na funga kwa ushahidi sahihi wa leja.",
              ),
            )}
          </p>
        </div>
      </header>
      <section className="metric-grid">
        <article className="metric-card">
          <p className="metric-label">
            {l(text("Draft or submitted", "Rasimu au imewasilishwa"))}
          </p>
          <strong>
            {
              documents.filter(
                (d) => d.status === "DRAFT" || d.status === "SUBMITTED",
              ).length
            }
          </strong>
          <small>
            {l(
              text(
                "Must clear before close",
                "Lazima zikamilike kabla ya kufunga",
              ),
            )}
          </small>
        </article>
        <article className="metric-card">
          <p className="metric-label">{l(text("Posted", "Zimechapishwa"))}</p>
          <strong>
            {documents.filter((d) => d.status === "POSTED").length}
          </strong>
          <small>
            {l(
              text(
                "Balanced immutable journals",
                "Majarida yaliyosawazika yasiyobadilika",
              ),
            )}
          </small>
        </article>
        <article className="metric-card">
          <p className="metric-label">
            {l(text("Open periods", "Vipindi wazi"))}
          </p>
          <strong>{periods.filter((p) => p.open).length}</strong>
          <small>
            {l(
              text(
                "Company accounting calendar",
                "Kalenda ya uhasibu ya kampuni",
              ),
            )}
          </small>
        </article>
      </section>
      <section className="card operation-compose">
        <div className="card-heading">
          <div>
            <h2>
              {l(text("Prepare financial command", "Andaa agizo la kifedha"))}
            </h2>
            <p>
              {l(
                text(
                  "Cash and bank GL accounts are derived by the server.",
                  "Akaunti za leja za fedha na benki zinatolewa na seva.",
                ),
              )}
            </p>
          </div>
        </div>
        {canManage ? (
          <form className="bank-import-form" onSubmit={create}>
            <label>
              {l(text("Command", "Agizo"))}
              <select
                value={kind}
                onChange={(e) =>
                  setKind(
                    e.target.value as CreateFinancialDocumentCommand["type"],
                  )
                }
              >
                <option value="CASH_TRANSFER">Cash / bank transfer</option>
                <option value="BANK_ADJUSTMENT">Bank fee / suspense</option>
                <option value="MANUAL_JOURNAL">Manual journal</option>
                <option value="REVERSAL">Linked reversal</option>
              </select>
            </label>
            <label>
              {l(text("Accounting date", "Tarehe ya uhasibu"))}
              <input
                type="date"
                required
                value={accountingDate}
                onChange={(e) => setAccountingDate(e.target.value)}
              />
            </label>
            <label>
              {l(text("Reason", "Sababu"))}
              <input
                minLength={8}
                required
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </label>
            {kind === "CASH_TRANSFER" || kind === "BANK_ADJUSTMENT" ? (
              <label>
                {l(text("Cash / bank account", "Akaunti ya fedha / benki"))}
                <select
                  value={fromAccount}
                  onChange={(e) => setFromAccount(e.target.value)}
                >
                  {workspace.accounts.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.code} · {a.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            {kind === "CASH_TRANSFER" ? (
              <label>
                {l(text("Destination account", "Akaunti lengwa"))}
                <select
                  value={toAccount}
                  onChange={(e) => setToAccount(e.target.value)}
                >
                  {workspace.accounts.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.code} · {a.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            {kind === "CASH_TRANSFER" || kind === "BANK_ADJUSTMENT" ? (
              <label>
                {l(
                  text(
                    kind === "BANK_ADJUSTMENT"
                      ? "Signed amount (minor)"
                      : "Amount (minor)",
                    kind === "BANK_ADJUSTMENT"
                      ? "Kiasi chenye alama (senti)"
                      : "Kiasi (senti)",
                  ),
                )}
                <input
                  type="number"
                  value={amount}
                  onChange={(e) => setAmount(Number(e.target.value))}
                />
              </label>
            ) : null}
            {kind === "BANK_ADJUSTMENT" ? (
              <label>
                {l(text("Offset GL account", "Akaunti ya leja inayolingana"))}
                <input
                  required
                  value={offsetAccount}
                  onChange={(e) => setOffsetAccount(e.target.value)}
                  placeholder="bank-fees"
                />
              </label>
            ) : null}
            {kind === "MANUAL_JOURNAL" ? (
              <label className="bank-lines-input">
                {l(
                  text(
                    "Lines: account | debit | credit | memo",
                    "Mistari: akaunti | debit | credit | maelezo",
                  ),
                )}
                <textarea
                  rows={4}
                  required
                  value={journalRows}
                  onChange={(e) => setJournalRows(e.target.value)}
                />
              </label>
            ) : null}
            {kind === "REVERSAL" ? (
              <label>
                {l(
                  text(
                    "Posted document ID",
                    "Kitambulisho cha hati iliyochapishwa",
                  ),
                )}
                <input
                  required
                  value={reversesId}
                  onChange={(e) => setReversesId(e.target.value)}
                />
              </label>
            ) : null}
            <button className="primary-button" disabled={busy} type="submit">
              {l(text("Create draft", "Unda rasimu"))}
            </button>
          </form>
        ) : null}
        {problem ? (
          <div className="sale-problem" role="alert">
            <span>
              <strong>{problem.code}</strong>
              {problem.detail}
            </span>
          </div>
        ) : null}
      </section>
      <section className="card records-card">
        <div className="records-toolbar">
          <div>
            <h2>
              {l(
                text(
                  "Financial document register",
                  "Rejesta ya hati za kifedha",
                ),
              )}
            </h2>
            <span>{documents.length}</span>
          </div>
        </div>
        <div className="statement-list">
          {documents.map((d) => (
            <article key={d.id} className="finance-document-row">
              <span>
                <strong>
                  {d.number} · {d.type.replaceAll("_", " ")}
                </strong>
                <small>
                  {formatTimestamp(
                    d.accounting_at,
                    locale,
                    workspace.context.timezone,
                  )}{" "}
                  · {compactId(d.id)}
                </small>
              </span>
              <span
                className={`reconciliation-status ${d.status === "POSTED" ? "status-resolved" : "status-open"}`}
              >
                {d.status}
              </span>
              <b>
                {formatMinorUnits(
                  d.lines.reduce((sum, line) => sum + line.debit_minor, 0),
                  d.currency,
                  locale,
                )}
              </b>
              <div>
                {d.status === "DRAFT" ? (
                  <button
                    className="secondary-button"
                    disabled={busy}
                    onClick={() => transition(d, "SUBMITTED")}
                  >
                    {l(text("Submit", "Wasilisha"))}
                  </button>
                ) : null}
                {d.status === "SUBMITTED" && canPost ? (
                  <>
                    <button
                      className="primary-button"
                      disabled={
                        busy || d.created_by === workspace.context.actor_id
                      }
                      onClick={() => transition(d, "POSTED")}
                    >
                      {l(text("Post", "Chapisha"))}
                    </button>
                    <button
                      className="secondary-button"
                      disabled={
                        busy || d.created_by === workspace.context.actor_id
                      }
                      onClick={() => transition(d, "REJECTED")}
                    >
                      {l(text("Reject", "Kataa"))}
                    </button>
                  </>
                ) : null}
              </div>
            </article>
          ))}
        </div>
      </section>
      <section className="card">
        <div className="card-heading">
          <div>
            <h2>
              {l(
                text("Fiscal-period control", "Udhibiti wa kipindi cha fedha"),
              )}
            </h2>
            <p>
              {l(
                text(
                  "Close is blocked by unreconciled statements or unfinished documents. A different user approves.",
                  "Kufunga kunazuiwa na taarifa zisizopatanishwa au hati ambazo hazijakamilika. Mtumiaji mwingine anaidhinisha.",
                ),
              )}
            </p>
          </div>
        </div>
        <div className="statement-list">
          {periods.map((p) => (
            <article key={p.id} className="finance-document-row">
              <span>
                <strong>
                  {new Date(p.starts_at).toLocaleDateString(locale)} –{" "}
                  {new Date(p.ends_at).toLocaleDateString(locale)}
                </strong>
                <small>{compactId(p.id)}</small>
              </span>
              <span
                className={`reconciliation-status ${p.open ? "status-open" : "status-resolved"}`}
              >
                {p.open ? "OPEN" : "CLOSED"}
              </span>
              <button
                className="secondary-button"
                disabled={busy}
                onClick={() => requestPeriod(p.id, p.open ? "CLOSE" : "REOPEN")}
              >
                {p.open
                  ? l(text("Request close", "Omba kufunga"))
                  : l(text("Request reopen", "Omba kufungua"))}
              </button>
            </article>
          ))}
        </div>
        <div className="card-heading">
          <div>
            <h3>
              {l(
                text(
                  "Close & reopen requests",
                  "Maombi ya kufunga na kufungua",
                ),
              )}
            </h3>
            <p>
              {l(
                text(
                  "Pending requests are visible to every authorized checker.",
                  "Maombi yanayosubiri yanaonekana kwa kila mkaguzi mwenye ruhusa.",
                ),
              )}
            </p>
          </div>
        </div>
        <div className="statement-list">
          {periodActions.map((action) => (
            <article key={action.id} className="finance-document-row">
              <span>
                <strong>
                  {action.action} · {compactId(action.period_id)}
                </strong>
                <small>
                  {formatTimestamp(
                    action.requested_at,
                    locale,
                    workspace.context.timezone,
                  )}{" "}
                  · {action.reason}
                </small>
              </span>
              <span
                className={`reconciliation-status ${action.approved_at ? "status-resolved" : "status-open"}`}
              >
                {action.approved_at ? "APPROVED" : "PENDING"}
              </span>
              {!action.approved_at &&
              action.requested_by !== workspace.context.actor_id ? (
                <button
                  className="primary-button"
                  disabled={busy}
                  onClick={() => approvePeriod(action.id)}
                >
                  {l(text("Approve action", "Idhinisha hatua"))}
                </button>
              ) : (
                <small>
                  {action.requested_by === workspace.context.actor_id &&
                  !action.approved_at
                    ? l(
                        text(
                          "Awaiting another checker",
                          "Inasubiri mkaguzi mwingine",
                        ),
                      )
                    : ""}
                </small>
              )}
            </article>
          ))}
        </div>
      </section>
    </section>
  );
}
