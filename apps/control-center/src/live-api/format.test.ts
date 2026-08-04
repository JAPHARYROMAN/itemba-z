import { describe, expect, it } from "vitest";
import { formatMinorUnits, TZS_MINOR_UNITS_PER_MAJOR } from "@/live-api/format";

describe("TZS minor-unit formatting", () => {
  it("uses exactly 100 minor units per TZS without floating-point business arithmetic", () => {
    expect(TZS_MINOR_UNITS_PER_MAJOR).toBe(BigInt(100));
    expect(formatMinorUnits(8_500_000, "TZS", "en")).toBe("TZS 85,000.00");
    expect(formatMinorUnits(101, "TZS", "en")).toBe("TZS 1.01");
    expect(formatMinorUnits(-101, "TZS", "en")).toBe("-TZS 1.01");
  });

  it("rejects unsafe, fractional, and unsupported inputs", () => {
    expect(() => formatMinorUnits(Number.MAX_SAFE_INTEGER + 1, "TZS", "en")).toThrow(/safe integer/i);
    expect(() => formatMinorUnits(100.5, "TZS", "en")).toThrow(/safe integer/i);
    expect(() => formatMinorUnits(100, "USD", "en")).toThrow(/Unsupported/);
  });
});
