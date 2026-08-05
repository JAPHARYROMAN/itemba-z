"use client";

import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";
import { formatMinorUnits } from "@/live-api/format";
import type {
  AdvancedFinanceTransitionCommand,
  AdvancedFinanceWorkspace,
  Budget,
  CreateBudgetCommand,
  CreateFixedAssetCommand,
  DisposeFixedAssetCommand,
  FixedAsset,
  PublicProblem,
} from "@/live-api/types";

type Action = Record<string, unknown> & {
  action: string;
  id?: string;
  command: unknown;
};
function errorOf(value: unknown): PublicProblem {
  return value && typeof value === "object" && "detail" in value
    ? (value as PublicProblem)
    : {
        type: "about:blank",
        title: "Failed",
        status: 500,
        code: "advanced_finance_failed",
        detail: value instanceof Error ? value.message : "The command failed.",
      };
}
async function send(body: Action) {
  const response = await fetch("/api/live/finance/advanced", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Idempotency-Key": crypto.randomUUID(),
    },
    body: JSON.stringify(body),
  });
  const value: unknown = await response.json();
  if (!response.ok) throw value;
  return value;
}

export function AdvancedFinance({
  workspace,
}: {
  workspace: AdvancedFinanceWorkspace;
}) {
  const { l, locale } = useLanguage();
  const money = (v: number) =>
    formatMinorUnits(v, workspace.context.currency, locale);
  const [budgets, setBudgets] = useState(workspace.budgets);
  const [assets, setAssets] = useState(workspace.assets);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<PublicProblem | null>(null);
  const permissions = new Set(workspace.context.permissions);
  const canBudgetManage = permissions.has("finance.budgets.manage");
  const canBudgetApprove = permissions.has("finance.budgets.approve");
  const canAssetManage = permissions.has("finance.assets.manage");
  const canAssetApprove = permissions.has("finance.assets.approve");
  const canDepreciate = permissions.has("finance.assets.depreciate");
  const canDispose = permissions.has("finance.assets.dispose");
  const accounts = (type?: string) =>
    workspace.accounts.filter((a) => !type || a.type === type);
  async function run(body: Action) {
    setBusy(true);
    setFailure(null);
    try {
      return await send(body);
    } catch (e) {
      setFailure(errorOf(e));
      return null;
    } finally {
      setBusy(false);
    }
  }
  async function transitionBudget(
    b: Budget,
    status: "SUBMITTED" | "APPROVED" | "REJECTED",
  ) {
    const value = (await run({
      action: "transition_budget",
      id: b.id,
      command: {
        status,
        reason: `${status} after governed budget evidence review`,
      } satisfies AdvancedFinanceTransitionCommand,
    })) as Budget | null;
    if (value) setBudgets((v) => v.map((x) => (x.id === value.id ? value : x)));
  }
  async function transitionAsset(
    a: FixedAsset,
    status: "SUBMITTED" | "ACTIVE" | "REJECTED",
  ) {
    const value = (await run({
      action: "transition_asset",
      id: a.id,
      command: {
        status,
        reason: `${status} after governed fixed asset evidence review`,
      },
    })) as FixedAsset | null;
    if (value) setAssets((v) => v.map((x) => (x.id === value.id ? value : x)));
  }
  return (
    <section className="page-stack">
      <header className="module-header">
        <div>
          <p className="eyebrow">
            {l(text("Plan and preserve", "Panga na linda mali"))}
          </p>
          <h1>
            {l(text("Budgets and fixed assets", "Bajeti na mali za kudumu"))}
          </h1>
          <p>
            {l(
              text(
                "Governed plans and an accounting-backed fixed-asset subledger.",
                "Mipango inayodhibitiwa na leja ndogo ya mali za kudumu inayounganishwa na uhasibu.",
              ),
            )}
          </p>
        </div>
      </header>
      {failure ? (
        <div className="problem-banner" role="alert">
          <strong>{failure.code}</strong>
          <span>{failure.detail}</span>
        </div>
      ) : null}
      <section className="card">
        <div className="card-heading">
          <div>
            <h2>{l(text("Budget governance", "Usimamizi wa bajeti"))}</h2>
          </div>
        </div>
        <BudgetForm
          disabled={busy || !canBudgetManage}
          accounts={accounts().filter(
            (a) => a.type === "REVENUE" || a.type === "EXPENSE",
          )}
          onCreate={async (c) => {
            const value = (await run({
              action: "create_budget",
              command: c,
            })) as Budget | null;
            if (value) setBudgets((v) => [value, ...v]);
          }}
        />
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>{l(text("Budget", "Bajeti"))}</th>
                <th>{l(text("Year", "Mwaka"))}</th>
                <th>{l(text("Status", "Hali"))}</th>
                <th>{l(text("Planned", "Iliyopangwa"))}</th>
                <th>{l(text("Control", "Udhibiti"))}</th>
              </tr>
            </thead>
            <tbody>
              {budgets.map((b) => (
                <tr key={b.id}>
                  <td>{b.name}</td>
                  <td>{b.fiscal_year}</td>
                  <td>{b.status}</td>
                  <td>
                    {money(b.lines.reduce((n, x) => n + x.amount_minor, 0))}
                  </td>
                  <td>
                    <div className="action-row">
                      {b.status === "DRAFT" ? (
                        <button
                          disabled={busy || !canBudgetManage}
                          className="button button-secondary"
                          onClick={() => transitionBudget(b, "SUBMITTED")}
                        >
                          {l(text("Submit", "Wasilisha"))}
                        </button>
                      ) : null}
                      {b.status === "SUBMITTED" ? (
                        <>
                          <button
                            disabled={busy || !canBudgetApprove}
                            className="button button-primary"
                            onClick={() => transitionBudget(b, "APPROVED")}
                          >
                            {l(text("Approve", "Idhinisha"))}
                          </button>
                          <button
                            disabled={busy || !canBudgetApprove}
                            className="button button-secondary"
                            onClick={() => transitionBudget(b, "REJECTED")}
                          >
                            {l(text("Reject", "Kataa"))}
                          </button>
                        </>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      <section className="card">
        <div className="card-heading">
          <div>
            <h2>
              {l(text("Fixed-asset register", "Rejesta ya mali za kudumu"))}
            </h2>
          </div>
        </div>
        <AssetForm
          disabled={busy || !canAssetManage}
          workspace={workspace}
          onCreate={async (c) => {
            const value = (await run({
              action: "create_asset",
              command: c,
            })) as FixedAsset | null;
            if (value) setAssets((v) => [value, ...v]);
          }}
        />
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>{l(text("Asset", "Mali"))}</th>
                <th>{l(text("Status", "Hali"))}</th>
                <th>{l(text("Cost", "Gharama"))}</th>
                <th>
                  {l(text("Accumulated depreciation", "Uchakavu uliokusanywa"))}
                </th>
                <th>{l(text("Net book value", "Thamani halisi"))}</th>
                <th>{l(text("Control", "Udhibiti"))}</th>
              </tr>
            </thead>
            <tbody>
              {assets.map((a) => (
                <tr key={a.id}>
                  <td>
                    <strong>{a.code}</strong>
                    <br />
                    {a.name}
                  </td>
                  <td>{a.status}</td>
                  <td>{money(a.cost_minor)}</td>
                  <td>{money(a.accumulated_depreciation_minor)}</td>
                  <td>{money(a.net_book_value_minor)}</td>
                  <td>
                    <div className="action-row">
                      {a.status === "DRAFT" ? (
                        <button
                          className="button button-secondary"
                          disabled={busy || !canAssetManage}
                          onClick={() => transitionAsset(a, "SUBMITTED")}
                        >
                          {l(text("Submit", "Wasilisha"))}
                        </button>
                      ) : null}
                      {a.status === "SUBMITTED" ? (
                        <>
                          <button
                            className="button button-primary"
                            disabled={busy || !canAssetApprove}
                            onClick={() => transitionAsset(a, "ACTIVE")}
                          >
                            {l(text("Capitalize", "Tambua kama mali"))}
                          </button>
                          <button
                            className="button button-secondary"
                            disabled={busy || !canAssetApprove}
                            onClick={() => transitionAsset(a, "REJECTED")}
                          >
                            {l(text("Reject", "Kataa"))}
                          </button>
                        </>
                      ) : null}
                      {a.status === "ACTIVE" ? (
                        <>
                          <button
                            className="button button-secondary"
                            disabled={busy || !canDepreciate}
                            onClick={async () => {
                              const parts = new Intl.DateTimeFormat("en", {
                                timeZone: workspace.context.timezone,
                                year: "numeric",
                                month: "2-digit",
                              }).formatToParts(new Date());
                              const period = `${parts.find((part) => part.type === "year")?.value}-${parts.find((part) => part.type === "month")?.value}-01`;
                              const d = (await run({
                                action: "depreciate_asset",
                                id: a.id,
                                command: {
                                  period,
                                  reason:
                                    "Approved monthly straight line depreciation",
                                },
                              })) as { amount_minor: number } | null;
                              if (d)
                                setAssets((v) =>
                                  v.map((x) =>
                                    x.id === a.id
                                      ? {
                                          ...x,
                                          accumulated_depreciation_minor:
                                            x.accumulated_depreciation_minor +
                                            d.amount_minor,
                                          net_book_value_minor:
                                            x.net_book_value_minor -
                                            d.amount_minor,
                                        }
                                      : x,
                                  ),
                                );
                            }}
                          >
                            {l(text("Post depreciation", "Rekodi uchakavu"))}
                          </button>
                          <AssetDisposal
                            disabled={busy || !canDispose}
                            accounts={accounts("ASSET")}
                            onDispose={async (command) => {
                              const value = (await run({
                                action: "dispose_asset",
                                id: a.id,
                                command,
                              })) as FixedAsset | null;
                              if (value)
                                setAssets((v) =>
                                  v.map((x) => (x.id === value.id ? value : x)),
                                );
                            }}
                          />
                        </>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </section>
  );
}

function AssetDisposal({
  disabled,
  accounts,
  onDispose,
}: {
  disabled: boolean;
  accounts: AdvancedFinanceWorkspace["accounts"];
  onDispose: (command: DisposeFixedAssetCommand) => void;
}) {
  const { l } = useLanguage();
  return (
    <form
      className="action-row"
      onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        onDispose({
          proceeds_minor: Math.round(Number(data.get("proceeds")) * 100),
          proceeds_account_id: String(data.get("account")),
          reason: "Approved fixed asset disposal evidence",
        });
      }}
    >
      <input
        aria-label={l(text("Disposal proceeds TZS", "Mapato ya uuzaji TZS"))}
        name="proceeds"
        type="number"
        min="0"
        step="0.01"
        defaultValue="0"
        required
      />
      <select
        aria-label={l(text("Proceeds account", "Akaunti ya mapato"))}
        name="account"
        required
      >
        {accounts.map((account) => (
          <option key={account.id} value={account.id}>
            {account.code}
          </option>
        ))}
      </select>
      <button className="button button-secondary" disabled={disabled}>
        {l(text("Dispose", "Ondoa mali"))}
      </button>
    </form>
  );
}

function BudgetForm({
  disabled,
  accounts,
  onCreate,
}: {
  disabled: boolean;
  accounts: AdvancedFinanceWorkspace["accounts"];
  onCreate: (c: CreateBudgetCommand) => void;
}) {
  const { l } = useLanguage();
  return (
    <form
      className="bank-import-form"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const year = Number(f.get("year"));
        onCreate({
          name: String(f.get("name")),
          fiscal_year: year,
          currency: "TZS",
          reason: String(f.get("reason")),
          lines: [
            {
              account_id: String(f.get("account")),
              month: `${year}-${String(f.get("month")).padStart(2, "0")}-01`,
              amount_minor: Math.round(Number(f.get("amount")) * 100),
            },
          ],
        });
      }}
    >
      <label>
        {l(text("Budget name", "Jina la bajeti"))}
        <input name="name" required minLength={3} />
      </label>
      <label>
        {l(text("Year", "Mwaka"))}
        <input
          name="year"
          type="number"
          defaultValue={new Date().getFullYear()}
          required
        />
      </label>
      <label>
        {l(text("Month", "Mwezi"))}
        <input
          name="month"
          type="number"
          min="1"
          max="12"
          defaultValue="1"
          required
        />
      </label>
      <label>
        {l(text("Account", "Akaunti"))}
        <select name="account" required>
          {accounts.map((a) => (
            <option key={a.id} value={a.id}>
              {a.code} · {a.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        {l(text("Amount TZS", "Kiasi TZS"))}
        <input name="amount" type="number" min="0" step="0.01" required />
      </label>
      <label>
        {l(text("Reason", "Sababu"))}
        <input name="reason" required minLength={8} />
      </label>
      <button disabled={disabled} className="button button-primary">
        {l(text("Create budget", "Unda bajeti"))}
      </button>
    </form>
  );
}
function AssetForm({
  disabled,
  workspace,
  onCreate,
}: {
  disabled: boolean;
  workspace: AdvancedFinanceWorkspace;
  onCreate: (c: CreateFixedAssetCommand) => void;
}) {
  const { l } = useLanguage();
  const assets = workspace.accounts.filter((a) => a.type === "ASSET"),
    expenses = workspace.accounts.filter((a) => a.type === "EXPENSE"),
    revenue = workspace.accounts.filter((a) => a.type === "REVENUE");
  const opts = (items: typeof assets) =>
    items.map((a) => (
      <option key={a.id} value={a.id}>
        {a.code} · {a.name}
      </option>
    ));
  return (
    <form
      className="bank-import-form"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        onCreate({
          code: String(f.get("code")),
          name: String(f.get("name")),
          category: String(f.get("category")),
          currency: "TZS",
          acquired_at: String(f.get("date")),
          cost_minor: Math.round(Number(f.get("cost")) * 100),
          residual_minor: Math.round(Number(f.get("residual")) * 100),
          useful_life_months: Number(f.get("life")),
          asset_account_id: String(f.get("asset")),
          accumulated_depreciation_account_id: String(f.get("accum")),
          depreciation_expense_account_id: String(f.get("expense")),
          capitalization_offset_account_id: String(f.get("offset")),
          disposal_gain_account_id: String(f.get("gain")),
          disposal_loss_account_id: String(f.get("loss")),
          reason: String(f.get("reason")),
        });
      }}
    >
      <label>
        {l(text("Code", "Namba"))}
        <input name="code" required />
      </label>
      <label>
        {l(text("Asset name", "Jina la mali"))}
        <input name="name" required />
      </label>
      <label>
        {l(text("Category", "Aina"))}
        <input name="category" required />
      </label>
      <label>
        {l(text("Acquired", "Ilinunuliwa"))}
        <input name="date" type="date" required />
      </label>
      <label>
        {l(text("Cost TZS", "Gharama TZS"))}
        <input name="cost" type="number" min="0.01" step="0.01" required />
      </label>
      <label>
        {l(text("Residual TZS", "Thamani ya mwisho TZS"))}
        <input
          name="residual"
          type="number"
          min="0"
          step="0.01"
          defaultValue="0"
          required
        />
      </label>
      <label>
        {l(text("Life months", "Miezi ya matumizi"))}
        <input name="life" type="number" min="1" max="1200" required />
      </label>
      <label>
        {l(text("Asset account", "Akaunti ya mali"))}
        <select name="asset" required>
          {opts(assets)}
        </select>
      </label>
      <label>
        {l(text("Accumulated depreciation", "Uchakavu uliokusanywa"))}
        <select name="accum" required>
          {opts(assets)}
        </select>
      </label>
      <label>
        {l(text("Depreciation expense", "Gharama ya uchakavu"))}
        <select name="expense" required>
          {opts(expenses)}
        </select>
      </label>
      <label>
        {l(text("Capitalization offset", "Akaunti ya mkabala"))}
        <select name="offset" required>
          {opts(workspace.accounts)}
        </select>
      </label>
      <label>
        {l(text("Disposal gain", "Faida ya uuzaji"))}
        <select name="gain" required>
          {opts(revenue)}
        </select>
      </label>
      <label>
        {l(text("Disposal loss", "Hasara ya uuzaji"))}
        <select name="loss" required>
          {opts(expenses)}
        </select>
      </label>
      <label>
        {l(text("Reason", "Sababu"))}
        <input name="reason" required minLength={8} />
      </label>
      <button disabled={disabled} className="button button-primary">
        {l(text("Register asset", "Sajili mali"))}
      </button>
    </form>
  );
}
