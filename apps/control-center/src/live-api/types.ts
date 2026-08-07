import type { components } from "@/generated/itemba-z.v1";
import type { LocalizedText, Status } from "@/domain/erp";

export type WorkingContext = components["schemas"]["WorkingContext"];
export type AuditRecord = components["schemas"]["AuditRecord"];
export type AuditTrail = components["schemas"]["AuditTrail"];
export type Dashboard = components["schemas"]["Dashboard"];
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
export type Employee = components["schemas"]["Employee"];
export type Attendance = components["schemas"]["Attendance"];
export type LeaveType = components["schemas"]["LeaveType"];
export type LeaveRequest = components["schemas"]["LeaveRequest"];
export type EmployeeLoan = components["schemas"]["EmployeeLoan"];
export type PayrollLine = components["schemas"]["PayrollLine"];
export type PayrollRun = components["schemas"]["PayrollRun"];
export type PeopleSnapshot = components["schemas"]["PeopleSnapshot"];
export type CreateEmployeeCommand = components["schemas"]["CreateEmployeeCommand"];
export type AttendanceCommand = components["schemas"]["AttendanceCommand"];
export type LeaveTypeCommand = components["schemas"]["LeaveTypeCommand"];
export type LeaveCommand = components["schemas"]["LeaveCommand"];
export type PeopleTransitionCommand = components["schemas"]["PeopleTransitionCommand"];
export type LoanCommand = components["schemas"]["LoanCommand"];
export type PayrollCommand = components["schemas"]["PayrollCommand"];
export type WorkforceSnapshot = components["schemas"]["WorkforceSnapshot"];
export type ShiftTemplate = components["schemas"]["ShiftTemplate"];
export type ShiftAssignment = components["schemas"]["ShiftAssignment"];
export type EmployeeDocument = components["schemas"]["EmployeeDocument"];
export type PayrollArtifact = components["schemas"]["PayrollArtifact"];
export type WorkforceStatus = components["schemas"]["WorkforceStatus"];
export type CreateShiftTemplateCommand = components["schemas"]["CreateShiftTemplateCommand"];
export type CreateShiftAssignmentCommand = components["schemas"]["CreateShiftAssignmentCommand"];
export type RegisterEmployeeDocumentCommand = components["schemas"]["RegisterEmployeeDocumentCommand"];
export type GeneratePayrollArtifactCommand = components["schemas"]["GeneratePayrollArtifactCommand"];
export interface PeopleWorkspace { context: WorkingContext; people: PeopleSnapshot; workforce: WorkforceSnapshot; configurations: ConfigurationVersion[]; accounts: GLAccount[] }
export type ConfigurationVersion = components["schemas"]["ConfigurationVersion"];
export type NumberSequence = components["schemas"]["NumberSequence"];
export type NumberAllocation = components["schemas"]["NumberAllocation"];
export type ConfigurationSnapshot = components["schemas"]["ConfigurationSnapshot"];
export type CreateConfigurationCommand = components["schemas"]["CreateConfigurationCommand"];
export type ConfigurationTransitionCommand = components["schemas"]["ConfigurationTransitionCommand"];
export type CreateNumberSequenceCommand = components["schemas"]["CreateNumberSequenceCommand"];
export interface ConfigurationWorkspace { context: WorkingContext; configuration: ConfigurationSnapshot }
export type IntegrationRoute = components["schemas"]["IntegrationRoute"];
export type IntegrationDelivery = components["schemas"]["IntegrationDelivery"];
export type IntegrationAttempt = components["schemas"]["IntegrationAttempt"];
export type IntegrationSnapshot = components["schemas"]["IntegrationWorkspace"];
export type CreateIntegrationRouteCommand = components["schemas"]["CreateIntegrationRouteCommand"];
export type IntegrationRouteTransitionCommand = components["schemas"]["IntegrationRouteTransitionCommand"];
export interface IntegrationOperationsWorkspace { context: WorkingContext; integrations: IntegrationSnapshot }

