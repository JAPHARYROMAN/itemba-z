import type { OperationDocument, OperationDocumentType, OperationStatus } from "@/live-api/types";
import type { PurchaseRoute } from "@/lib/purchase-permissions";

export const PURCHASE_TYPE_BY_ROUTE: Record<Exclude<PurchaseRoute, "overview" | "sourcing">, OperationDocumentType> = {
  requests: "PURCHASE_REQUEST",
  orders: "PURCHASE_ORDER",
  receipts: "GOODS_RECEIPT",
  bills: "SUPPLIER_INVOICE",
  payments: "SUPPLIER_PAYMENT",
  returns: "PURCHASE_RETURN",
};

export const PURCHASE_ROUTE_BY_TYPE: Partial<Record<OperationDocumentType, PurchaseRoute>> = {
  PURCHASE_REQUEST: "requests",
  PURCHASE_ORDER: "orders",
  GOODS_RECEIPT: "receipts",
  SUPPLIER_INVOICE: "bills",
  SUPPLIER_PAYMENT: "payments",
  PURCHASE_RETURN: "returns",
};

const SOURCE_TYPES: Partial<Record<OperationDocumentType, OperationDocumentType>> = {
  PURCHASE_ORDER: "PURCHASE_REQUEST",
  GOODS_RECEIPT: "PURCHASE_ORDER",
  SUPPLIER_INVOICE: "GOODS_RECEIPT",
  SUPPLIER_PAYMENT: "SUPPLIER_INVOICE",
  PURCHASE_RETURN: "SUPPLIER_INVOICE",
};

export function sourceTypeFor(type: OperationDocumentType): OperationDocumentType | null {
  return SOURCE_TYPES[type] ?? null;
}

export function sourceIsRequired(type: OperationDocumentType): boolean {
  return ["GOODS_RECEIPT", "SUPPLIER_INVOICE", "SUPPLIER_PAYMENT", "PURCHASE_RETURN"].includes(type);
}

export function eligibleSources(documents: readonly OperationDocument[], type: OperationDocumentType): OperationDocument[] {
  const sourceType = sourceTypeFor(type);
  if (!sourceType) return [];
  return documents.filter((document) => document.type === sourceType && (
    sourceType === "GOODS_RECEIPT" || sourceType === "SUPPLIER_INVOICE"
      ? document.status === "POSTED"
      : document.status === "APPROVED" || document.status === "CLOSED"
  ));
}

export function nextPurchaseStatuses(document: OperationDocument): OperationStatus[] {
  if (document.status === "DRAFT") return ["SUBMITTED"];
  if (document.status === "SUBMITTED") return ["APPROVED", "REJECTED"];
  if (document.status === "APPROVED") {
    if (["PURCHASE_REQUEST", "PURCHASE_ORDER"].includes(document.type)) return ["CLOSED"];
    return ["POSTED"];
  }
  return [];
}

export function purchaseStatusLabel(status: OperationStatus, locale: string): string {
  const sw = locale.startsWith("sw");
  const labels: Record<OperationStatus, [string, string]> = {
    DRAFT: ["Draft", "Rasimu"], SUBMITTED: ["Awaiting approval", "Inasubiri idhini"], APPROVED: ["Approved", "Imeidhinishwa"],
    REJECTED: ["Not approved", "Haijaidhinishwa"], POSTED: ["Posted", "Imechapishwa"], DISPATCHED: ["Dispatched", "Imetumwa"],
    RECEIVED: ["Received", "Imepokelewa"], CLOSED: ["Closed", "Imefungwa"], REVERSED: ["Reversed", "Imebatilishwa"],
  };
  return labels[status][sw ? 1 : 0];
}

export function purchaseTypeLabel(type: OperationDocumentType, locale: string): string {
  const sw = locale.startsWith("sw");
  const labels: Partial<Record<OperationDocumentType, [string, string]>> = {
    PURCHASE_REQUEST: ["Purchase request", "Ombi la ununuzi"], PURCHASE_ORDER: ["Purchase order", "Oda ya ununuzi"],
    GOODS_RECEIPT: ["Goods receipt", "Mapokezi ya bidhaa"], SUPPLIER_INVOICE: ["Supplier bill", "Ankara ya msambazaji"],
    SUPPLIER_PAYMENT: ["Supplier payment", "Malipo ya msambazaji"], PURCHASE_RETURN: ["Purchase return", "Marejesho ya ununuzi"],
  };
  return (labels[type] ?? [type, type])[sw ? 1 : 0];
}

export function purchaseStatusTone(status: OperationStatus): "success" | "warning" | "danger" | "neutral" {
  if (["POSTED", "CLOSED", "RECEIVED"].includes(status)) return "success";
  if (["SUBMITTED", "APPROVED", "DISPATCHED"].includes(status)) return "warning";
  if (["REJECTED", "REVERSED"].includes(status)) return "danger";
  return "neutral";
}
