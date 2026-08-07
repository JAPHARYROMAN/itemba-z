"use client";

import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import { compactId, formatMinorUnits, formatTimestamp } from "@/live-api/format";
import type { BankStatement, BankingWorkspace, ImportBankStatementCommand, PublicProblem } from "@/live-api/types";

function problemFrom(value: unknown): PublicProblem {
  if (value && typeof value === "object" && "code" in value && "detail" in value) return value as PublicProblem;
  if (value instanceof Error) return { type: "about:blank", title: "Request failed", status: 400, code: "statement_input_invalid", detail: value.message };
  return { type: "about:blank", title: "Request failed", status: 500, code: "banking_failed", detail: "The banking command could not be completed." };
}

function parseLines(value: string, periodStart: string): ImportBankStatementCommand["lines"] {
  return value.split(/\r?\n/).map((row) => row.trim()).filter(Boolean).map((row, index) => {
    const [date = periodStart, external_reference = "", description = "", amount = ""] = row.split("|").map((part) => part.trim());
    const amount_minor = Number(amount);
    if (!date || description.length < 3 || !Number.isSafeInteger(amount_minor) || amount_minor === 0) throw new Error(`Invalid statement line ${index + 1}`);
    return { transaction_at: `${date}T12:00:00+03:00`, external_reference, description, amount_minor };
  });
}

