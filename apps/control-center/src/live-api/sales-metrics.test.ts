import { describe, expect, it } from "vitest";
import { grossOriginalSaleMinor } from "@/live-api/sales-metrics";

describe("live sales metrics", () => {
  it("counts original sale documents without double-counting linked reversals", () => {
    expect(grossOriginalSaleMinor([
      { record_type: "SALE", total_minor: 12_000 },
      { record_type: "REVERSAL", total_minor: 12_000 },
      { record_type: "SALE", total_minor: 8_000 },
    ])).toBe(20_000);
  });

  it("fails closed when the page total exceeds the safe integer range", () => {
    expect(grossOriginalSaleMinor([
      { record_type: "SALE", total_minor: Number.MAX_SAFE_INTEGER },
      { record_type: "SALE", total_minor: 1 },
    ])).toBeNull();
  });
});
