import { describe, expect, it } from "vitest";
import { canAccessPurchaseRoute, canManagePurchaseType, purchaseLandingHref } from "@/lib/purchase-permissions";

describe("purchase permissions", () => {
  it("requires every route capability and chooses the first permitted landing page", () => {
    expect(canAccessPurchaseRoute(["operations.read", "products.read"], "requests")).toBe(false);
    expect(canAccessPurchaseRoute(["operations.read", "products.read", "purchases.requests.manage"], "requests")).toBe(true);
    expect(purchaseLandingHref(["operations.read"])).toBe("/purchases");
    expect(purchaseLandingHref(["masterdata.read", "purchases.sourcing.read"])).toBe("/purchases/sourcing");
    expect(purchaseLandingHref([])).toBeNull();
  });

  it("maps document management to the exact server permission", () => {
    expect(canManagePurchaseType(["purchases.payments.post"], "SUPPLIER_PAYMENT")).toBe(true);
    expect(canManagePurchaseType(["purchases.payments.post"], "SUPPLIER_INVOICE")).toBe(false);
  });
});
