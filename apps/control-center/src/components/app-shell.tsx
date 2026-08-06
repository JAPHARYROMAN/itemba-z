"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import {
  Bell, ChartNoAxesCombined, ClipboardList,
  Landmark, LayoutDashboard, Menu, PackageOpen, Search, Settings2,
  ShoppingCart, Smartphone, Truck, UserRoundCog, UsersRound, Wifi, X, ListChecks,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { BrandMark } from "@/components/brand-mark";
import { LiveContextStrip } from "@/components/live-sales/live-context-strip";
import { useLanguage } from "@/components/language-provider";
import { navigationGroups, type NavigationIcon } from "@/lib/navigation";
import { text } from "@/lib/i18n";
import { SessionControl } from "@/components/session-control";
import type { Dashboard, PublicProblem } from "@/live-api/types";

const iconMap: Record<NavigationIcon, LucideIcon> = {
  dashboard: LayoutDashboard, customers: UsersRound, suppliers: Truck, sales: ShoppingCart,
  purchases: ClipboardList, inventory: PackageOpen, finance: Landmark, people: UserRoundCog,
  reports: ChartNoAxesCombined, settings: Settings2,
  reconciliation: ListChecks,
  devices: Smartphone,
};

export function AppShell({ children }: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  const { locale, setLocale, t, l } = useLanguage();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [notificationProblem, setNotificationProblem] = useState<PublicProblem | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (pathname === "/login") return;
    const controller = new AbortController();
    fetch("/api/live/dashboard", { signal: controller.signal, headers: { Accept: "application/json" } })
      .then(async (response) => {
        const payload = await response.json();
        if (!response.ok) throw payload;
        setDashboard(payload as Dashboard);
      })
      .catch((error: PublicProblem | DOMException) => {
        if (!(error instanceof DOMException && error.name === "AbortError")) setNotificationProblem(error as PublicProblem);
      });
    return () => controller.abort();
  }, [pathname]);

  useEffect(() => {
    const focusSearch = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLocaleLowerCase() === "k") {
        event.preventDefault(); searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", focusSearch);
    return () => window.removeEventListener("keydown", focusSearch);
  }, []);

  if (pathname === "/login") return <>{children}</>;

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">{t("skipToContent")}</a>
      <aside className={`sidebar${navigationOpen ? " sidebar-open" : ""}`} aria-label={t("primaryNavigation")}>
        <div className="sidebar-brand">
          <Link href="/" className="brand-link" onClick={() => setNavigationOpen(false)} aria-label="ITEMBA-Z dashboard">
            <BrandMark />
            <span><strong>ITEMBA-Z</strong><small>Control Center</small></span>
          </Link>
          <button className="icon-button sidebar-close" type="button" onClick={() => setNavigationOpen(false)} aria-label={t("closeNavigation")}><X size={20} /></button>
        </div>

        <nav className="sidebar-nav">
          {navigationGroups.map((group) => (
            <div className="nav-group" key={group.label}>
              <p>{t(group.label)}</p>
              {group.items.map((item) => {
                const Icon = iconMap[item.icon];
                const active = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
                return (
                  <Link key={item.href} href={item.href} className={`nav-item${active ? " nav-item-active" : ""}`} aria-current={active ? "page" : undefined} onClick={() => setNavigationOpen(false)}>
                    <Icon size={19} strokeWidth={1.8} /><span>{l(item.label)}</span>{item.badge ? <span className="nav-badge">{item.badge}</span> : null}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className="sidebar-system">
          <div className="system-icon"><Wifi size={17} /></div>
          <div><strong>{l(text("Live API guarded", "API hai inalindwa"))}</strong><span>OIDC · server BFF</span></div>
        </div>
      </aside>

      {navigationOpen ? <button className="sidebar-backdrop" onClick={() => setNavigationOpen(false)} aria-label={t("closeNavigation")} /> : null}

      <div className="shell-content" inert={navigationOpen ? true : undefined}>
        <header className="topbar">
          <div className="topbar-main">
            <button className="icon-button mobile-menu" type="button" onClick={() => setNavigationOpen(true)} aria-label={t("openNavigation")}><Menu size={21} /></button>
            <form className="global-search" action="/search" role="search">
              <Search size={18} aria-hidden="true" />
              <input ref={searchRef} type="search" name="q" minLength={2} maxLength={120} placeholder={t("searchPlaceholder")} aria-label={t("searchPlaceholder")} />
              <kbd>⌘/Ctrl K</kbd>
            </form>
            <div className="topbar-actions">
              <div className="language-toggle" role="group" aria-label={t("language")}>
                <button type="button" className={locale === "en" ? "active" : ""} aria-pressed={locale === "en"} onClick={() => setLocale("en")}>EN</button>
                <button type="button" className={locale === "sw" ? "active" : ""} aria-pressed={locale === "sw"} onClick={() => setLocale("sw")}>SW</button>
              </div>
              <details className="header-popover">
                <summary className="icon-button" aria-label={t("notifications")}><Bell size={19} />{dashboard && dashboard.pending_approvals > 0 ? <span className="notification-dot" /> : null}</summary>
                <div className="popover-panel notification-panel">
                  <div className="popover-heading"><strong>{t("notifications")}</strong><span>{dashboard?.pending_approvals ?? "—"}</span></div>
                  {notificationProblem ? <div className="notification-state" role="status"><strong>{l(text("Live notifications unavailable", "Arifa hai hazipatikani"))}</strong><small>{notificationProblem.correlation_id ?? notificationProblem.code}</small></div> : dashboard ? <Link href="/#approvals"><span className={`notice-dot ${dashboard.pending_approvals > 0 ? "warning" : "info"}`} /><span><strong>{dashboard.pending_approvals > 0 ? l(text("Transactional decisions required", "Maamuzi ya miamala yanahitajika")) : l(text("No pending transactional approvals", "Hakuna idhini za miamala zinazosubiri"))}</strong><small>{l(text(`${dashboard.pending_approvals} submitted records in your exact scope`, `Rekodi ${dashboard.pending_approvals} zilizowasilishwa katika upeo wako`))}</small></span></Link> : <div className="notification-state" role="status"><strong>{l(text("Loading live notifications…", "Inapakia arifa hai…"))}</strong></div>}
                </div>
              </details>
              <SessionControl />
            </div>
          </div>
          <LiveContextStrip />
        </header>
        <main id="main-content" className="main-content">{children}</main>
      </div>
    </div>
  );
}
