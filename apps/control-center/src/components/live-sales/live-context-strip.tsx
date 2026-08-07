"use client";

import { Building2, CircleAlert, Coins, MapPin, Warehouse } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { useShellContext } from "@/components/shell/shell-context";

export function LiveContextStrip() {
  const { t } = useLanguage();
  const { context } = useShellContext();

  if (context.state === "loading") {
    return (
      <div className="calm-context-state" role="status" aria-live="polite">
        <span className="calm-context-spinner" aria-hidden="true" />
        <span>{t("contextLoading")}</span>
      </div>
    );
  }

  if (context.state === "unavailable" || context.state === "not-permitted") {
    return (
      <div className="calm-context-state calm-context-state-error" role="status" aria-live="polite">
        <CircleAlert size={18} aria-hidden="true" />
        <span><strong>{t("contextUnavailable")}</strong><small>{t("contextUnavailableHint")}</small></span>
      </div>
    );
  }

  const workingContext = context.data;
  return (
    <dl className="calm-context-strip" aria-label={t("activeContext")}>
      <div className="calm-context-item">
        <dt><Building2 size={18} aria-hidden="true" /><span>{t("company")}</span></dt>
        <dd>{workingContext.company_name}</dd>
      </div>
      <div className="calm-context-item">
        <dt><MapPin size={18} aria-hidden="true" /><span>{t("branch")}</span></dt>
        <dd>{workingContext.branch_name}</dd>
      </div>
      <div className="calm-context-item">
        <dt><Warehouse size={18} aria-hidden="true" /><span>{t("warehouse")}</span></dt>
        <dd>{workingContext.warehouse_name}</dd>
      </div>
      <div className="calm-context-item calm-context-currency">
        <dt><Coins size={18} aria-hidden="true" /><span>{t("currency")}</span></dt>
        <dd>{workingContext.currency}</dd>
      </div>
    </dl>
  );
}
