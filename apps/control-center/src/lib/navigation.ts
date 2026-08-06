import type { LocalizedText, ModuleKey } from "@/domain/erp";
import { text } from "@/lib/i18n";
import { salesLandingHref } from "@/lib/sales-permissions";

export type NavigationIcon =
  | "dashboard"
  | "customers"
  | "suppliers"
  | "sales"
  | "purchases"
  | "inventory"
  | "finance"
  | "people"
  | "reports"
  | "settings"
  | "reconciliation"
  | "devices"
  | "integrations";

export interface NavigationItem {
  href: string;
  label: LocalizedText;
  icon: NavigationIcon;
  module?: ModuleKey;
  permissionRules?: string[];
}

export const navigationGroups: Array<{
  label: "overview" | "workspace" | "governance";
  items: NavigationItem[];
}> = [
  {
    label: "overview",
    items: [{ href: "/", label: text("Home", "Nyumbani"), icon: "dashboard", permissionRules: ["dashboard.read"] }],
  },
  {
    label: "workspace",
    items: [
      { href: "/customers", label: text("Customers", "Wateja"), icon: "customers", module: "customers", permissionRules: ["customers.*"] },
      { href: "/suppliers", label: text("Suppliers", "Wasambazaji"), icon: "suppliers", module: "suppliers", permissionRules: ["masterdata.*", "purchases.sourcing.*", "operations.read"] },
      { href: "/sales", label: text("Sales", "Mauzo"), icon: "sales", module: "sales", permissionRules: ["sales.*", "customers.collections.post", "operations.read"] },
      { href: "/purchases", label: text("Purchases", "Manunuzi"), icon: "purchases", module: "purchases", permissionRules: ["purchases.*", "operations.read"] },
      { href: "/inventory", label: text("Inventory", "Bidhaa"), icon: "inventory", module: "inventory", permissionRules: ["inventory.*", "products.read", "operations.read"] },
      { href: "/finance", label: text("Finance", "Fedha"), icon: "finance", module: "finance", permissionRules: ["finance.*"] },
      { href: "/human-resources", label: text("Human resources", "Rasilimali watu"), icon: "people", module: "human-resources", permissionRules: ["hr.*"] },
    ],
  },
  {
    label: "governance",
    items: [
      { href: "/reconciliation", label: text("Reconciliation", "Upatanisho"), icon: "reconciliation", permissionRules: ["mobile.reconciliation.*"] },
      { href: "/devices", label: text("POS devices", "Vifaa vya POS"), icon: "devices", permissionRules: ["mobile.devices.*"] },
      { href: "/integrations", label: text("Integrations", "Miunganisho"), icon: "integrations", permissionRules: ["integrations.*"] },
      { href: "/reports", label: text("Reports", "Ripoti"), icon: "reports", module: "reports", permissionRules: ["reports.*"] },
      { href: "/settings", label: text("Administration", "Usimamizi"), icon: "settings", module: "settings", permissionRules: ["settings.*"] },
    ],
  },
];

function matchesPermissionRule(permission: string, rule: string): boolean {
  return rule.endsWith(".*") ? permission.startsWith(rule.slice(0, -1)) : permission === rule;
}

export function canSeeNavigationItem(item: NavigationItem, permissions: readonly string[]): boolean {
  if (item.href === "/sales") return salesLandingHref(permissions) !== null;
  if (!item.permissionRules?.length) return true;
  return item.permissionRules.some((rule) => permissions.some((permission) => matchesPermissionRule(permission, rule)));
}

export function visibleNavigationGroups(permissions: readonly string[]) {
  return navigationGroups
    .map((group) => ({
      ...group,
      items: group.items.flatMap((item) => {
        if (!canSeeNavigationItem(item, permissions)) return [];
        if (item.href !== "/sales") return [item];
        const href = salesLandingHref(permissions);
        return href ? [{ ...item, href }] : [];
      }),
    }))
    .filter((group) => group.items.length > 0);
}

export function isNavigationItemActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);
}
