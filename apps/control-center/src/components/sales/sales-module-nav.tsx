"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useLanguage } from "@/components/language-provider";
import { salesCopy, type SalesCopyKey } from "@/lib/sales-copy";
import { canAccessSalesRoute, SALES_ROUTE_HREFS, type SalesRoute } from "@/lib/sales-permissions";
import styles from "./sales-home.module.css";

interface SalesNavigationItem {
  href: string;
  label: SalesCopyKey;
  route: SalesRoute;
  activePath: string;
}

const items: SalesNavigationItem[] = [
  { href: SALES_ROUTE_HREFS.overview, label: "overview", route: "overview", activePath: SALES_ROUTE_HREFS.overview },
  { href: SALES_ROUTE_HREFS.transactions, label: "transactions", route: "transactions", activePath: SALES_ROUTE_HREFS.transactions },
  { href: SALES_ROUTE_HREFS.documents, label: "quotesOrders", route: "documents", activePath: SALES_ROUTE_HREFS.documents },
  { href: SALES_ROUTE_HREFS.payments, label: "payments", route: "payments", activePath: SALES_ROUTE_HREFS.payments },
  { href: SALES_ROUTE_HREFS.returns, label: "returns", route: "returns", activePath: SALES_ROUTE_HREFS.returns },
];

export function visibleSalesNavigation(permissions: readonly string[]): SalesNavigationItem[] {
  return items.filter((item) => canAccessSalesRoute(permissions, item.route));
}

export function SalesModuleNav({ permissions }: { permissions: readonly string[] }) {
  const pathname = usePathname();
  const { locale } = useLanguage();
  const navigation = visibleSalesNavigation(permissions);

  return (
    <nav className={styles.moduleNav} aria-label={`${salesCopy(locale, "sales")} · ${salesCopy(locale, "overview")}`}>
      {navigation.map((item) => {
        const active = item.activePath === "/sales" ? pathname === "/sales" : pathname.startsWith(item.activePath);
        return (
          <Link key={item.label} href={item.href} className={active ? styles.moduleNavActive : undefined} aria-current={active ? "page" : undefined}>
            {salesCopy(locale, item.label)}
          </Link>
        );
      })}
    </nav>
  );
}
