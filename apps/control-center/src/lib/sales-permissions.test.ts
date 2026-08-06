import { describe, expect, it } from "vitest";
import { canAccessSalesRoute, salesLandingHref } from "@/lib/sales-permissions";

describe("Sales route permissions", () => {
  it("requires every route dependency", () => {
    expect(canAccessSalesRoute(["customers.collections.post", "customers.read"], "payments")).toBe(false);
    expect(canAccessSalesRoute(["customers.collections.post", "customers.accounts.read", "customers.read"], "payments")).toBe(true);
  });

  it("chooses a task the role can actually load", () => {
    expect(salesLandingHref(["sales.reverse", "sales.read", "customers.read"])).toBe("/sales/returns");
    expect(salesLandingHref(["sales.complete", "customers.read", "products.read"])).toBe("/sales/new");
    expect(salesLandingHref(["sales.read"])).toBeNull();
  });
});
