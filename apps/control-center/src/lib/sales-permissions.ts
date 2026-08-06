export type SalesRoute = "overview" | "transactions" | "documents" | "payments" | "returns" | "new";

export const SALES_ROUTE_HREFS: Record<SalesRoute, string> = {
  overview: "/sales",
  transactions: "/sales/transactions",
  documents: "/sales/documents",
  payments: "/sales/payments",
  returns: "/sales/returns",
  new: "/sales/new",
};

const SALES_ROUTE_PERMISSIONS: Record<SalesRoute, readonly string[]> = {
  overview: ["sales.read", "customers.read", "products.read"],
  transactions: ["sales.read", "customers.read"],
  documents: ["operations.read", "customers.read", "products.read"],
  payments: ["customers.collections.post", "customers.accounts.read", "customers.read"],
  returns: ["sales.reverse", "sales.read", "customers.read"],
  new: ["sales.complete", "customers.read", "products.read"],
};

export function canAccessSalesRoute(permissions: readonly string[], route: SalesRoute): boolean {
  const granted = new Set(permissions);
  return SALES_ROUTE_PERMISSIONS[route].every((permission) => granted.has(permission));
}

export function salesLandingHref(permissions: readonly string[]): string | null {
  const routeOrder: readonly SalesRoute[] = ["overview", "documents", "payments", "returns", "new", "transactions"];
  const route = routeOrder.find((candidate) => canAccessSalesRoute(permissions, candidate));
  return route ? SALES_ROUTE_HREFS[route] : null;
}
