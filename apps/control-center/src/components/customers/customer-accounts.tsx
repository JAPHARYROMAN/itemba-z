"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { AlertTriangle, CircleDollarSign, Search, ShieldCheck, UserRoundPlus, Users } from "lucide-react";
import { CommercialModuleNav } from "@/components/commercial/commercial-module-nav";
import { useLanguage } from "@/components/language-provider";
import { formatMinorUnits } from "@/live-api/format";
import type { CustomerAccountsWorkspace } from "@/live-api/types";
import { text } from "@/lib/i18n";
import styles from "@/components/commercial/commercial.module.css";

type Filter = "all" | "credit" | "cash" | "exposure";

export function CustomerAccounts({ workspace }: { workspace: CustomerAccountsWorkspace }) {
  const { locale, l } = useLanguage();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const customers = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("en");
    return workspace.customers.filter((customer) => {
      const matches = !needle || customer.name.toLocaleLowerCase("en").includes(needle) || customer.code.toLocaleLowerCase("en").includes(needle);
      const filtered = filter === "all" || (filter === "credit" && customer.credit_enabled) || (filter === "cash" && !customer.credit_enabled) || (filter === "exposure" && customer.current_exposure_minor > 0);
      return matches && filtered;
    });
  }, [filter, query, workspace.customers]);
  const enabled = workspace.customers.filter((customer) => customer.credit_enabled).length;
  const exposed = workspace.customers.filter((customer) => customer.current_exposure_minor > 0).length;
  const exposure = workspace.customers.reduce((sum, customer) => sum + customer.current_exposure_minor, 0);

  return <div className={styles.page}>
    <CommercialModuleNav area="customers" permissions={workspace.context.permissions} />
    <header className={styles.hero}><div><p className={styles.scope}>{workspace.context.company_name} · {workspace.context.branch_name}</p><h1>{l(text("Customers", "Wateja"))}</h1><p>{l(text("Find an account, understand exposure, and move directly to the next customer task.", "Tafuta akaunti, elewa mfiduo, na uende moja kwa moja kwenye jukumu linalofuata la mteja."))}</p></div>{workspace.context.permissions.includes("customers.collections.post") ? <Link className={styles.heroAction} href="/sales/payments"><CircleDollarSign size={17} />{l(text("Record payment", "Rekodi malipo"))}</Link> : null}</header>
    <section className={styles.metrics} aria-label={l(text("Customer account summary", "Muhtasari wa akaunti za wateja"))}>
      <article className={styles.metric}><span><Users size={17} />{l(text("Customers", "Wateja"))}</span><strong>{workspace.customers.length}</strong></article>
      <article className={styles.metric}><span><ShieldCheck size={17} />{l(text("Credit enabled", "Wenye mkopo"))}</span><strong>{enabled}</strong></article>
      <article className={styles.metric}><span><CircleDollarSign size={17} />{l(text("Current exposure", "Mfiduo wa sasa"))}</span><strong>{formatMinorUnits(exposure, workspace.context.currency, locale)}</strong></article>
    </section>
    {exposed > 0 ? <aside className={styles.attention}><AlertTriangle size={19} /><div><strong>{l(text(`${exposed} account${exposed === 1 ? "" : "s"} need collection attention`, `Akaunti ${exposed} zinahitaji ufuatiliaji wa makusanyo`))}</strong><p>{l(text("Open each account to review invoice aging before recording a collection.", "Fungua kila akaunti kukagua umri wa ankara kabla ya kurekodi makusanyo."))}</p></div></aside> : null}
    <section className={styles.card}><div className={styles.toolbar}><div><h2>{l(text("Customer directory", "Orodha ya wateja"))}</h2><p aria-live="polite">{customers.length} {l(text("results", "matokeo"))}</p></div><div className={styles.filters}><label className={styles.search}><Search size={16} /><span className="sr-only">{l(text("Search customers", "Tafuta wateja"))}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={l(text("Search name or code", "Tafuta jina au msimbo"))} /></label><label className={styles.select}><span className="sr-only">{l(text("Filter customers", "Chuja wateja"))}</span><select value={filter} onChange={(event) => setFilter(event.target.value as Filter)}><option value="all">{l(text("All customers", "Wateja wote"))}</option><option value="credit">{l(text("Credit enabled", "Wenye mkopo"))}</option><option value="cash">{l(text("Cash only", "Taslimu tu"))}</option><option value="exposure">{l(text("With exposure", "Wenye mfiduo"))}</option></select></label></div></div>
      {customers.length ? <div className={styles.tableRegion} tabIndex={0} role="region" aria-label={l(text("Customer accounts table", "Jedwali la akaunti za wateja"))}><table className={styles.table}><thead><tr><th>{l(text("Customer", "Mteja"))}</th><th>{l(text("Buying terms", "Masharti ya ununuzi"))}</th><th className={styles.alignRight}>{l(text("Exposure", "Mfiduo"))}</th><th className={styles.alignRight}>{l(text("Available credit", "Mkopo uliopo"))}</th></tr></thead><tbody>{customers.map((customer) => <tr key={customer.id}><td><Link className={styles.primaryLink} href={`/customers/${customer.id}`}>{customer.name}<small>{customer.code}</small></Link></td><td data-label={l(text("Buying terms", "Masharti"))}><span className={`${styles.status} ${customer.credit_enabled ? styles.success : styles.neutral}`}>{customer.is_general_customer ? l(text("General · cash only", "Jumla · taslimu tu")) : customer.credit_enabled ? l(text("Credit enabled", "Mkopo umeruhusiwa")) : l(text("Cash only", "Taslimu tu"))}</span></td><td data-label={l(text("Exposure", "Mfiduo"))} className={styles.alignRight}>{formatMinorUnits(customer.current_exposure_minor, workspace.context.currency, locale)}</td><td data-label={l(text("Available", "Iliyopo"))} className={styles.alignRight}>{customer.credit_enabled ? formatMinorUnits(customer.available_credit_minor, workspace.context.currency, locale) : "—"}</td></tr>)}</tbody></table></div> : <div className={styles.empty}><UserRoundPlus size={30} /><strong>{l(text("No customers match these filters", "Hakuna mteja anayelingana na vichujio hivi"))}</strong><p>{l(text("Clear the search or choose a different account filter.", "Futa utafutaji au chagua kichujio tofauti cha akaunti."))}</p></div>}
    </section>
  </div>;
}
