"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import {
  Bell, Building2, CalendarDays, ChartNoAxesCombined, ChevronDown, CircleHelp, ClipboardList,
  Landmark, LayoutDashboard, LogOut, MapPin, Menu, PackageOpen, Search, Settings2, ShieldCheck,
  ShoppingCart, Truck, UserRoundCog, UsersRound, Wifi, X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { BrandMark } from "@/components/brand-mark";
import { useLanguage } from "@/components/language-provider";
import { navigationGroups, type NavigationIcon } from "@/lib/navigation";

const iconMap: Record<NavigationIcon, LucideIcon> = {
  dashboard: LayoutDashboard, customers: UsersRound, suppliers: Truck, sales: ShoppingCart,
  purchases: ClipboardList, inventory: PackageOpen, finance: Landmark, people: UserRoundCog,
  reports: ChartNoAxesCombined, settings: Settings2,
};

export function AppShell({ children }: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  const { locale, setLocale, t, l } = useLanguage();
  const [navigationOpen, setNavigationOpen] = useState(false);

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
          <div><strong>{t("connected")}</strong><span>{t("configuration")}</span></div>
        </div>
      </aside>

      {navigationOpen ? <button className="sidebar-backdrop" onClick={() => setNavigationOpen(false)} aria-label={t("closeNavigation")} /> : null}

      <div className="shell-content" inert={navigationOpen ? true : undefined}>
        <header className="topbar">
          <div className="topbar-main">
            <button className="icon-button mobile-menu" type="button" onClick={() => setNavigationOpen(true)} aria-label={t("openNavigation")}><Menu size={21} /></button>
            <form className="global-search" action="/search" role="search">
              <Search size={18} aria-hidden="true" />
              <input type="search" name="q" placeholder={t("searchPlaceholder")} aria-label={t("searchPlaceholder")} />
              <kbd>⌘ K</kbd>
            </form>
            <div className="topbar-actions">
              <div className="language-toggle" role="group" aria-label={t("language")}>
                <button type="button" className={locale === "en" ? "active" : ""} aria-pressed={locale === "en"} onClick={() => setLocale("en")}>EN</button>
                <button type="button" className={locale === "sw" ? "active" : ""} aria-pressed={locale === "sw"} onClick={() => setLocale("sw")}>SW</button>
              </div>
              <details className="header-popover">
                <summary className="icon-button" aria-label={t("notifications")}><Bell size={19} /><span className="notification-dot" /></summary>
                <div className="popover-panel notification-panel">
                  <div className="popover-heading"><strong>{t("notifications")}</strong><span>3</span></div>
                  <Link href="/purchases/PO-2026-00412"><span className="notice-dot warning" /><span><strong>{t("purchaseApproval")}</strong><small>PO-2026-00412 · TZS 14.8m</small></span></Link>
                  <Link href="/inventory/SKU-HOM-0091"><span className="notice-dot danger" /><span><strong>{t("criticalStock")}</strong><small>{t("unitsAvailable")}</small></span></Link>
                  <Link href="/finance"><span className="notice-dot info" /><span><strong>{t("bankReconciliation")}</strong><small>{t("unmatchedItems")}</small></span></Link>
                </div>
              </details>
              <details className="header-popover profile-popover">
                <summary className="profile-summary"><span className="avatar">AM</span><span className="profile-copy"><strong>Amina Msuya</strong><small>Finance Manager</small></span><ChevronDown size={15} /></summary>
                <div className="popover-panel profile-panel">
                  <p><span>{t("signedInAs")}</span><strong>amina.msuya@itemba.co.tz</strong></p>
                  <button type="button"><CircleHelp size={17} />{t("help")}</button>
                  <button type="button"><ShieldCheck size={17} />{t("securityAccess")}</button>
                  <button type="button"><LogOut size={17} />{t("signOut")}</button>
                </div>
              </details>
            </div>
          </div>
          <div className="context-bar" aria-label={t("activeContext")}>
            <div><Building2 size={15} /><span><small>{t("company")}</small><strong>Itemba Trading Co. Ltd</strong></span></div>
            <div><MapPin size={15} /><span><small>{t("branch")}</small><strong>Dar es Salaam HQ</strong></span></div>
            <div><CalendarDays size={15} /><span><small>{t("period")}</small><strong>Aug 2026 <em>{t("open")}</em></strong></span></div>
          </div>
        </header>
        <main id="main-content" className="main-content">{children}</main>
      </div>
    </div>
  );
}
