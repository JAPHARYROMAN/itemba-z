import type { BackendIdentity } from "@/live-api/auth";
import { buildIdentityHeaders } from "@/live-api/auth";
import type {
  ChangeMobileDeviceAllocationCommand, ChangeMobileDeviceStatusCommand, CompleteSaleCommand, CustomerAccountDetail, CustomerPage,
  MobileDevice, MobileDevicePage, MobileReconciliationCase, MobileReconciliationPage,
  MobileReconciliationStatus, ProductPage, PublicProblem, ResolveMobileReconciliationCommand,
  ReverseSaleCommand, Sale, SalePage, ScheduleCreditPolicyCommand, CreditPolicy, WorkingContext,
	CreateOperationCommand, OperationDocument, OperationDocumentType, OperationPage, TransitionOperationCommand,
	SupplierPage,
	CustomerCollection, ReceiveCustomerCollectionCommand,
	BankAccountPage, BankStatement, BankStatementPage, ImportBankStatementCommand, MatchBankStatementLineCommand, ReconcileBankStatementCommand,
	CreateFinancialDocumentCommand, FinancialDocument, FinancialDocumentPage, FiscalPeriodActionCommand, FiscalPeriodActionPage, FiscalPeriodActionRequest, FiscalPeriodPage, TransitionFinancialDocumentCommand,
	CreateGLAccountCommand, CreatePostingMappingCommand, GLAccount, GLAccountPage, GovernanceDecisionCommand, PostingMapping, PostingMappingPage,
	BalanceSheet, CashFlow, ExportFinancialReportCommand, GeneralLedger, ProfitAndLoss, ReportExportArtifact, TrialBalance,
	AdvancedFinanceTransitionCommand, Budget, BudgetActual, BudgetPage, CreateBudgetCommand, CreateFixedAssetCommand, CreatePurchasedFixedAssetCommand, DepreciateFixedAssetCommand, DisposeFixedAssetCommand, FixedAsset, FixedAssetDepreciation, FixedAssetPage,
	CreateTreasuryFacilityCommand, PostTreasuryTransactionCommand, TreasuryFacility, TreasuryFacilityPage, TreasuryTransitionCommand,
	CreateIntercompanyCommand, GroupConsolidation, IntercompanyPage, IntercompanyTransaction, IntercompanyTransitionCommand,
	Attendance, AttendanceCommand, CreateEmployeeCommand, Employee, EmployeeLoan, LeaveCommand, LeaveRequest, LeaveType, LeaveTypeCommand, LoanCommand, PayrollCommand, PayrollRun, PeopleSnapshot, PeopleTransitionCommand,
} from "@/live-api/types";
import { firstUnsafeIntegerPath } from "@/live-api/integer-safety";

type FetchImplementation = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

export class LiveApiError extends Error {
  readonly problem: PublicProblem;

  constructor(problem: PublicProblem) {
    super(problem.detail);
    this.name = "LiveApiError";
    this.problem = problem;
  }
}

export interface ApiClientOptions {
  baseUrl: string;
  identity: BackendIdentity;
  fetchImplementation?: FetchImplementation;
  createCorrelationId?: () => string;
  requestTimeoutMs?: number;
}

interface RequestOptions {
  method?: "GET" | "POST";
  body?: unknown;
  idempotencyKey?: string;
}

function normalizeBaseUrl(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:configuration",
      title: "Live API configuration invalid",
      status: 500,
      code: "api_base_url_invalid",
      detail: "ITEMBA_API_BASE_URL must be an absolute HTTP or HTTPS URL.",
    });
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:configuration",
      title: "Live API configuration invalid",
      status: 500,
      code: "api_base_url_invalid",
      detail: "ITEMBA_API_BASE_URL must use HTTP or HTTPS.",
    });
  }
  return url.toString().replace(/\/$/, "");
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && !/[\r\n]/.test(value);
}

