import type { OperationDocumentType } from "@/live-api/types";

export type PurchaseRoute = "overview" | "requests" | "sourcing" | "orders" | "receipts" | "bills" | "payments" | "returns";

export const PURCHASE_ROUTE_HREFS: Record<PurchaseRoute, string> = {
  overview: "/purchases",
  requests: "/purchases/requests",
  sourcing: "/purchases/sourcing",
  orders: "/purchases/orders",
  receipts: "/purchases/receipts",
  bills: "/purchases/bills",
  payments: "/purchases/payments",
  returns: "/purchases/returns",
};

const ROUTE_PERMISSIONS: Record<PurchaseRoute, readonly string[]> = {
  overview: ["operations.read"],
  requests: ["operations.read", "products.read", "purchases.requests.manage"],
  sourcing: ["masterdata.read", "purchases.sourcing.read"],
  orders: ["operations.read", "products.read", "purchases.orders.manage"],
  receipts: ["operations.read", "products.read", "purchases.receive"],
  bills: ["operations.read", "products.read", "purchases.invoices.post"],
  payments: ["operations.read", "products.read", "purchases.payments.post"],
  returns: ["operations.read", "products.read", "purchases.receive"],
};

const TYPE_PERMISSIONS: Record<OperationDocumentType, string> = {
  QUOTATION: "sales.orders.manage",
  SALES_ORDER: "sales.orders.manage",
  PURCHASE_REQUEST: "purchases.requests.manage",
  PURCHASE_ORDER: "purchases.orders.manage",
  GOODS_RECEIPT: "purchases.receive",
  SUPPLIER_INVOICE: "purchases.invoices.post",
  SUPPLIER_PAYMENT: "purchases.payments.post",
  PURCHASE_RETURN: "purchases.receive",
  STOCK_TRANSFER: "inventory.transfers.manage",
  STOCK_COUNT: "inventory.counts.manage",
  STOCK_ADJUSTMENT: "inventory.adjustments.post",
};

export function canAccessPurchaseRoute(permissions: readonly string[], route: PurchaseRoute): boolean {
  const granted = new Set(permissions);
  return ROUTE_PERMISSIONS[route].every((permission) => granted.has(permission));
}

export function canManagePurchaseType(permissions: readonly string[], type: OperationDocumentType): boolean {
  return permissions.includes(TYPE_PERMISSIONS[type]);
}

export function purchaseLandingHref(permissions: readonly string[]): string | null {
  const route = (["overview", "requests", "orders", "receipts", "bills", "payments", "returns", "sourcing"] as const)
    .find((candidate) => canAccessPurchaseRoute(permissions, candidate));
  return route ? PURCHASE_ROUTE_HREFS[route] : null;
}
