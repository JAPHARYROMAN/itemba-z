"use client";

import { AlertTriangle, RefreshCw, Radio, ShieldCheck } from "lucide-react";
import type { PublicProblem, WorkingContext } from "@/live-api/types";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";

export function LiveBadge({ context }: { context?: WorkingContext }) {
  const { l } = useLanguage();
  return (
    <span className="live-badge">
      <Radio size={13} aria-hidden="true" />
      {l(text("Live ERP", "ERP hai"))}
      {context ? <small>{context.currency}</small> : null}
    </span>
  );
}

export function LiveUnavailable({ problem }: { problem: PublicProblem }) {
  const { l } = useLanguage();
  return (
    <section className="live-unavailable" role="alert">
      <span className="live-unavailable-icon"><AlertTriangle size={25} /></span>
      <div>
        <p className="eyebrow">{l(text("Live data unavailable", "Data hai haipatikani"))}</p>
        <h1>{l(text("The sales workspace is safely closed", "Eneo la mauzo limefungwa kwa usalama"))}</h1>
        <p>{l(text(
          "The Control Center could not establish an authenticated live ERP connection. No demonstration data has been substituted.",
          "Control Center haikuweza kuunganisha ERP hai kwa uthibitisho. Hakuna data ya maonyesho iliyotumika badala yake.",
        ))}</p>
        <div className="problem-detail"><ShieldCheck size={16} /><span><strong>{problem.code}</strong>{problem.detail}{problem.correlation_id ? <small>Correlation · {problem.correlation_id}</small> : null}</span></div>
        <button type="button" className="primary-button" onClick={() => window.location.reload()}><RefreshCw size={16} />{l(text("Try live connection again", "Jaribu muunganisho tena"))}</button>
      </div>
    </section>
  );
}

export function LiveLoading() {
  const { l } = useLanguage();
  return <div className="live-loading" role="status"><span /><div><strong>{l(text("Connecting to live ERP", "Inaunganisha ERP hai"))}</strong><small>{l(text("Authorizing scope and controls…", "Inathibitisha upeo na udhibiti…"))}</small></div></div>;
}
