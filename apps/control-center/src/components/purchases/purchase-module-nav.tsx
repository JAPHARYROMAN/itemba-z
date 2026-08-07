"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useLanguage } from "@/components/language-provider";
import { purchaseCopy } from "@/lib/purchase-copy";
import { canAccessPurchaseRoute, PURCHASE_ROUTE_HREFS, type PurchaseRoute } from "@/lib/purchase-permissions";
import styles from "./purchases.module.css";

const routes: PurchaseRoute[] = ["overview", "requests", "sourcing", "orders", "receipts", "bills", "payments", "returns"];

export function PurchaseModuleNav({ permissions }: { permissions: readonly string[] }) {
  const pathname = usePathname();
  const { locale } = useLanguage();
  return <nav className={styles.moduleNav} aria-label={purchaseCopy(locale, "purchases")}>
    {routes.filter((route) => canAccessPurchaseRoute(permissions, route)).map((route) => {
      const href = PURCHASE_ROUTE_HREFS[route];
      const active = route === "overview" ? pathname === href : pathname.startsWith(href);
      return <Link key={route} href={href} className={active ? styles.moduleNavActive : undefined} aria-current={active ? "page" : undefined}>{purchaseCopy(locale, route)}</Link>;
    })}
  </nav>;
}