export type MasterEntityType = "SUPPLIER" | "PRODUCT";
export type MasterRevisionStatus = "DRAFT" | "SUBMITTED" | "ACTIVE" | "REJECTED";
export interface SupplierMasterData { code: string; name: string; tax_id: string; email: string; phone: string; payment_terms_days: number; active: boolean }
export interface ProductMasterData { sku: string; name: string; base_unit_code: string; currency: string; list_price_minor: number; standard_cost_minor: number; tax_code: string; revenue_account_id: string; cogs_account_id: string; inventory_account_id: string; active: boolean }
export interface MasterRevision { id: string; entity_type: MasterEntityType; entity_id: string; status: MasterRevisionStatus; supplier?: SupplierMasterData; product?: ProductMasterData; reason: string; created_by: string; approved_by?: string }
export type RFQStatus = "DRAFT" | "SUBMITTED" | "APPROVED" | "CLOSED" | "CANCELLED";
export interface RFQLine { id: string; product_id: string; quantity: number }
export interface RFQ { id: string; number: string; status: RFQStatus; currency: string; response_due_at: string; reason: string; created_by: string; lines: RFQLine[] }
export type SupplierQuoteStatus = "DRAFT" | "SUBMITTED" | "SELECTED" | "REJECTED";
export interface SupplierQuoteLine { id: string; product_id: string; quantity: number; unit_price_minor: number; amount_minor: number }
export interface SupplierQuote { id: string; rfq_id: string; supplier_id: string; reference: string; status: SupplierQuoteStatus; currency: string; delivery_days: number; payment_terms_days: number; valid_until: string; total_minor: number; reason: string; lines: SupplierQuoteLine[] }
export interface SourcingAward { id: string; rfq_id: string; quote_id: string; supplier_id: string; purchase_order_id: string; reason: string; selected_by: string; selected_at: string }
export interface CommercialSnapshot { revisions: MasterRevision[]; rfqs: RFQ[]; quotes: SupplierQuote[]; awards: SourcingAward[] }
export interface CommercialWorkspace { context: WorkingContext; commercial: CommercialSnapshot; products: ProductSummary[]; suppliers: SupplierSummary[] }
export interface SupplierWorkspace { context: WorkingContext; suppliers: SupplierSummary[]; commercial: CommercialSnapshot | null }
export interface GlobalSearchResult { id: string; module: string; moduleLabel: LocalizedText; title: LocalizedText; subtitle: string; href: string; status: Status }
export interface GlobalSearchWorkspace { context: WorkingContext; query: string; results: GlobalSearchResult[]; searchedSources: LocalizedText[] }
export interface CreateMasterRevisionCommand { entity_type: MasterEntityType; entity_id?: string; supplier?: SupplierMasterData; product?: ProductMasterData; reason: string }
export interface CreateRFQCommand { currency: string; response_due_at: string; reason: string; lines: Array<{ product_id: string; quantity: number }> }
export interface CreateSupplierQuoteCommand { rfq_id: string; supplier_id: string; reference: string; currency: string; delivery_days: number; payment_terms_days: number; valid_until: string; reason: string; lines: Array<{ product_id: string; quantity: number; unit_price_minor: number }> }
export type InventoryPolicy = components["schemas"]["InventoryPolicy"];
export type InventoryPolicyStatus = components["schemas"]["InventoryPolicyStatus"];
export type InventoryControlSnapshot = components["schemas"]["InventoryControlWorkspace"];
export type CreateInventoryPolicyCommand = components["schemas"]["CreateInventoryPolicyCommand"];
export type InventoryPolicyTransitionCommand = components["schemas"]["InventoryPolicyTransitionCommand"];
export type RegisterReceiptLotsCommand = components["schemas"]["RegisterReceiptLotsCommand"];
export type LotRegistration = components["schemas"]["LotRegistration"];
export interface InventoryControlWorkspace { context: WorkingContext; inventory: InventoryControlSnapshot; products: ProductSummary[]; suppliers: SupplierSummary[]; receipts: OperationDocument[] }

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

export interface DashboardWorkspace {
  context: WorkingContext;
  dashboard: Dashboard;
}

export interface SalesBootstrap {
  context: WorkingContext;
  customers: CustomerSummary[];
  products: ProductSummary[];
  documents: OperationDocument[];
}

export interface SalesWorkspace extends SalesBootstrap {
  sales: Sale[];
  nextCursor: string | null;
}

