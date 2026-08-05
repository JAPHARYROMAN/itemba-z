"use client";
import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";
import { formatMinorUnits } from "@/live-api/format";
import type {
  AdvancedFinanceWorkspace,
  BudgetActual as BudgetActualType,
} from "@/live-api/types";

export function BudgetActual({
  workspace,
}: {
  workspace: AdvancedFinanceWorkspace;
}) {
  const { l, locale } = useLanguage();
  const approved = workspace.budgets.filter((b) => b.status === "APPROVED");
  const [id, setId] = useState(approved[0]?.id ?? "");
  const [report, setReport] = useState<BudgetActualType | null>(null);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");
  const money = (v: number) =>
    formatMinorUnits(v, workspace.context.currency, locale);
  async function run() {
    setBusy(true);
    setFailure("");
    try {
      const response = await fetch(
        `/api/live/finance/advanced?budget_id=${encodeURIComponent(id)}`,
        { cache: "no-store" },
      );
      if (!response.ok) throw await response.json();
      setReport((await response.json()) as BudgetActualType);
    } catch (error) {
      setFailure(
        error && typeof error === "object" && "detail" in error
          ? String(error.detail)
          : "The budget comparison could not be completed.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="card">
      <div className="card-heading">
        <div>
          <h2>{l(text("Budget versus actual", "Bajeti dhidi ya halisi"))}</h2>
          <p>
            {l(
              text(
                "Actuals are derived from the immutable general ledger.",
                "Kiasi halisi kinatokana na leja kuu isiyobadilika.",
              ),
            )}
          </p>
        </div>
      </div>
      {failure ? (
        <div className="problem-banner" role="alert">
          {failure}
        </div>
      ) : null}
      <div className="action-row">
        <select
          aria-label={l(text("Approved budget", "Bajeti iliyoidhinishwa"))}
          value={id}
          onChange={(e) => setId(e.target.value)}
        >
          {approved.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name} · {b.fiscal_year}
            </option>
          ))}
        </select>
        <button
          className="button button-primary"
          disabled={!id || busy}
          onClick={run}
        >
          {l(text("Run comparison", "Linganisha"))}
        </button>
      </div>
      {report ? (
        <>
          <section className="metric-grid">
            <article className="metric-card">
              <p className="metric-label">{l(text("Budget", "Bajeti"))}</p>
              <strong>{money(report.total_budget_minor)}</strong>
            </article>
            <article className="metric-card">
              <p className="metric-label">{l(text("Actual", "Halisi"))}</p>
              <strong>{money(report.total_actual_minor)}</strong>
            </article>
            <article className="metric-card">
              <p className="metric-label">{l(text("Variance", "Tofauti"))}</p>
              <strong>{money(report.total_variance_minor)}</strong>
            </article>
          </section>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>{l(text("Month", "Mwezi"))}</th>
                  <th>{l(text("Account", "Akaunti"))}</th>
                  <th>{l(text("Budget", "Bajeti"))}</th>
                  <th>{l(text("Actual", "Halisi"))}</th>
                  <th>{l(text("Variance", "Tofauti"))}</th>
                </tr>
              </thead>
              <tbody>
                {report.lines.map((x) => (
                  <tr key={`${x.account_id}-${x.month}`}>
                    <td>{x.month.slice(0, 7)}</td>
                    <td>
                      {x.code} · {x.name}
                    </td>
                    <td>{money(x.budget_minor)}</td>
                    <td>{money(x.actual_minor)}</td>
                    <td>{money(x.variance_minor)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : null}
    </section>
  );
}
