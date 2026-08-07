import { describe, expect, it } from "vitest";
import { isNavigationItemActive, visibleNavigationGroups } from "@/lib/navigation";

const hrefs = (permissions: string[]) => visibleNavigationGroups(permissions).flatMap((group) => group.items.map((item) => item.href));

describe("permission-shaped navigation", () => {
  it("shows only workspaces supported by exact server permissions", () => {
    expect(hrefs(["sales.read", "customers.read", "customers.collections.post", "products.read"])).toEqual([
      "/customers",
      "/sales",
      "/inventory",
    ]);
  });

  it("lands a least-privilege Sales role on its first fully authorized task", () => {
    expect(hrefs(["customers.read", "customers.accounts.read", "customers.collections.post"])).toContain("/sales/payments");
    expect(hrefs(["sales.reverse", "sales.read", "customers.read"])).toContain("/sales/returns");
    expect(hrefs(["sales.orders.manage", "operations.read", "customers.read", "products.read"])).toContain("/sales/documents");
  });

  it("recognizes module permission families without matching false prefixes", () => {
    expect(hrefs(["finance.bank.read"])).toEqual(["/finance"]);
    expect(hrefs(["financex.bank.read"])).toEqual([]);
    expect(hrefs(["mobile.devices.manage"])).toEqual(["/devices"]);
  });

  it("removes empty groups and never exposes the retired static inventory badge", () => {
    const groups = visibleNavigationGroups(["settings.read"]);
    expect(groups).toHaveLength(1);
    expect(groups[0]?.items).toHaveLength(1);
    expect(groups[0]?.items[0]).not.toHaveProperty("badge");
  });

  it("matches nested route boundaries but rejects similar prefixes", () => {
    expect(isNavigationItemActive("/sales/transactions", "/sales")).toBe(true);
    expect(isNavigationItemActive("/salesforce", "/sales")).toBe(false);
    expect(isNavigationItemActive("/", "/")).toBe(true);
    expect(isNavigationItemActive("/sales", "/")).toBe(false);
  });
});
