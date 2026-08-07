"use client";

import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";
import type { CreateGLAccountCommand, CreatePostingMappingCommand, FinanceControlWorkspace, GLAccount, PostingMapping, PublicProblem } from "@/live-api/types";

const mappingKeys: CreatePostingMappingCommand["key"][] = ["SALES_RECEIVABLE", "SALES_TAX_PAYABLE", "PAYMENT_CASH", "PAYMENT_MOBILE_MONEY", "PAYMENT_BANK_CARD", "PAYMENT_BANK_TRANSFER", "PROCUREMENT_GRNI", "PROCUREMENT_PAYABLE", "INVENTORY_ADJUSTMENT", "STOCK_IN_TRANSIT"];

async function payload(response: Response): Promise<unknown> { const value: unknown = await response.json(); if (!response.ok) throw value; return value; }
function asProblem(value: unknown): PublicProblem { if (value && typeof value === "object" && "detail" in value) return value as PublicProblem; return { type: "about:blank", title: "Request failed", status: 500, code: "account_governance_failed", detail: value instanceof Error ? value.message : "The accounting configuration command failed." }; }

export function AccountGovernance({ workspace }: { workspace: FinanceControlWorkspace }) {
  const { l } = useLanguage();
  const [accounts, setAccounts] = useState(workspace.glAccounts);
  const [mappings, setMappings] = useState(workspace.postingMappings);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [accountType, setAccountType] = useState<CreateGLAccountCommand["type"]>("ASSET");
  const [control, setControl] = useState(false);
  const [manual, setManual] = useState(false);
  const [mappingKey, setMappingKey] = useState<CreatePostingMappingCommand["key"]>("SALES_RECEIVABLE");
  const [mappingAccount, setMappingAccount] = useState(workspace.glAccounts.find((item) => item.status === "ACTIVE")?.id ?? "");
  const [effectiveDate, setEffectiveDate] = useState("");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const canManage = workspace.context.permissions.includes("finance.accounts.manage");
  const canApprove = workspace.context.permissions.includes("finance.accounts.approve");

  async function submitAccount(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setProblem(null);
    try {
      const command: CreateGLAccountCommand = { code, name, type: accountType, control_account: control, allow_manual_posting: manual };
      const result = await payload(await fetch("/api/live/finance/accounts", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify(command) })) as GLAccount;
      setAccounts((current) => [...current, result].toSorted((a, b) => a.code.localeCompare(b.code))); setCode(""); setName(""); setControl(false); setManual(false);
    } catch (error) { setProblem(asProblem(error)); } finally { setBusy(false); }
  }
  async function decideAccount(account: GLAccount, status: "ACTIVE" | "REJECTED") {
    setBusy(true); setProblem(null);
    try { const result = await payload(await fetch(`/api/live/finance/accounts/${encodeURIComponent(account.id)}/decisions`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ status, reason: status === "ACTIVE" ? "Independent account classification review completed" : "Account classification requires correction" }) })) as GLAccount; setAccounts((current) => current.map((item) => item.id === result.id ? result : item)); if (status === "ACTIVE") setMappingAccount((current) => current || result.id); }
    catch (error) { setProblem(asProblem(error)); } finally { setBusy(false); }
  }
  async function submitMapping(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setProblem(null);
    try {
      if (!effectiveDate) throw new Error("Effective date is required");
      const command: CreatePostingMappingCommand = { key: mappingKey, account_id: mappingAccount, effective_from: `${effectiveDate}T00:00:00+03:00`, reason: "Effective posting rule submitted for review" };
      const result = await payload(await fetch("/api/live/finance/posting-mappings", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify(command) })) as PostingMapping;
      setMappings((current) => [result, ...current]);
    } catch (error) { setProblem(asProblem(error)); } finally { setBusy(false); }
  }
  async function decideMapping(mapping: PostingMapping, status: "ACTIVE" | "REJECTED") {
    setBusy(true); setProblem(null);
    try { const result = await payload(await fetch(`/api/live/finance/posting-mappings/${mapping.id}/decisions`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ status, reason: status === "ACTIVE" ? "Independent effective posting review completed" : "Posting rule requires correction" }) })) as PostingMapping; setMappings((current) => current.map((item) => item.id === result.id ? result : item)); }
    catch (error) { setProblem(asProblem(error)); } finally { setBusy(false); }
  }

  return <section className="page-stack finance-controls">
    <header className="module-header"><div><p className="eyebrow">{l(text("Accounting foundation", "Msingi wa uhasibu"))}</p><h1>{l(text("Chart of accounts & posting rules", "Mpangilio wa akaunti na kanuni za uchapishaji"))}</h1><p>{l(text("Govern legal-company accounts and effective-dated posting mappings with independent approval.", "Dhibiti akaunti za kampuni na upangaji wa uchapishaji kwa tarehe kwa idhini huru."))}</p></div></header>
    {problem ? <div className="problem-banner" role="alert"><strong>{problem.code}</strong><span>{problem.detail}</span></div> : null}
    <section className="metric-grid">
      <article className="metric-card"><p className="metric-label">{l(text("Active accounts", "Akaunti hai"))}</p><strong>{accounts.filter((item) => item.status === "ACTIVE").length}</strong><small>{l(text("Approved ledger structure", "Muundo wa leja ulioidhinishwa"))}</small></article>
      <article className="metric-card"><p className="metric-label">{l(text("Pending review", "Zinasubiri ukaguzi"))}</p><strong>{accounts.filter((item) => item.status === "SUBMITTED").length + mappings.filter((item) => item.status === "SUBMITTED").length}</strong><small>{l(text("Maker-checker queue", "Foleni ya mtayarishaji na mkaguzi"))}</small></article>
      <article className="metric-card"><p className="metric-label">{l(text("Active mappings", "Mipangilio hai"))}</p><strong>{mappings.filter((item) => item.status === "ACTIVE").length}</strong><small>{l(text("Effective posting controls", "Udhibiti wa uchapishaji unaotumika"))}</small></article>
    </section>
    {canManage ? <section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Submit ledger account", "Wasilisha akaunti ya leja"))}</h2><p>{l(text("Control accounts cannot accept manual journals.", "Akaunti za udhibiti hazikubali majarida ya mkono."))}</p></div></div><form className="bank-import-form" onSubmit={submitAccount}>
      <label>{l(text("Account code", "Msimbo wa akaunti"))}<input required minLength={2} value={code} onChange={(event) => setCode(event.target.value.toLowerCase())} /></label>
      <label>{l(text("Account name", "Jina la akaunti"))}<input required minLength={2} value={name} onChange={(event) => setName(event.target.value)} /></label>
      <label>{l(text("Account type", "Aina ya akaunti"))}<select value={accountType} onChange={(event) => setAccountType(event.target.value as CreateGLAccountCommand["type"])}>{["ASSET", "LIABILITY", "EQUITY", "REVENUE", "EXPENSE"].map((item) => <option key={item}>{item}</option>)}</select></label>
      <label><input type="checkbox" checked={control} onChange={(event) => { setControl(event.target.checked); if (event.target.checked) setManual(false); }} /> {l(text("Control account", "Akaunti ya udhibiti"))}</label>
      <label><input type="checkbox" disabled={control} checked={manual} onChange={(event) => setManual(event.target.checked)} /> {l(text("Allow manual journals", "Ruhusu majarida ya mkono"))}</label>
      <button className="button button-primary" disabled={busy}>{l(text("Submit account", "Wasilisha akaunti"))}</button>
    </form></section> : null}
    <section className="card"><div className="card-heading"><div><h2>{l(text("Account register", "Rejesta ya akaunti"))}</h2><p>{l(text("Posted account facts remain immutable.", "Taarifa za akaunti zilizochapishwa hazibadiliki."))}</p></div></div><div className="table-scroll"><table><thead><tr><th>{l(text("Code", "Msimbo"))}</th><th>{l(text("Name", "Jina"))}</th><th>{l(text("Type", "Aina"))}</th><th>{l(text("Status", "Hali"))}</th><th>{l(text("Actions", "Vitendo"))}</th></tr></thead><tbody>{accounts.map((account) => <tr key={account.record_id}><td>{account.code}</td><td>{account.name}</td><td>{account.type}</td><td><span className={`status-pill status-${account.status.toLowerCase()}`}>{account.status}</span></td><td>{canApprove && account.status === "SUBMITTED" ? <div className="action-row"><button className="button button-secondary" disabled={busy} onClick={() => decideAccount(account, "ACTIVE")}>{l(text("Approve", "Idhinisha"))}</button><button className="button button-ghost" disabled={busy} onClick={() => decideAccount(account, "REJECTED")}>{l(text("Reject", "Kataa"))}</button></div> : "—"}</td></tr>)}</tbody></table></div></section>
    {canManage ? <section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Submit posting mapping", "Wasilisha upangaji wa uchapishaji"))}</h2><p>{l(text("Only active, type-compatible accounts are accepted.", "Akaunti hai na zenye aina inayofaa pekee zinakubaliwa."))}</p></div></div><form className="bank-import-form" onSubmit={submitMapping}>
      <label>{l(text("Posting purpose", "Lengo la uchapishaji"))}<select value={mappingKey} onChange={(event) => setMappingKey(event.target.value as CreatePostingMappingCommand["key"])}>{mappingKeys.map((item) => <option key={item}>{item}</option>)}</select></label>
      <label>{l(text("GL account", "Akaunti ya leja"))}<select required value={mappingAccount} onChange={(event) => setMappingAccount(event.target.value)}>{accounts.filter((item) => item.status === "ACTIVE").map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name}</option>)}</select></label>
      <label>{l(text("Effective date", "Tarehe ya kuanza"))}<input type="date" required value={effectiveDate} onChange={(event) => setEffectiveDate(event.target.value)} /></label>
      <button className="button button-primary" disabled={busy}>{l(text("Submit mapping", "Wasilisha upangaji"))}</button>
    </form></section> : null}
    <section className="card"><div className="card-heading"><div><h2>{l(text("Posting mapping register", "Rejesta ya upangaji wa uchapishaji"))}</h2></div></div><div className="table-scroll"><table><thead><tr><th>{l(text("Purpose", "Lengo"))}</th><th>{l(text("Account", "Akaunti"))}</th><th>{l(text("Effective", "Inaanza"))}</th><th>{l(text("Status", "Hali"))}</th><th>{l(text("Actions", "Vitendo"))}</th></tr></thead><tbody>{mappings.map((mapping) => <tr key={mapping.id}><td>{mapping.key}</td><td>{mapping.account_id}</td><td>{mapping.effective_from.slice(0, 10)}</td><td><span className={`status-pill status-${mapping.status.toLowerCase()}`}>{mapping.status}</span></td><td>{canApprove && mapping.status === "SUBMITTED" ? <div className="action-row"><button className="button button-secondary" disabled={busy} onClick={() => decideMapping(mapping, "ACTIVE")}>{l(text("Approve", "Idhinisha"))}</button><button className="button button-ghost" disabled={busy} onClick={() => decideMapping(mapping, "REJECTED")}>{l(text("Reject", "Kataa"))}</button></div> : "—"}</td></tr>)}</tbody></table></div></section>
  </section>;
}
