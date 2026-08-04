import { describe, expect, it } from "vitest";
import { getModule, getRecord, isModuleKey, moduleKeys, searchRecords } from "@/data/erp-repository";

describe("ERP repository", () => {
  it("exposes every release-one module through the typed boundary", async () => {
    expect(moduleKeys).toHaveLength(9);
    expect(isModuleKey("human-resources")).toBe(true);
    expect(isModuleKey("manufacturing")).toBe(false);
    const sales = await getModule("sales");
    expect(sales.records).toHaveLength(3);
    expect(sales.records[0]?.id).toBe("SO-2026-01428");
  });

  it("finds records by business identifiers and bilingual copy", async () => {
    expect((await searchRecords("Kijiji"))[0]?.href).toContain("CUST-00184");
    expect((await searchRecords("Maji Safi"))[0]?.id).toBe("SKU-BEV-0018");
    expect(await getRecord("finance", "JV-2026-00861")).toMatchObject({ id: "JV-2026-00861" });
  });
});
