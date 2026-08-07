import { describe, expect, it } from "vitest";
import type { OperationDocument } from "@/live-api/types";
import { eligibleSources, nextPurchaseStatuses, purchaseStatusLabel, sourceIsRequired } from "@/lib/purchase-presentation";

function document(type: OperationDocument["type"], status: OperationDocument["status"]): OperationDocument {
  return { id: `${type}-${status}`, number: `${type}-001`, type, status, party_type: "SUPPLIER", currency: "TZS", total_minor: 1000, reason: "Controlled purchasing test", created_at: "2026-08-07T08:00:00Z", lines: [] };
}

describe("purchase presentation policy", () => {
  it("offers only authoritative source states for downstream documents", () => {
    const documents = [document("PURCHASE_ORDER", "DRAFT"), document("PURCHASE_ORDER", "APPROVED"), document("GOODS_RECEIPT", "APPROVED"), document("GOODS_RECEIPT", "POSTED")];
    expect(eligibleSources(documents, "GOODS_RECEIPT").map((item) => item.status)).toEqual(["APPROVED"]);
    expect(eligibleSources(documents, "SUPPLIER_INVOICE").map((item) => item.status)).toEqual(["POSTED"]);
    expect(sourceIsRequired("GOODS_RECEIPT")).toBe(true);
    expect(sourceIsRequired("PURCHASE_ORDER")).toBe(false);
  });

  it("keeps transitions append-only and business-readable", () => {
    expect(nextPurchaseStatuses(document("PURCHASE_ORDER", "DRAFT"))).toEqual(["SUBMITTED"]);
    expect(nextPurchaseStatuses(document("PURCHASE_ORDER", "SUBMITTED"))).toEqual(["APPROVED", "REJECTED"]);
    expect(nextPurchaseStatuses(document("GOODS_RECEIPT", "APPROVED"))).toEqual(["POSTED"]);
    expect(nextPurchaseStatuses(document("PURCHASE_ORDER", "CLOSED"))).toEqual([]);
    expect(purchaseStatusLabel("SUBMITTED", "sw-TZ")).toBe("Inasubiri idhini");
  });
});
