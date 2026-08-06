"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useLanguage } from "@/components/language-provider";
import { commercialCopy, type CommercialCopyKey } from "@/lib/commercial-copy";
import { canReadSupplierApprovals, hasEveryPermission } from "@/lib/commercial-permissions";
import styles from "./commercial.module.css";

type Item = { href: string; label: CommercialCopyKey; visible: (permissions: readonly string[]) => boolean };

const customerItems: Item[] = [
  { href: "/customers", label: "customerDirectory", visible: (permissions) => permissions.includes("customers.read") || permissions.includes("customers.accounts.read") },
  { href: "/sales/payments", label: "collections", visible: (permissions) => hasEveryPermission(permissions, ["customers.collections.post", "customers.accounts.read", "customers.read"]) },
];

const supplierItems: Item[] = [
  { href: "/suppliers", label: "supplierDirectory", visible: (permissions) => hasEveryPermission(permissions, ["operations.read"]) },
  { href: "/suppliers#approvals", label: "approvals", visible: canReadSupplierApprovals },
  { href: "/purchases", label: "purchasing", visible: (permissions) => hasEveryPermission(permissions, ["operations.read"]) },
];

export function CommercialModuleNav({ area, permissions }: { area: "customers" | "suppliers"; permissions: readonly string[] }) {
  const pathname = usePathname();
  const { locale } = useLanguage();
  const items = area === "customers" ? customerItems : supplierItems;
  return <nav className={styles.moduleNav} aria-label={commercialCopy(locale, area)}>
    {items.filter((item) => item.visible(permissions)).map((item) => {
      const active = item.href === `/${area}` && pathname === item.href;
      return <Link key={`${item.href}-${item.label}`} href={item.href} className={active ? styles.moduleNavActive : undefined} aria-current={active ? "page" : undefined}>{commercialCopy(locale, item.label)}</Link>;
    })}
  </nav>;
}