function problemFromPayload(status: number, payload: unknown, correlationId: string): PublicProblem {
  if (payload && typeof payload === "object") {
    const value = payload as Record<string, unknown>;
    return {
      type: typeof value.type === "string" ? value.type : "about:blank",
      title: typeof value.title === "string" ? value.title : "Live API request failed",
      status,
      code: typeof value.code === "string" ? value.code : `upstream_http_${status}`,
      detail: typeof value.detail === "string" ? value.detail : "The ERP service rejected the request.",
      correlation_id: typeof value.correlation_id === "string" ? value.correlation_id : correlationId,
    };
  }
  return {
    type: "urn:itemba-z:control-center:upstream",
    title: "Live API request failed",
    status,
    code: `upstream_http_${status}`,
    detail: "The ERP service returned an unreadable error response.",
    correlation_id: correlationId,
  };
}

export class ItembaApiClient {
  private readonly baseUrl: string;
  private readonly identity: BackendIdentity;
  private readonly fetchImplementation: FetchImplementation;
  private readonly createCorrelationId: () => string;
  private readonly requestTimeoutMs: number;

  constructor(options: ApiClientOptions) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl);
    this.identity = options.identity;
    this.fetchImplementation = options.fetchImplementation ?? fetch;
    this.createCorrelationId = options.createCorrelationId ?? (() => crypto.randomUUID());
    this.requestTimeoutMs = options.requestTimeoutMs ?? 8_000;
  }

  private async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const correlationId = this.createCorrelationId();
    const unsafeRequestIntegerPath = options.body === undefined ? null : firstUnsafeIntegerPath(options.body);
    if (unsafeRequestIntegerPath) {
      throw new LiveApiError({
        type: "urn:itemba-z:control-center:request-contract",
        title: "Transaction integer is unsafe",
        status: 400,
        code: "request_integer_unsafe",
        detail: `The transaction command contains a non-safe integer at ${unsafeRequestIntegerPath}.`,
        correlation_id: correlationId,
      });
    }
    const headers = buildIdentityHeaders(this.identity);
    headers.set("Accept", "application/json, application/problem+json");
    headers.set("X-Correlation-ID", correlationId);

    if (options.body !== undefined) headers.set("Content-Type", "application/json");
    if (options.idempotencyKey !== undefined) {
      if (!validIdempotencyKey(options.idempotencyKey)) {
        throw new LiveApiError({
          type: "urn:itemba-z:control-center:idempotency",
          title: "Idempotency key invalid",
          status: 400,
          code: "idempotency_key_invalid",
          detail: "Idempotency-Key must contain 16 to 128 characters.",
          correlation_id: correlationId,
        });
      }
      headers.set("Idempotency-Key", options.idempotencyKey);
    }

    let response: Response;
    try {
      response = await this.fetchImplementation(`${this.baseUrl}${path}`, {
        method: options.method ?? "GET",
        headers,
        body: options.body === undefined ? undefined : JSON.stringify(options.body),
        cache: "no-store",
        signal: AbortSignal.timeout(this.requestTimeoutMs),
      });
    } catch {
      throw new LiveApiError({
        type: "urn:itemba-z:control-center:upstream",
        title: "Live ERP unavailable",
        status: 503,
        code: "upstream_unavailable",
        detail: "The Control Center could not reach the live ERP service.",
        correlation_id: correlationId,
      });
    }

    const payload: unknown = await response.json().catch(() => undefined);
    const unsafeIntegerPath = firstUnsafeIntegerPath(payload);
    if (unsafeIntegerPath) {
      throw new LiveApiError({
        type: "urn:itemba-z:control-center:upstream-contract",
        title: "Live API integer is unsafe",
        status: 502,
        code: "upstream_integer_unsafe",
        detail: `The ERP response contains a non-safe integer at ${unsafeIntegerPath}.`,
        correlation_id: correlationId,
      });
    }
    if (!response.ok) throw new LiveApiError(problemFromPayload(response.status, payload, correlationId));
    return payload as T;
  }

  getWorkingContext(): Promise<WorkingContext> {
    return this.request("/v1/context");
  }

  listCustomers(): Promise<CustomerPage> {
    return this.request("/v1/customers?page_size=200");
  }

  getCustomerAccount(customerId: string): Promise<CustomerAccountDetail> {
    return this.request(`/v1/customers/${encodeURIComponent(customerId)}/account`);
  }

  scheduleCustomerCreditPolicy(customerId: string, command: ScheduleCreditPolicyCommand, idempotencyKey: string): Promise<CreditPolicy> {
    return this.request(`/v1/customers/${encodeURIComponent(customerId)}/credit-policies`, {
      method: "POST", body: command, idempotencyKey,
    });
  }

  listProducts(): Promise<ProductPage> {
    return this.request("/v1/products?page_size=200");
  }

  listSales(cursor?: string): Promise<SalePage> {
    const query = new URLSearchParams({ page_size: "100" });
    if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/sales?${query.toString()}`);
  }

  getSale(saleId: string): Promise<Sale> {
    return this.request(`/v1/sales/${encodeURIComponent(saleId)}`);
  }

  completeSale(command: CompleteSaleCommand, idempotencyKey: string): Promise<Sale> {
    return this.request("/v1/sales", { method: "POST", body: command, idempotencyKey });
  }

  reverseSale(saleId: string, command: ReverseSaleCommand, idempotencyKey: string): Promise<Sale> {
    return this.request(`/v1/sales/${encodeURIComponent(saleId)}/reversals`, {
      method: "POST",
      body: command,
      idempotencyKey,
    });
  }

  listOperationDocuments(type?: OperationDocumentType, cursor?: string): Promise<OperationPage> {
    const query = new URLSearchParams({ page_size: "100" }); if (type) query.set("type", type); if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/operations/documents?${query.toString()}`);
  }

  listSuppliers(): Promise<SupplierPage> { return this.request("/v1/suppliers?page_size=200"); }

  receiveCustomerCollection(customerId: string, command: ReceiveCustomerCollectionCommand, idempotencyKey: string): Promise<CustomerCollection> { return this.request(`/v1/customers/${encodeURIComponent(customerId)}/collections`, { method: "POST", body: command, idempotencyKey }); }

  createOperationDocument(command: CreateOperationCommand, idempotencyKey: string): Promise<OperationDocument> {
    return this.request("/v1/operations/documents", { method: "POST", body: command, idempotencyKey });
  }

  transitionOperationDocument(documentId: string, command: TransitionOperationCommand, idempotencyKey: string): Promise<OperationDocument> {
    return this.request(`/v1/operations/documents/${encodeURIComponent(documentId)}/transitions`, { method: "POST", body: command, idempotencyKey });
  }

  listBankAccounts(): Promise<BankAccountPage> { return this.request("/v1/banking/accounts"); }
  listBankStatements(cursor?: string): Promise<BankStatementPage> { const query = new URLSearchParams({ page_size: "100" }); if (cursor) query.set("cursor", cursor); return this.request(`/v1/banking/statements?${query.toString()}`); }
  getBankStatement(statementId: string): Promise<BankStatement> { return this.request(`/v1/banking/statements/${encodeURIComponent(statementId)}`); }
  importBankStatement(command: ImportBankStatementCommand, idempotencyKey: string): Promise<BankStatement> { return this.request("/v1/banking/statements", { method: "POST", body: command, idempotencyKey }); }
  matchBankStatementLine(statementId: string, lineId: string, command: MatchBankStatementLineCommand, idempotencyKey: string): Promise<BankStatement> { return this.request(`/v1/banking/statements/${encodeURIComponent(statementId)}/lines/${encodeURIComponent(lineId)}/matches`, { method: "POST", body: command, idempotencyKey }); }
  reconcileBankStatement(statementId: string, command: ReconcileBankStatementCommand, idempotencyKey: string): Promise<BankStatement> { return this.request(`/v1/banking/statements/${encodeURIComponent(statementId)}/reconciliation`, { method: "POST", body: command, idempotencyKey }); }
  listFinancialDocuments(cursor?: string): Promise<FinancialDocumentPage> { const query = new URLSearchParams({ page_size: "100" }); if (cursor) query.set("cursor", cursor); return this.request(`/v1/finance/documents?${query.toString()}`); }
  createFinancialDocument(command: CreateFinancialDocumentCommand, idempotencyKey: string): Promise<FinancialDocument> { return this.request("/v1/finance/documents", { method: "POST", body: command, idempotencyKey }); }
  transitionFinancialDocument(documentId: string, command: TransitionFinancialDocumentCommand, idempotencyKey: string): Promise<FinancialDocument> { return this.request(`/v1/finance/documents/${encodeURIComponent(documentId)}/transitions`, { method: "POST", body: command, idempotencyKey }); }
  listFiscalPeriods(): Promise<FiscalPeriodPage> { return this.request("/v1/finance/fiscal-periods"); }
  listFiscalPeriodActions(): Promise<FiscalPeriodActionPage> { return this.request("/v1/finance/fiscal-period-actions"); }
  requestFiscalPeriodAction(periodId: string, command: FiscalPeriodActionCommand, idempotencyKey: string): Promise<FiscalPeriodActionRequest> { return this.request(`/v1/finance/fiscal-periods/${encodeURIComponent(periodId)}/actions`, { method: "POST", body: command, idempotencyKey }); }
  approveFiscalPeriodAction(actionId: string, reason: string, idempotencyKey: string): Promise<FiscalPeriodActionRequest> { return this.request(`/v1/finance/fiscal-period-actions/${encodeURIComponent(actionId)}/approval`, { method: "POST", body: { reason }, idempotencyKey }); }
  listGLAccounts(): Promise<GLAccountPage> { return this.request("/v1/finance/accounts"); }
  createGLAccount(command: CreateGLAccountCommand, idempotencyKey: string): Promise<GLAccount> { return this.request("/v1/finance/accounts", { method: "POST", body: command, idempotencyKey }); }
  decideGLAccount(accountId: string, command: GovernanceDecisionCommand, idempotencyKey: string): Promise<GLAccount> { return this.request(`/v1/finance/accounts/${encodeURIComponent(accountId)}/decisions`, { method: "POST", body: command, idempotencyKey }); }
  listPostingMappings(): Promise<PostingMappingPage> { return this.request("/v1/finance/posting-mappings"); }
  createPostingMapping(command: CreatePostingMappingCommand, idempotencyKey: string): Promise<PostingMapping> { return this.request("/v1/finance/posting-mappings", { method: "POST", body: command, idempotencyKey }); }
  decidePostingMapping(mappingId: string, command: GovernanceDecisionCommand, idempotencyKey: string): Promise<PostingMapping> { return this.request(`/v1/finance/posting-mappings/${encodeURIComponent(mappingId)}/decisions`, { method: "POST", body: command, idempotencyKey }); }
  trialBalance(asOf: string): Promise<TrialBalance> { return this.request(`/v1/reports/financial/trial-balance?as_of=${encodeURIComponent(asOf)}`); }
  generalLedger(from: string, to: string, accountId: string): Promise<GeneralLedger> { const query = new URLSearchParams({ from, to, account_id: accountId }); return this.request(`/v1/reports/financial/general-ledger?${query}`); }
  profitAndLoss(from: string, to: string): Promise<ProfitAndLoss> { const query = new URLSearchParams({ from, to }); return this.request(`/v1/reports/financial/profit-and-loss?${query}`); }
  balanceSheet(asOf: string): Promise<BalanceSheet> { return this.request(`/v1/reports/financial/balance-sheet?as_of=${encodeURIComponent(asOf)}`); }
  cashFlow(from: string, to: string): Promise<CashFlow> { const query = new URLSearchParams({ from, to }); return this.request(`/v1/reports/financial/cash-flow?${query}`); }
  exportFinancialReport(command: ExportFinancialReportCommand, idempotencyKey: string): Promise<ReportExportArtifact> { return this.request("/v1/reports/financial/exports", { method: "POST", body: command, idempotencyKey }); }
  listBudgets(): Promise<BudgetPage> { return this.request("/v1/finance/budgets"); }
  createBudget(command: CreateBudgetCommand, idempotencyKey: string): Promise<Budget> { return this.request("/v1/finance/budgets", { method: "POST", body: command, idempotencyKey }); }
  transitionBudget(id: string, command: AdvancedFinanceTransitionCommand, idempotencyKey: string): Promise<Budget> { return this.request(`/v1/finance/budgets/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey }); }
  budgetActual(id: string): Promise<BudgetActual> { return this.request(`/v1/finance/budgets/${encodeURIComponent(id)}/actual`); }
  listFixedAssets(): Promise<FixedAssetPage> { return this.request("/v1/finance/assets"); }
  createFixedAsset(command: CreateFixedAssetCommand, idempotencyKey: string): Promise<FixedAsset> { return this.request("/v1/finance/assets", { method: "POST", body: command, idempotencyKey }); }
  createFixedAssetFromPurchase(command: CreatePurchasedFixedAssetCommand, idempotencyKey: string): Promise<FixedAsset> { return this.request("/v1/finance/assets/from-purchase", { method: "POST", body: command, idempotencyKey }); }
  transitionFixedAsset(id: string, command: AdvancedFinanceTransitionCommand, idempotencyKey: string): Promise<FixedAsset> { return this.request(`/v1/finance/assets/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey }); }
  depreciateFixedAsset(id: string, command: DepreciateFixedAssetCommand, idempotencyKey: string): Promise<FixedAssetDepreciation> { return this.request(`/v1/finance/assets/${encodeURIComponent(id)}/depreciation`, { method: "POST", body: command, idempotencyKey }); }
  disposeFixedAsset(id: string, command: DisposeFixedAssetCommand, idempotencyKey: string): Promise<FixedAsset> { return this.request(`/v1/finance/assets/${encodeURIComponent(id)}/disposal`, { method: "POST", body: command, idempotencyKey }); }
  listTreasuryFacilities(): Promise<TreasuryFacilityPage> { return this.request("/v1/finance/facilities"); }
  createTreasuryFacility(command: CreateTreasuryFacilityCommand, idempotencyKey: string): Promise<TreasuryFacility> { return this.request("/v1/finance/facilities", { method: "POST", body: command, idempotencyKey }); }
  transitionTreasuryFacility(id: string, command: TreasuryTransitionCommand, idempotencyKey: string): Promise<TreasuryFacility> { return this.request(`/v1/finance/facilities/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey }); }
  postTreasuryTransaction(id: string, command: PostTreasuryTransactionCommand, idempotencyKey: string): Promise<TreasuryFacility> { return this.request(`/v1/finance/facilities/${encodeURIComponent(id)}/transactions`, { method: "POST", body: command, idempotencyKey }); }
  listIntercompanyTransactions(): Promise<IntercompanyPage> { return this.request("/v1/finance/intercompany"); }
  createIntercompanyTransaction(command: CreateIntercompanyCommand, idempotencyKey: string): Promise<IntercompanyTransaction> { return this.request("/v1/finance/intercompany", { method: "POST", body: command, idempotencyKey }); }
  transitionIntercompanyTransaction(id: string, command: IntercompanyTransitionCommand, idempotencyKey: string): Promise<IntercompanyTransaction> { return this.request(`/v1/finance/intercompany/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey }); }
  groupConsolidation(asOf: string): Promise<GroupConsolidation> { return this.request(`/v1/reports/financial/consolidation?as_of=${encodeURIComponent(asOf)}`); }
  getPeopleSnapshot(): Promise<PeopleSnapshot> { return this.request("/v1/hr"); }
  createEmployee(command: CreateEmployeeCommand, key: string): Promise<Employee> { return this.request("/v1/hr/employees", { method: "POST", body: command, idempotencyKey: key }); }
  recordAttendance(command: AttendanceCommand, key: string): Promise<Attendance> { return this.request("/v1/hr/attendance", { method: "POST", body: command, idempotencyKey: key }); }
  createLeaveType(command: LeaveTypeCommand, key: string): Promise<LeaveType> { return this.request("/v1/hr/leave-types", { method: "POST", body: command, idempotencyKey: key }); }
  createLeave(command: LeaveCommand, key: string): Promise<LeaveRequest> { return this.request("/v1/hr/leave-requests", { method: "POST", body: command, idempotencyKey: key }); }
  transitionLeave(id: string, command: PeopleTransitionCommand, key: string): Promise<LeaveRequest> { return this.request(`/v1/hr/leave-requests/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey: key }); }
  createEmployeeLoan(command: LoanCommand, key: string): Promise<EmployeeLoan> { return this.request("/v1/hr/loans", { method: "POST", body: command, idempotencyKey: key }); }
  transitionEmployeeLoan(id: string, command: PeopleTransitionCommand, key: string): Promise<EmployeeLoan> { return this.request(`/v1/hr/loans/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey: key }); }
  createPayroll(command: PayrollCommand, key: string): Promise<PayrollRun> { return this.request("/v1/hr/payroll-runs", { method: "POST", body: command, idempotencyKey: key }); }
  transitionPayroll(id: string, command: PeopleTransitionCommand, key: string): Promise<PayrollRun> { return this.request(`/v1/hr/payroll-runs/${encodeURIComponent(id)}/transitions`, { method: "POST", body: command, idempotencyKey: key }); }

  listReconciliationCases(status?: MobileReconciliationStatus, cursor?: string): Promise<MobileReconciliationPage> {
    const query = new URLSearchParams({ page_size: "100" });
    if (status) query.set("status", status);
    if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/mobile/reconciliation-cases?${query.toString()}`);
  }

  getReconciliationCase(caseId: string): Promise<MobileReconciliationCase> {
    return this.request(`/v1/mobile/reconciliation-cases/${encodeURIComponent(caseId)}`);
  }

  resolveReconciliationCase(caseId: string, command: ResolveMobileReconciliationCommand, idempotencyKey: string): Promise<MobileReconciliationCase> {
    return this.request(`/v1/mobile/reconciliation-cases/${encodeURIComponent(caseId)}/resolutions`, {
      method: "POST",
      body: command,
      idempotencyKey,
    });
  }

  listManagedDevices(cursor?: string): Promise<MobileDevicePage> {
    const query = new URLSearchParams({ page_size: "100" });
    if (cursor) query.set("cursor", cursor);
    return this.request(`/v1/mobile/devices?${query.toString()}`);
  }

  changeManagedDeviceStatus(deviceId: string, command: ChangeMobileDeviceStatusCommand, idempotencyKey: string): Promise<MobileDevice> {
    return this.request(`/v1/mobile/devices/${encodeURIComponent(deviceId)}/status-changes`, {
      method: "POST", body: command, idempotencyKey,
    });
  }

  changeManagedDeviceAllocation(deviceId: string, command: ChangeMobileDeviceAllocationCommand, idempotencyKey: string): Promise<MobileDevice> {
    return this.request(`/v1/mobile/devices/${encodeURIComponent(deviceId)}/allocation-changes`, {
      method: "POST", body: command, idempotencyKey,
    });
  }
}