export function BankingWorkbench({ workspace }: { workspace: BankingWorkspace }) {
  const { l, locale } = useLanguage();
  const [statements, setStatements] = useState(workspace.statements);
  const [selected, setSelected] = useState<BankStatement | null>(null);
  const [accountId, setAccountId] = useState(workspace.accounts.find((account) => account.active)?.id ?? "");
  const [externalReference, setExternalReference] = useState("");
  const [periodStart, setPeriodStart] = useState("");
  const [periodEnd, setPeriodEnd] = useState("");
  const [openingMinor, setOpeningMinor] = useState(0);
  const [closingMinor, setClosingMinor] = useState(0);
  const [rows, setRows] = useState("");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const canImport = workspace.context.permissions.includes("finance.bank.import");
  const canMatch = workspace.context.permissions.includes("finance.bank.match");
  const canReconcile = workspace.context.permissions.includes("finance.bank.reconcile");

  async function readPayload(response: Response): Promise<unknown> { const payload: unknown = await response.json(); if (!response.ok) throw payload; return payload; }
  async function loadDetail(statementId: string) { setBusy(true); setProblem(null); try { const payload = await readPayload(await fetch(`/api/live/banking/statements/${statementId}`, { cache: "no-store" })); setSelected(payload as BankStatement); } catch (error) { setProblem(problemFrom(error)); } finally { setBusy(false); } }
  function replaceStatement(statement: BankStatement) { setSelected(statement); setStatements((current) => current.map((item) => item.id === statement.id ? statement : item)); }

  async function importStatement(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setProblem(null);
    try {
      const account = workspace.accounts.find((item) => item.id === accountId);
      if (!account || !periodStart || !periodEnd) throw new Error("Missing statement header");
      const command: ImportBankStatementCommand = { account_id: accountId, external_reference: externalReference, currency: account.currency, period_start: `${periodStart}T00:00:00+03:00`, period_end: `${periodEnd}T23:59:59+03:00`, opening_minor: openingMinor, closing_minor: closingMinor, lines: parseLines(rows, periodStart) };
      const created = await readPayload(await fetch("/api/live/banking/statements", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify(command) })) as BankStatement;
      setStatements((current) => [created, ...current]); setSelected(created); setExternalReference(""); setRows("");
      await loadDetail(created.id);
    } catch (error) { setProblem(problemFrom(error)); } finally { setBusy(false); }
  }

  async function matchLine(lineId: string, journalLineId: string) { if (!selected) return; setBusy(true); setProblem(null); try { const result = await readPayload(await fetch(`/api/live/banking/statements/${selected.id}/lines/${lineId}/matches`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ journal_line_id: journalLineId, reason: "Verified amount, account, currency, date and source evidence" }) })) as BankStatement; replaceStatement(result); } catch (error) { setProblem(problemFrom(error)); } finally { setBusy(false); } }
  async function reconcile() { if (!selected) return; setBusy(true); setProblem(null); try { const result = await readPayload(await fetch(`/api/live/banking/statements/${selected.id}/reconciliation`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ reason: "All statement lines independently reviewed and matched" }) })) as BankStatement; replaceStatement(result); } catch (error) { setProblem(problemFrom(error)); } finally { setBusy(false); } }

  const matchedCount = selected?.lines.filter((line) => line.match).length ?? 0;
  return <main className="page-stack banking-workbench">
    <header className="module-header"><div><p className="eyebrow">{l(text("Record to report", "Rekodi hadi ripoti"))}</p><h1>{l(text("Cash & bank reconciliation", "Upatanisho wa fedha na benki"))}</h1><p>{l(text("Import balanced statements, bind each movement to exact ledger evidence, then approve independently.", "Ingiza taarifa zilizosawazika, unganisha kila muamala na ushahidi wa leja, kisha idhinisha kwa kujitegemea."))}</p></div><LiveBadge context={workspace.context} /></header>
    <section className="metric-grid banking-metrics"><article className="metric-card"><p className="metric-label">{l(text("Accounts", "Akaunti"))}</p><strong>{workspace.accounts.length}</strong><small>{l(text("Exact working scope", "Upeo halisi wa kazi"))}</small></article><article className="metric-card"><p className="metric-label">{l(text("Imported statements", "Taarifa zilizoingizwa"))}</p><strong>{statements.length}</strong><small>{l(text("Immutable source evidence", "Ushahidi wa chanzo usiobadilika"))}</small></article><article className="metric-card"><p className="metric-label">{l(text("Awaiting reconciliation", "Zinasubiri upatanisho"))}</p><strong>{statements.filter((item) => item.status === "IMPORTED").length}</strong><small>{l(text("Maker-checker required", "Muundaji na mkaguzi wanahitajika"))}</small></article></section>
    <section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Import statement", "Ingiza taarifa ya benki"))}</h2><p>{l(text("One line per row: YYYY-MM-DD | reference | description | signed minor amount", "Mstari mmoja kwa muamala: YYYY-MM-DD | rejea | maelezo | kiasi chenye alama kwa senti"))}</p></div></div>{canImport ? <form className="bank-import-form" onSubmit={importStatement}><label>{l(text("Account", "Akaunti"))}<select required value={accountId} onChange={(event) => setAccountId(event.target.value)}>{workspace.accounts.map((account) => <option disabled={!account.active} key={account.id} value={account.id}>{account.code} · {account.name}</option>)}</select></label><label>{l(text("Statement reference", "Rejea ya taarifa"))}<input minLength={3} maxLength={120} required value={externalReference} onChange={(event) => setExternalReference(event.target.value)} /></label><label>{l(text("Period start", "Mwanzo wa kipindi"))}<input required type="date" value={periodStart} onChange={(event) => setPeriodStart(event.target.value)} /></label><label>{l(text("Period end", "Mwisho wa kipindi"))}<input required type="date" value={periodEnd} onChange={(event) => setPeriodEnd(event.target.value)} /></label><label>{l(text("Opening (minor)", "Salio la mwanzo (senti)"))}<input required type="number" value={openingMinor} onChange={(event) => setOpeningMinor(Number(event.target.value))} /></label><label>{l(text("Closing (minor)", "Salio la mwisho (senti)"))}<input required type="number" value={closingMinor} onChange={(event) => setClosingMinor(Number(event.target.value))} /></label><label className="bank-lines-input">{l(text("Statement lines", "Mistari ya taarifa"))}<textarea required rows={5} value={rows} onChange={(event) => setRows(event.target.value)} placeholder="2026-08-05 | TRX-100 | Customer transfer | 2500000" /></label><button className="primary-button" disabled={busy || !accountId} type="submit">{l(text("Validate & import", "Hakiki na ingiza"))}</button></form> : <div className="permission-empty"><strong>{l(text("Import permission required", "Ruhusa ya kuingiza inahitajika"))}</strong><p>{l(text("You may inspect reconciliation evidence but cannot import statements in this scope.", "Unaweza kukagua ushahidi wa upatanisho lakini huwezi kuingiza taarifa katika upeo huu."))}</p></div>}{problem ? <div className="sale-problem" role="alert"><span><strong>{problem.code}</strong>{problem.detail}</span></div> : null}</section>
    <section className="banking-grid"><article className="card records-card"><div className="records-toolbar"><div><h2>{l(text("Statement register", "Rejesta ya taarifa"))}</h2><span>{statements.length}</span></div></div><div className="statement-list">{statements.map((statement) => <button className={selected?.id === statement.id ? "active" : ""} disabled={busy} key={statement.id} onClick={() => loadDetail(statement.id)}><span><strong>{statement.external_reference}</strong><small>{compactId(statement.id)}</small></span><span className={`reconciliation-status ${statement.status === "RECONCILED" ? "status-resolved" : "status-open"}`}>{statement.status}</span><b>{formatMinorUnits(statement.closing_minor, statement.currency, locale)}</b></button>)}{statements.length === 0 ? <p className="bank-empty">{l(text("No statements imported in this scope.", "Hakuna taarifa zilizoingizwa katika upeo huu."))}</p> : null}</div></article>
      <article className="card bank-evidence"><div className="card-heading"><div><h2>{l(text("Reconciliation evidence", "Ushahidi wa upatanisho"))}</h2><p>{selected ? `${matchedCount}/${selected.lines.length} ${l(text("lines matched", "mistari imeunganishwa"))}` : l(text("Select a statement", "Chagua taarifa"))}</p></div>{selected ? <span className={`reconciliation-status ${selected.status === "RECONCILED" ? "status-resolved" : "status-open"}`}>{selected.status}</span> : null}</div>{selected ? <div className="bank-lines">{selected.lines.map((line) => <section key={line.id}><div><span><strong>{line.description}</strong><small>{formatTimestamp(line.transaction_at, locale, workspace.context.timezone)} · {line.external_reference || "—"}</small></span><b>{formatMinorUnits(line.amount_minor, selected.currency, locale)}</b></div>{line.match ? <p className="bank-match-ok">{l(text("Matched to ledger line", "Imeunganishwa na mstari wa leja"))} {line.match.journal_line_id}</p> : <div className="bank-candidates">{line.candidates.map((candidate) => canMatch ? <button className="secondary-button" disabled={busy} key={candidate.journal_line_id} onClick={() => matchLine(line.id, candidate.journal_line_id)}><span>{candidate.source_type} · {compactId(candidate.source_id)}</span><small>{candidate.memo}</small></button> : <small key={candidate.journal_line_id}>{candidate.source_type} · {compactId(candidate.source_id)} · {candidate.memo}</small>)}{line.candidates.length === 0 ? <small>{l(text("No unused exact ledger candidate.", "Hakuna rekodi sahihi ya leja isiyotumika."))}</small> : null}</div>}</section>)}</div> : <div className="permission-empty"><strong>{l(text("Evidence remains source-linked", "Ushahidi unabaki umeunganishwa na chanzo"))}</strong><p>{l(text("Choose a statement to inspect lines, exact candidates, match evidence, and approval state.", "Chagua taarifa kuona mistari, rekodi zinazolingana, ushahidi wa muunganiko na hali ya idhini."))}</p></div>}{selected?.status === "IMPORTED" && canReconcile ? <div className="bank-reconcile-action"><button className="primary-button" disabled={busy || matchedCount !== selected.lines.length || selected.imported_by === workspace.context.actor_id} onClick={reconcile}>{l(text("Approve reconciliation", "Idhinisha upatanisho"))}</button><small>{selected.imported_by === workspace.context.actor_id ? l(text("A different authorized user must approve this statement.", "Mtumiaji mwingine mwenye ruhusa lazima aidhinishe taarifa hii.")) : l(text("Approval records immutable evidence.", "Idhini huhifadhi ushahidi usiobadilika."))}</small></div> : null}</article>
    </section>
  </main>;
}
