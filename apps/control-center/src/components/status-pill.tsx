"use client";

import type { Status } from "@/domain/erp";
import { useLanguage } from "@/components/language-provider";

export function StatusPill({ status, compact = false }: { status: Status; compact?: boolean }) {
  const { l } = useLanguage();
  return <span className={`status-pill status-${status.tone}${compact ? " status-compact" : ""}`}><span aria-hidden="true" />{l(status.label)}</span>;
}
