import { describe, expect, it, vi } from "vitest";
import { listAllOperationDocuments } from "@/live-api/operation-pagination";
import type { OperationDocument } from "@/live-api/types";

function document(id: string): OperationDocument {
  return { id, number: id, type: "SALES_ORDER", status: "APPROVED", party_type: "CUSTOMER", currency: "TZS", total_minor: 100, reason: "Approved order", created_at: "2026-08-06T00:00:00Z", lines: [] };
}

describe("operation document pagination", () => {
  it("loads every page and removes repeated boundary records", async () => {
    const read = vi.fn()
      .mockResolvedValueOnce({ items: [document("one")], next_cursor: "next" })
      .mockResolvedValueOnce({ items: [document("one"), document("two")], next_cursor: null });
    await expect(listAllOperationDocuments({ listOperationDocuments: read }, "SALES_ORDER")).resolves.toEqual([document("one"), document("two")]);
    expect(read).toHaveBeenNthCalledWith(2, "SALES_ORDER", "next");
  });

  it("fails closed on a repeated cursor", async () => {
    const read = vi.fn().mockResolvedValue({ items: [], next_cursor: "same" });
    await expect(listAllOperationDocuments({ listOperationDocuments: read }, "SALES_ORDER")).rejects.toMatchObject({ problem: { code: "operation_document_pagination_invalid" } });
  });
});
