import type { Sale } from "@/live-api/types";
import type { SalesCopyKey } from "@/lib/sales-copy";

const uuidReference = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export type SaleStatusPresentation = {
  label: SalesCopyKey;
  tone: "success" | "warning" | "danger";
};

export function saleStatusPresentation(sale: Sale): SaleStatusPresentation {
  if (sale.record_type === "REVERSAL" || sale.status === "REVERSED") {
    return { label: "reversed", tone: "warning" };
  }
  if (sale.fiscal_status === "FAILED") return { label: "fiscalFailed", tone: "danger" };
  if (sale.fiscal_status === "PENDING") return { label: "fiscalPending", tone: "warning" };
  if (sale.fiscal_status === "NOT_CONFIGURED") return { label: "fiscalNotConfigured", tone: "warning" };
  return { label: "posted", tone: "success" };
}

export function saleNeedsAttention(sale: Sale): boolean {
  return sale.record_type === "REVERSAL"
    || sale.status === "REVERSED"
    || sale.fiscal_status !== "FISCALIZED";
}

export function saleBusinessReference(sale: Sale): string {
  const reference = sale.receipt_reference.trim();
  if (!uuidReference.test(reference)) return reference;

  const token = reference.replaceAll("-", "").slice(0, 12).toUpperCase();
  return `${sale.record_type === "REVERSAL" ? "REV" : "SALE"}-${token}`;
}