export interface SalesRegisterWorkspace {
  context: WorkingContext;
  customers: CustomerSummary[];
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
export interface PurchaseWorkspace { context: WorkingContext; products: ProductSummary[]; suppliers: SupplierSummary[]; documents: OperationDocument[] }
export interface PurchaseDocumentWorkspace extends PurchaseWorkspace { document: OperationDocument }
export interface ReceiveCustomerCollectionCommand { invoice_sale_id: string; method: PaymentMethod; amount_minor: number; currency: string }
export interface CustomerCollection { id: string; customer_id: string; invoice_sale_id: string; method: PaymentMethod; account_id: string; amount_minor: number; currency: string; occurred_at: string; correlation_id: string }
export type BankAccount = components["schemas"]["BankAccount"];
export type BankAccountPage = components["schemas"]["BankAccountPage"];
export type BankStatement = components["schemas"]["BankStatement"];
export type BankStatementPage = components["schemas"]["BankStatementPage"];
export type ImportBankStatementCommand = components["schemas"]["ImportBankStatementCommand"];
export type MatchBankStatementLineCommand = components["schemas"]["MatchBankStatementLineCommand"];
export type ReconcileBankStatementCommand = components["schemas"]["ReconcileBankStatementCommand"];
export interface BankingWorkspace { context: WorkingContext; accounts: BankAccount[]; statements: BankStatement[]; nextCursor: string | null }
export type FinancialDocument = components["schemas"]["FinancialDocument"];
export type FinancialDocumentPage = components["schemas"]["FinancialDocumentPage"];
export type CreateFinancialDocumentCommand = components["schemas"]["CreateFinancialDocumentCommand"];
export type TransitionFinancialDocumentCommand = components["schemas"]["TransitionFinancialDocumentCommand"];
export type FiscalPeriod = components["schemas"]["FiscalPeriod"];
export type FiscalPeriodPage = components["schemas"]["FiscalPeriodPage"];
export type FiscalPeriodActionCommand = components["schemas"]["FiscalPeriodActionCommand"];
export type FiscalPeriodActionRequest = components["schemas"]["FiscalPeriodActionRequest"];
export type FiscalPeriodActionPage = components["schemas"]["FiscalPeriodActionPage"];
export type GLAccount = components["schemas"]["GLAccount"];
export type GLAccountPage = components["schemas"]["GLAccountPage"];
export type CreateGLAccountCommand = components["schemas"]["CreateGLAccountCommand"];
export type GovernanceDecisionCommand = components["schemas"]["GovernanceDecisionCommand"];
export type PostingMapping = components["schemas"]["PostingMapping"];
export type PostingMappingPage = components["schemas"]["PostingMappingPage"];
export type CreatePostingMappingCommand = components["schemas"]["CreatePostingMappingCommand"];
export type TrialBalance = components["schemas"]["TrialBalance"];
export type GeneralLedger = components["schemas"]["GeneralLedger"];
export type ProfitAndLoss = components["schemas"]["ProfitAndLoss"];
export type BalanceSheet = components["schemas"]["BalanceSheet"];
export type CashFlow = components["schemas"]["CashFlow"];
export type ExportFinancialReportCommand = components["schemas"]["ExportFinancialReportCommand"];
export type ReportExportArtifact = components["schemas"]["ReportExportArtifact"];
export interface FinancialReportsWorkspace { context: WorkingContext; accounts: GLAccount[]; from: string; to: string; asOf: string; trialBalance: TrialBalance; profitAndLoss: ProfitAndLoss; balanceSheet: BalanceSheet; cashFlow: CashFlow }
export interface FinanceControlWorkspace { context: WorkingContext; accounts: BankAccount[]; glAccounts: GLAccount[]; postingMappings: PostingMapping[]; documents: FinancialDocument[]; periods: FiscalPeriod[]; periodActions: FiscalPeriodActionRequest[] }
export type Budget = components["schemas"]["Budget"];
export type BudgetPage = components["schemas"]["BudgetPage"];
export type BudgetActual = components["schemas"]["BudgetActual"];
export type CreateBudgetCommand = components["schemas"]["CreateBudgetCommand"];
export type AdvancedFinanceTransitionCommand = components["schemas"]["AdvancedFinanceTransitionCommand"];
export type FixedAsset = components["schemas"]["FixedAsset"];
export type FixedAssetPage = components["schemas"]["FixedAssetPage"];
export type CreateFixedAssetCommand = components["schemas"]["CreateFixedAssetCommand"];
export type CreatePurchasedFixedAssetCommand = components["schemas"]["CreatePurchasedFixedAssetCommand"];
export type FixedAssetDepreciation = components["schemas"]["FixedAssetDepreciation"];
export type DepreciateFixedAssetCommand = components["schemas"]["DepreciateFixedAssetCommand"];
export type DisposeFixedAssetCommand = components["schemas"]["DisposeFixedAssetCommand"];
export interface AdvancedFinanceWorkspace { context: WorkingContext; accounts: GLAccount[]; budgets: Budget[]; assets: FixedAsset[]; purchaseInvoices: OperationDocument[] }
export type TreasuryFacility = components["schemas"]["TreasuryFacility"];
export type TreasuryFacilityPage = components["schemas"]["TreasuryFacilityPage"];
export type CreateTreasuryFacilityCommand = components["schemas"]["CreateTreasuryFacilityCommand"];
export type TreasuryTransitionCommand = components["schemas"]["TreasuryTransitionCommand"];
export type PostTreasuryTransactionCommand = components["schemas"]["PostTreasuryTransactionCommand"];
export interface TreasuryWorkspace { context: WorkingContext; accounts: GLAccount[]; facilities: TreasuryFacility[] }
export type IntercompanyTransaction = components["schemas"]["IntercompanyTransaction"];
export type IntercompanyPage = components["schemas"]["IntercompanyPage"];
export type CreateIntercompanyCommand = components["schemas"]["CreateIntercompanyCommand"];
export type IntercompanyTransitionCommand = components["schemas"]["IntercompanyTransitionCommand"];
export type GroupConsolidation = components["schemas"]["GroupConsolidation"];
export interface GroupFinanceWorkspace { context: WorkingContext; accounts: GLAccount[]; transactions: IntercompanyTransaction[]; consolidation: GroupConsolidation | null }
