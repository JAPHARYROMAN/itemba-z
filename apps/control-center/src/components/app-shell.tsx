"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";
import {
  Bell,
  Cable,
  ChartNoAxesCombined,
  ClipboardCheck,
  ClipboardList,
  Landmark,
  LayoutDashboard,
  ListChecks,
  Menu,
  PackageOpen,
  Search,
  Settings2,
  ShoppingCart,
  Smartphone,
  Truck,
  UserRoundCog,
  UsersRound,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { BrandMark } from "@/components/brand-mark";
import { useLanguage } from "@/components/language-provider";
import { LiveContextStrip } from "@/components/live-sales/live-context-strip";
import { SessionControl } from "@/components/session-control";
import { ShellContextProvider, useShellContext } from "@/components/shell/shell-context";
import {
  isNavigationItemActive,
  visibleNavigationGroups,
  type NavigationIcon,
} from "@/lib/navigation";

const iconMap: Record<NavigationIcon, LucideIcon> = {
  dashboard: LayoutDashboard,
  customers: UsersRound,
  suppliers: Truck,
  sales: ShoppingCart,
  purchases: ClipboardList,
  inventory: PackageOpen,
  finance: Landmark,
  people: UserRoundCog,
  reports: ChartNoAxesCombined,
  settings: Settings2,
  reconciliation: ListChecks,
  devices: Smartphone,
  integrations: Cable,
};

const focusableSelector = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

export function AppShell({ children }: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  if (pathname === "/login") return <>{children}</>;
  return <ShellContextProvider><AuthenticatedShell>{children}</AuthenticatedShell></ShellContextProvider>;
}

function AuthenticatedShell({ children }: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  const { locale, setLocale, t, l } = useLanguage();
  const { context, dashboard } = useShellContext();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const mainContentRef = useRef<HTMLElement>(null);
  const announcedPathRef = useRef(pathname);

  const navigation = useMemo(
    () => context.state === "ready" ? visibleNavigationGroups(context.data.permissions) : [],
    [context],
  );
  const activeItem = navigation.flatMap((group) => group.items).find((item) => isNavigationItemActive(pathname, item.href));

  const closeNavigation = useCallback((restoreFocus = true) => {
    setNavigationOpen(false);
    if (restoreFocus) window.requestAnimationFrame(() => menuButtonRef.current?.focus());
  }, []);

  useEffect(() => {
    const desktopViewport = window.matchMedia("(min-width: 1025px)");
    const closeDrawerOnDesktop = (event: MediaQueryListEvent) => {
      if (event.matches) closeNavigation(false);
    };
    desktopViewport.addEventListener("change", closeDrawerOnDesktop);
    return () => desktopViewport.removeEventListener("change", closeDrawerOnDesktop);
  }, [closeNavigation]);

  useEffect(() => {
    const focusSearch = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLocaleLowerCase() === "k") {
        event.preventDefault();
        searchRef.current?.focus();
      }
      if (event.key === "Escape" && navigationOpen) closeNavigation();
    };
    window.addEventListener("keydown", focusSearch);
    return () => window.removeEventListener("keydown", focusSearch);
  }, [closeNavigation, navigationOpen]);

  useEffect(() => {
    if (!navigationOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();
    return () => { document.body.style.overflow = previousOverflow; };
  }, [navigationOpen]);

  useEffect(() => {
    if (announcedPathRef.current === pathname) return;
    announcedPathRef.current = pathname;
    const frame = window.requestAnimationFrame(() => mainContentRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
  }, [pathname]);

  function trapDrawerFocus(event: ReactKeyboardEvent<HTMLElement>) {
    if (!navigationOpen || event.key !== "Tab") return;
    const elements = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(focusableSelector));
    const first = elements.at(0);
    const last = elements.at(-1);
    if (!first || !last) return;
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  const pendingApprovals = dashboard.state === "ready" ? dashboard.data.pending_approvals : null;

  return (
    <div className="calm-shell">
      <a className="skip-link" href="#main-content">{t("skipToContent")}</a>
      <aside
        id="primary-navigation"
        className={`calm-sidebar${navigationOpen ? " calm-sidebar-open" : ""}`}
        onKeyDown={trapDrawerFocus}
      >
        <div className="calm-sidebar-brand">
          <Link href="/" className="brand-link calm-brand-link" onClick={() => closeNavigation(false)} aria-label="ITEMBA-Z home">
            <BrandMark />
            <span><strong>ITEMBA-Z</strong><small>Control Center</small></span>
          </Link>
          <button ref={closeButtonRef} className="calm-icon-button calm-sidebar-close" type="button" onClick={() => closeNavigation()} aria-label={t("closeNavigation")}>
            <X size={21} aria-hidden="true" />
          </button>
        </div>

        <nav className="calm-sidebar-nav" aria-label={t("primaryNavigation")}>
          {context.state === "loading" ? (
            <div className="calm-nav-loading" role="status" aria-live="polite" aria-label={t("navigationLoading")}>
              {Array.from({ length: 7 }, (_, index) => <span key={index} />)}
            </div>
          ) : context.state === "ready" ? navigation.map((group) => (
            <div className="calm-nav-group" key={group.label}>
              <p>{t(group.label)}</p>
              {group.items.map((item) => {
                const Icon = iconMap[item.icon];
                const active = isNavigationItemActive(pathname, item.href);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`calm-nav-item${active ? " calm-nav-item-active" : ""}`}
                    aria-current={active ? "page" : undefined}
                    onClick={() => closeNavigation(false)}
                  >
                    <Icon size={19} strokeWidth={1.8} aria-hidden="true" />
                    <span>{l(item.label)}</span>
                  </Link>
                );
              })}
            </div>
          )) : (
            <p className="calm-nav-unavailable" role="status">{t("navigationUnavailable")}</p>
          )}
        </nav>

        <div className="calm-sidebar-scope">
          <span aria-hidden="true" />
          <div>
            <strong>{context.state === "ready" ? context.data.company_name : "ITEMBA-Z"}</strong>
            <small>{context.state === "ready" ? context.data.branch_name : t("contextUnavailable")}</small>
          </div>
        </div>
      </aside>

      {navigationOpen ? <button className="calm-sidebar-backdrop" type="button" onClick={() => closeNavigation()} aria-label={t("closeNavigation")} /> : null}

      <div className="calm-shell-content" inert={navigationOpen ? true : undefined}>
        <header className="calm-topbar">
          <div className="calm-topbar-main">
            <button
              ref={menuButtonRef}
              className="calm-icon-button calm-mobile-menu"
              type="button"
              aria-label={t("openNavigation")}
              aria-controls="primary-navigation"
              aria-expanded={navigationOpen}
              onClick={() => setNavigationOpen(true)}
            >
              <Menu size={21} aria-hidden="true" />
            </button>

            <form className="calm-global-search" action="/search" role="search">
              <Search size={18} aria-hidden="true" />
              <input ref={searchRef} type="search" name="q" minLength={2} maxLength={120} placeholder={t("searchPlaceholder")} aria-label={t("searchPlaceholder")} />
              <kbd>Ctrl K</kbd>
            </form>

            <div className="calm-topbar-actions">
              {pendingApprovals !== null ? (
                <Link className="calm-approval-link" href="/#approvals" aria-label={`${t("viewApprovals")}: ${pendingApprovals}`}>
                  <ClipboardCheck size={18} aria-hidden="true" />
                  <span>{t("approvals")}</span>
                  <strong>{pendingApprovals}</strong>
                </Link>
              ) : null}

              <details className="header-popover calm-notifications">
                <summary className="calm-icon-button" aria-label={t("notifications")}>
                  <Bell size={19} aria-hidden="true" />
                  {pendingApprovals && pendingApprovals > 0 ? <span className="notification-dot" /> : null}
                </summary>
                <div className="popover-panel notification-panel">
                  <div className="popover-heading"><strong>{t("notifications")}</strong>{pendingApprovals !== null ? <span>{pendingApprovals}</span> : null}</div>
                  <div className="notification-state" role="status">
                    <strong>
                      {dashboard.state === "ready"
                        ? pendingApprovals && pendingApprovals > 0 ? t("viewApprovals") : t("noPendingApprovals")
                        : dashboard.state === "loading" ? t("approvalsLoading") : t("approvalsUnavailable")}
                    </strong>
                    {dashboard.state === "unavailable" ? <small>{t("contextUnavailableHint")}</small> : null}
                  </div>
                </div>
              </details>

              <div className="language-toggle calm-language-toggle" role="group" aria-label={t("language")}>
                <button type="button" className={locale === "en" ? "active" : ""} aria-pressed={locale === "en"} onClick={() => setLocale("en")}>EN</button>
                <button type="button" className={locale === "sw" ? "active" : ""} aria-pressed={locale === "sw"} onClick={() => setLocale("sw")}>SW</button>
              </div>
              <SessionControl />
            </div>
          </div>
          <LiveContextStrip />
        </header>

        <p className="sr-only" role="status" aria-live="polite">{activeItem ? l(activeItem.label) : "ITEMBA-Z"}</p>
        <main ref={mainContentRef} id="main-content" className="calm-main-content" tabIndex={-1}>{children}</main>
      </div>
    </div>
  );
}
