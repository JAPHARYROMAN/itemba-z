"use client";

import { Building2, CircleAlert, Coins, MapPin, Warehouse } from "lucide-react";
import { useEffect, useState } from "react";
import type { PublicProblem, WorkingContext } from "@/live-api/types";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";

type ContextState =
  | { state: "loading" }
  | { state: "ready"; context: WorkingContext }
  | { state: "unavailable"; problem: PublicProblem };

const initialState: ContextState = { state: "loading" };

export function LiveContextStrip() {
  const { t, l } = useLanguage();
  const [result, setResult] = useState<ContextState>(initialState);

  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/live/context", { cache: "no-store", signal: controller.signal, headers: { Accept: "application/json" } })
      .then(async (response) => {
        const payload = await response.json() as WorkingContext | PublicProblem;
        if (!response.ok) return { state: "unavailable", problem: payload as PublicProblem } satisfies ContextState;
        return { state: "ready", context: payload as WorkingContext } satisfies ContextState;
      })
      .then(setResult)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setResult({ state: "unavailable", problem: { type: "about:blank", title: "Live context unavailable", status: 503, code: "context_unavailable", detail: "The live context could not be loaded." } });
      });
    return () => controller.abort();
  }, []);

  if (result.state === "loading") {
    return <div className="context-bar context-loading" aria-live="polite"><div><span className="context-skeleton" /><span><small>{l(text("Live context", "Muktadha hai"))}</small><strong>{l(text("Authorizing…", "Inathibitisha…"))}</strong></span></div></div>;
  }
  if (result.state === "unavailable") {
    return <div className="context-bar context-unavailable" aria-live="polite"><div><CircleAlert size={15} /><span><small>{l(text("Live context", "Muktadha hai"))}</small><strong>{l(text("Unavailable — no mock data", "Haipatikani — hakuna data mbadala"))}</strong></span></div><div><span><small>{l(text("Reason", "Sababu"))}</small><strong>{result.problem.code}</strong></span></div></div>;
  }

  const { context } = result;
  return (
    <div className="context-bar" aria-label={t("activeContext")}>
      <div><Building2 size={15} /><span><small>{t("company")}</small><strong>{context.company_name}</strong></span></div>
      <div><MapPin size={15} /><span><small>{t("branch")}</small><strong>{context.branch_name}</strong></span></div>
      <div><Warehouse size={15} /><span><small>{l(text("Warehouse", "Ghala"))}</small><strong>{context.warehouse_name}</strong></span></div>
      <div><Coins size={15} /><span><small>{l(text("Operating currency", "Sarafu ya shughuli"))}</small><strong>{context.currency} <em>{l(text("Live", "Hai"))}</em></strong></span></div>
    </div>
  );
}
