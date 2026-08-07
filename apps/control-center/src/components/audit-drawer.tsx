"use client";

import { useEffect, useRef, useState } from "react";
import { History, TriangleAlert, X } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";
import { compactId, formatTimestamp } from "@/live-api/format";
import type { AuditTrail, PublicProblem, WorkingContext } from "@/live-api/types";

export function AuditDrawer({ entityType, entityId, context }: { entityType: string; entityId: string; context: WorkingContext }) {
  const { locale, l } = useLanguage();
  const [open, setOpen] = useState(false);
  const [trail, setTrail] = useState<AuditTrail | null>(null);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const [loading, setLoading] = useState(false);
  const closeButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    closeButton.current?.focus();
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") setOpen(false); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [open]);
  async function show() {
    setOpen(true);
    if (trail || loading) return;
    setLoading(true); setProblem(null);
    try {
      const response = await fetch(`/api/live/audit/${encodeURIComponent(entityType)}/${encodeURIComponent(entityId)}`, { headers: { Accept: "application/json" } });
      const payload = await response.json();
      if (!response.ok) throw payload;
      setTrail(payload as AuditTrail);
    } catch (error) { setProblem(error as PublicProblem); } finally { setLoading(false); }
  }
  return <><button type="button" className="secondary-button" onClick={show}><History size={17} />{l(text("Audit trail", "Historia ya ukaguzi"))}</button>{open ? <div className="audit-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setOpen(false); }}><aside className="audit-drawer" role="dialog" aria-modal="true" aria-labelledby="audit-title"><header><div><p className="eyebrow">LIVE · {entityType}</p><h2 id="audit-title">{l(text("Immutable audit trail", "Historia ya ukaguzi isiyobadilika"))}</h2><small>{compactId(entityId)}</small></div><button ref={closeButton} type="button" className="icon-button" onClick={() => setOpen(false)} aria-label={l(text("Close audit trail", "Funga historia ya ukaguzi"))}><X size={20} /></button></header>{loading ? <div className="notification-state" role="status">{l(text("Loading authoritative events…", "Inapakia matukio halali…"))}</div> : problem ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>{problem.code}</strong>{problem.detail}</span></div> : trail?.items.length ? <ol className="audit-list">{trail.items.map((event) => <li key={event.id}><span className="notice-dot info" /><div><strong>{event.action}</strong><p>{formatTimestamp(event.occurred_at, locale, context.timezone)}</p><small>{l(text("Actor", "Mtendaji"))} · {compactId(event.actor_id)} · {l(text("Correlation", "Uhusiano"))} · {compactId(event.correlation_id)}</small><details><summary>{l(text("Evidence payload", "Data ya ushahidi"))}</summary><pre>{JSON.stringify(event.data, null, 2)}</pre></details></div></li>)}</ol> : <div className="notification-state"><strong>{l(text("No audit event is visible for this record.", "Hakuna tukio la ukaguzi linaloonekana kwa rekodi hii."))}</strong></div>}</aside></div> : null}</>;
}
