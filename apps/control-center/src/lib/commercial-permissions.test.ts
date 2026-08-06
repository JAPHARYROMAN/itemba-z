import { describe, expect, it } from "vitest";
import { canReadSupplierApprovals } from "@/lib/commercial-permissions";

describe("commercial permissions", () => {
  it("requires both master-data and sourcing reads for the combined approval projection", () => {
    expect(canReadSupplierApprovals(["masterdata.read"])).toBe(false);
    expect(canReadSupplierApprovals(["purchases.sourcing.read"])).toBe(false);
    expect(canReadSupplierApprovals(["masterdata.read", "purchases.sourcing.read"])).toBe(true);
  });
});
