import type { LocalizedText, ModuleKey } from "@/domain/erp";
import { text } from "@/lib/i18n";

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
  | "reconciliation";

export interface NavigationItem {
  href: string;
  label: LocalizedText;
  icon: NavigationIcon;
  module?: ModuleKey;
  badge?: string;
}

export const navigationGroups: Array<{
  label: "overview" | "workspace" | "governance";
  items: NavigationItem[];
}> = [
  {
    label: "overview",
    items: [{ href: "/", label: text("Dashboard", "Dashibodi"), icon: "dashboard" }],
  },
  {
    label: "workspace",
    items: [
      { href: "/customers", label: text("Customers", "Wateja"), icon: "customers", module: "customers" },
      { href: "/suppliers", label: text("Suppliers", "Wasambazaji"), icon: "suppliers", module: "suppliers" },
      { href: "/sales", label: text("Sales", "Mauzo"), icon: "sales", module: "sales" },
      { href: "/purchases", label: text("Purchases", "Manunuzi"), icon: "purchases", module: "purchases" },
      { href: "/inventory", label: text("Inventory", "Bidhaa"), icon: "inventory", module: "inventory", badge: "4" },
      { href: "/finance", label: text("Finance", "Fedha"), icon: "finance", module: "finance" },
      { href: "/human-resources", label: text("Human resources", "Rasilimali watu"), icon: "people", module: "human-resources" },
    ],
  },
  {
    label: "governance",
    items: [
      { href: "/reconciliation", label: text("Reconciliation", "Upatanisho"), icon: "reconciliation" },
      { href: "/reports", label: text("Reports", "Ripoti"), icon: "reports", module: "reports" },
      { href: "/settings", label: text("Settings", "Mipangilio"), icon: "settings", module: "settings" },
    ],
  },
];
