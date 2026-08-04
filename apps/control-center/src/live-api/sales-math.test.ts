import { describe, expect, it } from "vitest";
import { cartEstimatedTotalMinor, cartLineTotalMinor } from "@/live-api/sales-math";

describe("cart money arithmetic", () => {
  it("multiplies and sums safe integer minor units", () => {
    const products = new Map([
      ["water", { unit_price_minor: 12_000 }],
      ["juice", { unit_price_minor: 8_500 }],
    ]);
    expect(cartLineTotalMinor(12_000, 3)).toBe(36_000);
    expect(cartEstimatedTotalMinor([
      { productId: "water", quantity: 2 },
      { productId: "juice", quantity: 1 },
    ], products)).toBe(32_500);
  });

  it("fails closed when a line multiplication or cart addition exceeds the safe range", () => {
    expect(cartLineTotalMinor(Number.MAX_SAFE_INTEGER, 2)).toBeNull();
    expect(cartEstimatedTotalMinor([
      { productId: "first", quantity: 1 },
      { productId: "second", quantity: 1 },
    ], new Map([
      ["first", { unit_price_minor: Number.MAX_SAFE_INTEGER }],
      ["second", { unit_price_minor: 1 }],
    ]))).toBeNull();
  });
});
