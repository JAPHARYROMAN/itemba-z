import { describe, expect, it } from "vitest";
import { minorUnitsToMajorInput, parseMajorUnitsToMinor } from "@/live-api/money-input";

describe("exact monetary input", () => {
  it.each([["0", 0], ["0.29", 29], ["1.2", 120], ["123456789.01", 12_345_678_901]])("parses %s without binary floating point", (value, expected) => {
    expect(parseMajorUnitsToMinor(value)).toBe(expected);
  });

  it.each(["", "-1", "1.005", "1e3", "01.00", "90071992547409.92"])("rejects unsafe or non-decimal input %s", (value) => {
    expect(parseMajorUnitsToMinor(value)).toBeNull();
  });

  it("formats minor units as a stable two-decimal input", () => {
    expect(minorUnitsToMajorInput(29)).toBe("0.29");
    expect(minorUnitsToMajorInput(120)).toBe("1.20");
  });
});
