import { describe, expect, it } from "vitest";
import { firstUnsafeIntegerPath, safeIntegerProduct, safeIntegerSum } from "@/live-api/integer-safety";

describe("safe integer utilities", () => {
  it("finds nested unsafe and fractional JSON numbers", () => {
    expect(firstUnsafeIntegerPath({ items: [{ quantity: 1 }, { quantity: Number.MAX_SAFE_INTEGER + 1 }] })).toBe("$.items[1].quantity");
    expect(firstUnsafeIntegerPath({ tax_basis_points: 18.5 })).toBe("$.tax_basis_points");
    expect(firstUnsafeIntegerPath({ amount: Number.MAX_SAFE_INTEGER })).toBeNull();
  });

  it("performs addition and multiplication through exact BigInt intermediates", () => {
    expect(safeIntegerSum([8_500_000, 1_500_000])).toBe(10_000_000);
    expect(safeIntegerSum([Number.MAX_SAFE_INTEGER, 1])).toBeNull();
    expect(safeIntegerProduct(12_000, 3)).toBe(36_000);
    expect(safeIntegerProduct(Number.MAX_SAFE_INTEGER, 2)).toBeNull();
  });
});
