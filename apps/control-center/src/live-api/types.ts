import type { components } from "@/generated/itemba-z.v1";

export type WorkingContext = components["schemas"]["WorkingContext"];
export type CustomerSummary = components["schemas"]["CustomerSummary"];
export type CustomerPage = components["schemas"]["CustomerPage"];
export type CustomerAccountDetail = components["schemas"]["CustomerAccountDetail"];
export type CreditPolicy = components["schemas"]["CreditPolicy"];
export type ScheduleCreditPolicyCommand = components["schemas"]["ScheduleCreditPolicyCommand"];
export type ProductSummary = components["schemas"]["ProductSummary"];
export type ProductPage = components["schemas"]["ProductPage"];
export type Sale = components["schemas"]["Sale"];
export type SalePage = components["schemas"]["SalePage"];
export type PaymentMethod = components["schemas"]["PaymentMethod"];
export type CompleteSaleCommand = components["schemas"]["CompleteSaleCommand"];
export type ReverseSaleCommand = components["schemas"]["ReverseSaleCommand"];
export type MobileReconciliationCase = components["schemas"]["MobileReconciliationCase"];
export type MobileReconciliationPage = components["schemas"]["MobileReconciliationPage"];
export type MobileReconciliationStatus = components["schemas"]["MobileReconciliationStatus"];
export type ResolveMobileReconciliationCommand = components["schemas"]["ResolveMobileReconciliationCommand"];
export type MobileDevice = components["schemas"]["MobileDeviceEnrollment"];
export type MobileDevicePage = components["schemas"]["MobileDevicePage"];
export type ChangeMobileDeviceStatusCommand = components["schemas"]["ChangeMobileDeviceStatusCommand"];
export type ChangeMobileDeviceAllocationCommand = components["schemas"]["ChangeMobileDeviceAllocationCommand"];

export interface PublicProblem {
  type: string;
  title: string;
  status: number;
  code: string;
  detail: string;
  correlation_id?: string;
}

export type LiveSnapshot<T> =
  | { state: "ready"; data: T }
  | { state: "unavailable"; problem: PublicProblem };

export interface SalesBootstrap {
  context: WorkingContext;
  customers: CustomerSummary[];
  products: ProductSummary[];
}

export interface SalesWorkspace extends SalesBootstrap {
  sales: Sale[];
  nextCursor: string | null;
}

export interface SaleDetailWorkspace extends SalesBootstrap {
  sale: Sale;
}

export interface ReconciliationWorkspace {
  context: WorkingContext;
  cases: MobileReconciliationCase[];
  nextCursor: string | null;
  status: MobileReconciliationStatus | "";
}

export interface ReconciliationDetailWorkspace {
  context: WorkingContext;
  reconciliationCase: MobileReconciliationCase;
}

export interface DeviceManagementWorkspace {
  context: WorkingContext;
  devices: MobileDevice[];
  products: ProductSummary[];
  nextCursor: string | null;
}

export interface CustomerAccountsWorkspace {
  context: WorkingContext;
  customers: CustomerSummary[];
}

export interface CustomerAccountWorkspace {
  context: WorkingContext;
  account: CustomerAccountDetail;
}

export type OperationDocumentType = "QUOTATION" | "SALES_ORDER" | "PURCHASE_REQUEST" | "PURCHASE_ORDER" | "GOODS_RECEIPT" | "SUPPLIER_INVOICE" | "SUPPLIER_PAYMENT" | "PURCHASE_RETURN" | "STOCK_TRANSFER" | "STOCK_COUNT" | "STOCK_ADJUSTMENT";
export type OperationStatus = "DRAFT" | "SUBMITTED" | "APPROVED" | "REJECTED" | "POSTED" | "DISPATCHED" | "RECEIVED" | "CLOSED" | "REVERSED";
export interface OperationLine { id: string; product_id: string; quantity: number; unit_price_minor: number; amount_minor: number }
export interface OperationDocument { id: string; number: string; type: OperationDocumentType; status: OperationStatus; party_type: "CUSTOMER" | "SUPPLIER" | "NONE"; party_id?: string; source_document_id?: string; destination_warehouse_id?: string; currency: string; total_minor: number; reason: string; created_at: string; lines: OperationLine[] }
export interface OperationPage { items: OperationDocument[]; next_cursor: string | null }
export interface CreateOperationCommand { type: OperationDocumentType; party_type: "CUSTOMER" | "SUPPLIER" | "NONE"; party_id?: string; source_document_id?: string; destination_warehouse_id?: string; currency: string; reason: string; lines: Array<{ product_id: string; quantity: number; unit_price_minor: number }> }
export interface TransitionOperationCommand { status: OperationStatus; reason: string; payment_method?: string }
export interface SupplierSummary { id: string; code: string; name: string; active: boolean; payment_terms_days: number }
export interface SupplierPage { items: SupplierSummary[]; next_cursor: string | null }
export interface OperationsWorkspace { context: WorkingContext; customers: CustomerSummary[]; products: ProductSummary[]; suppliers: SupplierSummary[]; documents: OperationDocument[]; nextCursor: string | null }
export interface ReceiveCustomerCollectionCommand { invoice_sale_id: string; method: PaymentMethod; amount_minor: number; currency: string }
export interface CustomerCollection { id: string; customer_id: string; invoice_sale_id: string; method: PaymentMethod; account_id: string; amount_minor: number; currency: string; occurred_at: string; correlation_id: string }
