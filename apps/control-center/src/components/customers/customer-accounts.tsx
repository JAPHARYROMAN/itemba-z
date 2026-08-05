"use client";

import Link from "next/link";
import { CircleDollarSign, Landmark, ShieldCheck, Users } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { formatMinorUnits } from "@/live-api/format";
import type { CustomerAccountsWorkspace } from "@/live-api/types";
import { text } from "@/lib/i18n";

export function CustomerAccounts({ workspace }: { workspace: CustomerAccountsWorkspace }) {
  const { locale, l } = useLanguage();
  const enabled = workspace.customers.filter((customer) => customer.credit_enabled).length;
  const exposure = workspace.customers.reduce((sum, customer) => sum + customer.current_exposure_minor, 0);
  return <div className="page-stack">
    <section className="page-heading module-heading"><div><div className="heading-badges"><LiveBadge context={workspace.context} /><span className="scope-chip">{workspace.context.company_name}</span></div><h1>{l(text("Customer accounts", "Akaunti za wateja"))}</h1><p>{l(text("Authoritative credit exposure and account access for the selected legal company.", "Mfiduo rasmi wa mikopo na akaunti kwa kampuni ya kisheria iliyochaguliwa."))}</p></div></section>
    <div className="metric-grid compact-metrics">
      <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><Users size={17} /></span></div><p>{l(text("Customers", "Wateja"))}</p><strong>{workspace.customers.length}</strong></article>
      <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><ShieldCheck size={17} /></span></div><p>{l(text("Credit enabled", "Wenye mkopo"))}</p><strong>{enabled}</strong></article>
      <article className="metric-card"><div className="metric-card-top"><span className="metric-icon"><Landmark size={17} /></span></div><p>{l(text("Current exposure", "Mfiduo wa sasa"))}</p><strong>{formatMinorUnits(exposure, workspace.context.currency, locale)}</strong></article>
    </div>
    <section className="card records-card"><div className="card-heading"><div><p className="eyebrow">{l(text("Live receivables", "Madeni hai"))}</p><h2>{l(text("Account register", "Rejesta ya akaunti"))}</h2></div><CircleDollarSign size={20} /></div>
      <div className="table-scroll"><table className="data-table"><thead><tr><th>{l(text("Customer", "Mteja"))}</th><th>{l(text("Credit", "Mkopo"))}</th><th className="align-right">{l(text("Exposure", "Mfiduo"))}</th><th className="align-right">{l(text("Available", "Inayopatikana"))}</th></tr></thead><tbody>{workspace.customers.map((customer) => <tr key={customer.id}><td><Link href={`/customers/${customer.id}`}><strong>{customer.name}</strong><small className="table-subline">{customer.code}</small></Link></td><td>{customer.is_general_customer ? l(text("General / cash only", "Jumla / taslimu tu")) : customer.credit_enabled ? l(text("Enabled", "Umeruhusiwa")) : l(text("Disabled", "Umezimwa"))}</td><td className="align-right numeric">{formatMinorUnits(customer.current_exposure_minor, workspace.context.currency, locale)}</td><td className="align-right numeric">{formatMinorUnits(customer.available_credit_minor, workspace.context.currency, locale)}</td></tr>)}</tbody></table></div>
    </section>
  </div>;
}
