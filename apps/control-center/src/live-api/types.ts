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
