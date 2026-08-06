import { describe, expect, it } from "vitest";
import type { Sale } from "@/live-api/types";
import { saleBusinessReference, saleNeedsAttention, saleStatusPresentation } from "@/lib/sales-presentation";

const baseSale = {
  id: "4afac72c-210e-4cb4-abb6-f62c19800ff9",
  receipt_reference: "4afac72c-210e-4cb4-abb6-f62c19800ff9",
  record_type: "SALE",
  status: "POSTED",
  fiscal_status: "FISCALIZED",
} as Sale;

describe("sales presentation", () => {
  it("turns a UUID receipt reference into the existing compact document-number convention", () => {
    expect(saleBusinessReference(baseSale)).toBe("SALE-4AFAC72C210E");
    expect(saleBusinessReference({ ...baseSale, receipt_reference: "RCP-000123" })).toBe("RCP-000123");
  });

  it("keeps every unresolved fiscal state visible as an exception", () => {
    const sale = { ...baseSale, fiscal_status: "NOT_CONFIGURED" } as Sale;
    expect(saleNeedsAttention(sale)).toBe(true);
    expect(saleStatusPresentation(sale)).toEqual({ label: "fiscalNotConfigured", tone: "warning" });
  });
});
