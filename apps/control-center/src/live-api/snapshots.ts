import "server-only";

import { createServerRepository } from "@/live-api/server-repository";
import { publicProblem } from "@/live-api/errors";
import { listAllOperationDocuments } from "@/live-api/operation-pagination";
import type {
  CustomerAccountWorkspace, CustomerAccountsWorkspace, DashboardWorkspace, DeviceManagementWorkspace, LiveSnapshot, MobileReconciliationStatus, ReconciliationDetailWorkspace,
  ReconciliationWorkspace, SaleDetailWorkspace, SalesBootstrap, SalesRegisterWorkspace, SalesWorkspace,
	OperationsWorkspace,
	PurchaseWorkspace,
	PurchaseDocumentWorkspace,
	BankingWorkspace,
	FinanceControlWorkspace,
	FinancialReportsWorkspace,
	AdvancedFinanceWorkspace,
	TreasuryWorkspace,
	GroupFinanceWorkspace,
	PeopleWorkspace,
	ConfigurationWorkspace,
	CommercialWorkspace,
	SupplierWorkspace,
	GlobalSearchWorkspace,
	InventoryControlWorkspace,
	IntegrationOperationsWorkspace,
	PublicProblem,
} from "@/live-api/types";
import { text } from "@/lib/i18n";
import { canAccessSalesRoute, type SalesRoute } from "@/lib/sales-permissions";
import { canAccessPurchaseRoute, type PurchaseRoute } from "@/lib/purchase-permissions";

function salesAccessDenied<T>(route: SalesRoute): LiveSnapshot<T> {
  const problem: PublicProblem = {
    type: "urn:itemba-z:control-center:permissions",
    title: "Sales task unavailable",
    status: 403,
    code: "sales_task_not_permitted",
    detail: `The current role does not have every permission required for the ${route} Sales task.`,
  };
  return { state: "unavailable", problem };
}

function purchaseAccessDenied<T>(route: PurchaseRoute): LiveSnapshot<T> {
  return { state: "unavailable", problem: { type: "urn:itemba-z:control-center:permissions", title: "Purchasing task unavailable", status: 403, code: "purchase_task_not_permitted", detail: `The current role does not have every permission required for the ${route} Purchasing task.` } };
}

const PURCHASE_DOCUMENT_TYPES = ["PURCHASE_REQUEST", "PURCHASE_ORDER", "GOODS_RECEIPT", "SUPPLIER_INVOICE", "SUPPLIER_PAYMENT", "PURCHASE_RETURN"] as const;

function newestDocumentsFirst(left: OperationsWorkspace["documents"][number], right: OperationsWorkspace["documents"][number]): number {
  return Date.parse(right.created_at) - Date.parse(left.created_at) || right.id.localeCompare(left.id);
}

export async function loadDashboardWorkspace(): Promise<LiveSnapshot<DashboardWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, dashboard] = await Promise.all([
      repository.getWorkingContext(),
      repository.getDashboard(),
    ]);
    return { state: "ready", data: { context, dashboard } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadInventoryControlWorkspace(): Promise<LiveSnapshot<InventoryControlWorkspace>> {
	try {
		const repository = await createServerRepository();
		const [context, inventory, products, suppliers, documents] = await Promise.all([repository.getWorkingContext(), repository.getInventoryControlSnapshot(), repository.listProducts(), repository.listSuppliers(), repository.listOperationDocuments()]);
		return { state: "ready", data: { context, inventory, products: products.items, suppliers: suppliers.items, receipts: documents.items.filter((document) => document.type === "GOODS_RECEIPT" && document.status === "APPROVED") } };
	} catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadPeopleWorkspace(): Promise<LiveSnapshot<PeopleWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, people, workforce, configuration, accounts] = await Promise.all([repository.getWorkingContext(), repository.getPeopleSnapshot(), repository.getWorkforceSnapshot(), repository.getConfigurationSnapshot(), repository.listGLAccounts()]);
    return { state: "ready", data: { context, people, workforce, configurations: configuration.versions.filter((version) => version.status === "ACTIVE" && (version.category === "HR" || version.category === "INTEGRATIONS")), accounts: accounts.items.filter((account) => account.status === "ACTIVE") } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadConfigurationWorkspace(): Promise<LiveSnapshot<ConfigurationWorkspace>> {
  try { const repository = await createServerRepository(); const [context, configuration] = await Promise.all([repository.getWorkingContext(), repository.getConfigurationSnapshot()]); return { state: "ready", data: { context, configuration } }; }
  catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadIntegrationOperationsWorkspace(): Promise<LiveSnapshot<IntegrationOperationsWorkspace>> {
  try { const repository = await createServerRepository(); const [context, integrations] = await Promise.all([repository.getWorkingContext(), repository.getIntegrationWorkspace()]); return { state: "ready", data: { context, integrations } }; }
  catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadCommercialWorkspace(): Promise<LiveSnapshot<CommercialWorkspace>> {
	try { const repository=await createServerRepository(); const [context,commercial,products,suppliers]=await Promise.all([repository.getWorkingContext(),repository.getCommercialSnapshot(),repository.listProducts(),repository.listSuppliers()]); return {state:"ready",data:{context,commercial,products:products.items,suppliers:suppliers.items}}; }
	catch(error){return {state:"unavailable",problem:publicProblem(error)};}
}

export async function loadSupplierWorkspace(): Promise<LiveSnapshot<SupplierWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    const [suppliers, commercial] = await Promise.all([
      repository.listSuppliers(),
      context.permissions.includes("masterdata.read") && context.permissions.includes("purchases.sourcing.read")
        ? repository.getCommercialSnapshot()
        : Promise.resolve(null),
    ]);
    return { state: "ready", data: { context, suppliers: suppliers.items, commercial } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadGlobalSearch(rawQuery: string): Promise<LiveSnapshot<GlobalSearchWorkspace>> {
  try {
    const query = rawQuery.trim().slice(0, 120);
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (query.length < 2) return { state: "ready", data: { context, query, results: [], searchedSources: [] } };
    const needle = query.toLocaleLowerCase("en");
    const allowed = (permission: string) => context.permissions.includes(permission);
    const sources: Array<Promise<void>> = [];
    const results: GlobalSearchWorkspace["results"] = [];
    const searchedSources: GlobalSearchWorkspace["searchedSources"] = [];
    const matches = (...values: Array<string | undefined>) => values.some((value) => value?.toLocaleLowerCase("en").includes(needle));

    if (allowed("customers.read")) {
      searchedSources.push(text("Customers", "Wateja"));
      sources.push(repository.listCustomers().then(({ items }) => items.filter((item) => matches(item.code, item.name, item.id)).forEach((item) => results.push({
        id: item.id, module: "customers", moduleLabel: text("Customer", "Mteja"), title: text(item.name, item.name),
        subtitle: `${item.code} · ${item.current_exposure_minor.toLocaleString()} ${context.currency}`,
        href: `/customers/${item.id}`, status: { label: text(item.status === "active" ? "Active" : "Inactive", item.status === "active" ? "Hai" : "Isiyotumika"), tone: item.status === "active" ? "success" : "neutral" },
      }))));
    }
    if (allowed("products.read")) {
      searchedSources.push(text("Products", "Bidhaa"));
      sources.push(repository.listProducts().then(({ items }) => items.filter((item) => matches(item.code, item.name, item.id)).forEach((item) => results.push({
        id: item.id, module: "inventory", moduleLabel: text("Product", "Bidhaa"), title: text(item.name, item.name),
        subtitle: `${item.code} · ${item.available_quantity.toLocaleString()} ${item.unit}`,
        href: `/inventory?product=${encodeURIComponent(item.id)}`, status: { label: text(item.available_quantity > 0 ? "Available" : "Out of stock", item.available_quantity > 0 ? "Inapatikana" : "Imeisha"), tone: item.available_quantity > 0 ? "success" : "danger" },
      }))));
    }
    if (allowed("sales.read")) {
      searchedSources.push(text("Sales", "Mauzo"));
      sources.push(repository.listSales().then(({ items }) => items.filter((item) => matches(item.receipt_reference, item.id, item.customer_id)).forEach((item) => results.push({
        id: item.id, module: "sales", moduleLabel: text("Sale", "Mauzo"), title: text(item.receipt_reference, item.receipt_reference),
        subtitle: `${item.total_minor.toLocaleString()} ${item.currency} · ${item.kind}`,
        href: `/sales/${item.id}`, status: { label: text(item.status, item.status === "POSTED" ? "IMECHAPISHWA" : "IMEBATILISHWA"), tone: item.status === "POSTED" ? "success" : "warning" },
      }))));
    }
    if (allowed("operations.read")) {
      searchedSources.push(text("Suppliers and operations", "Wasambazaji na shughuli"));
      sources.push(Promise.all([repository.listSuppliers(), repository.listOperationDocuments()]).then(([suppliers, documents]) => {
        suppliers.items.filter((item) => matches(item.code, item.name, item.id)).forEach((item) => results.push({
          id: item.id, module: "suppliers", moduleLabel: text("Supplier", "Msambazaji"), title: text(item.name, item.name),
          subtitle: `${item.code} · ${item.payment_terms_days} days`, href: `/suppliers?supplier=${encodeURIComponent(item.id)}`,
          status: { label: text(item.active ? "Active" : "Inactive", item.active ? "Hai" : "Isiyotumika"), tone: item.active ? "success" : "neutral" },
        }));
        documents.items.filter((item) => matches(item.number, item.id, item.party_id, item.type)).forEach((item) => {
          const href = item.type.startsWith("PURCHASE") || item.type === "GOODS_RECEIPT" || item.type.startsWith("SUPPLIER") ? `/purchases?document=${encodeURIComponent(item.id)}` : item.type.startsWith("STOCK") ? `/inventory?document=${encodeURIComponent(item.id)}` : `/sales/lifecycle?document=${encodeURIComponent(item.id)}`;
          results.push({ id: item.id, module: "operations", moduleLabel: text("Operation", "Shughuli"), title: text(item.number, item.number), subtitle: `${item.type} · ${item.total_minor.toLocaleString()} ${item.currency}`, href, status: { label: text(item.status, item.status), tone: item.status === "REJECTED" || item.status === "REVERSED" ? "danger" : item.status === "CLOSED" || item.status === "POSTED" || item.status === "RECEIVED" ? "success" : "info" } });
        });
      }));
    }
    if (allowed("finance.accounts.read")) {
      searchedSources.push(text("Chart of accounts", "Orodha ya akaunti"));
      sources.push(repository.listGLAccounts().then(({ items }) => items.filter((item) => matches(item.code, item.name, item.id)).forEach((item) => results.push({
        id: item.record_id, module: "finance", moduleLabel: text("GL account", "Akaunti ya leja"), title: text(item.name, item.name),
        subtitle: `${item.code} · ${item.type}`, href: `/finance?account=${encodeURIComponent(item.record_id)}`,
        status: { label: text(item.status, item.status), tone: item.status === "ACTIVE" ? "success" : item.status === "REJECTED" ? "danger" : "neutral" },
      }))));
    }
    if (allowed("hr.people.read")) {
      searchedSources.push(text("Employees", "Wafanyakazi"));
      sources.push(repository.getPeopleSnapshot().then(({ employees }) => employees.filter((item) => matches(item.number, item.full_name, item.job_title, item.department, item.id)).forEach((item) => results.push({
        id: item.id, module: "human-resources", moduleLabel: text("Employee", "Mfanyakazi"), title: text(item.full_name, item.full_name),
        subtitle: `${item.number} · ${item.job_title ?? item.department ?? "—"}`, href: `/human-resources?employee=${encodeURIComponent(item.id)}`,
        status: { label: text(item.status, item.status === "ACTIVE" ? "HAI" : "ISiyotumika"), tone: item.status === "ACTIVE" ? "success" : "neutral" },
      }))));
    }
    await Promise.all(sources);
    results.sort((left, right) => left.title.en.localeCompare(right.title.en));
    return { state: "ready", data: { context, query, results: results.slice(0, 50), searchedSources } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadGroupFinanceWorkspace(): Promise<LiveSnapshot<GroupFinanceWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    const [accounts, transactions, consolidation] = await Promise.all([
      repository.listGLAccounts(), repository.listIntercompanyTransactions(),
      context.permissions.includes("finance.consolidation.read") ? repository.groupConsolidation(new Date().toISOString()) : Promise.resolve(null),
    ]);
    return { state: "ready", data: { context, accounts: accounts.items.filter((account) => account.status === "ACTIVE"), transactions: transactions.items, consolidation } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadTreasuryWorkspace(): Promise<LiveSnapshot<TreasuryWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, accounts, facilities] = await Promise.all([repository.getWorkingContext(), repository.listGLAccounts(), repository.listTreasuryFacilities()]);
    return { state: "ready", data: { context, accounts: accounts.items.filter((account) => account.status === "ACTIVE"), facilities: facilities.items } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadAdvancedFinanceWorkspace(): Promise<LiveSnapshot<AdvancedFinanceWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, accounts, budgets, assets, documents] = await Promise.all([repository.getWorkingContext(), repository.listGLAccounts(), repository.listBudgets(), repository.listFixedAssets(), repository.listOperationDocuments()]);
    return { state: "ready", data: { context, accounts: accounts.items.filter((account) => account.status === "ACTIVE"), budgets: budgets.items, assets: assets.items, purchaseInvoices: documents.items.filter((document) => document.type === "SUPPLIER_INVOICE" && document.status === "POSTED") } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadCustomerAccounts(): Promise<LiveSnapshot<CustomerAccountsWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, customers] = await Promise.all([repository.getWorkingContext(), repository.listCustomers()]);
    return { state: "ready", data: { context, customers: customers.items } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadFinanceControlWorkspace(): Promise<LiveSnapshot<FinanceControlWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, accounts, glAccounts, postingMappings, documents, periods, periodActions] = await Promise.all([repository.getWorkingContext(), repository.listBankAccounts(), repository.listGLAccounts(), repository.listPostingMappings(), repository.listFinancialDocuments(), repository.listFiscalPeriods(), repository.listFiscalPeriodActions()]);
    return { state: "ready", data: { context, accounts: accounts.items, glAccounts: glAccounts.items, postingMappings: postingMappings.items, documents: documents.items, periods: periods.items, periodActions: periodActions.items } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadFinancialReportsWorkspace(): Promise<LiveSnapshot<FinancialReportsWorkspace>> {
  try {
    const repository = await createServerRepository();
    const now = new Date(); const parts = new Intl.DateTimeFormat("en", { timeZone: "Africa/Dar_es_Salaam", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(now); const datePart = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? ""; const to = `${datePart("year")}-${datePart("month")}-${datePart("day")}`; const from = `${to.slice(0, 8)}01`; const asOf = to;
    const [context, accounts, trialBalance, profitAndLoss, balanceSheet, cashFlow] = await Promise.all([repository.getWorkingContext(), repository.listGLAccounts(), repository.trialBalance(asOf), repository.profitAndLoss(from, to), repository.balanceSheet(asOf), repository.cashFlow(from, to)]);
    return { state: "ready", data: { context, accounts: accounts.items.filter((account) => account.status === "ACTIVE" || account.status === "INACTIVE"), from, to, asOf, trialBalance, profitAndLoss, balanceSheet, cashFlow } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadBankingWorkspace(cursor?: string): Promise<LiveSnapshot<BankingWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, accounts, statements] = await Promise.all([repository.getWorkingContext(), repository.listBankAccounts(), repository.listBankStatements(cursor)]);
    return { state: "ready", data: { context, accounts: accounts.items, statements: statements.items, nextCursor: statements.next_cursor ?? null } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadOperationsWorkspace(): Promise<LiveSnapshot<OperationsWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, customers, products, suppliers, documents] = await Promise.all([repository.getWorkingContext(), repository.listCustomers(), repository.listProducts(), repository.listSuppliers(), repository.listOperationDocuments()]);
    return { state: "ready", data: { context, customers: customers.items, products: products.items, suppliers: suppliers.items, documents: documents.items, nextCursor: documents.next_cursor ?? null } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
}

export async function loadPurchaseWorkspace(route: Exclude<PurchaseRoute, "sourcing">): Promise<LiveSnapshot<PurchaseWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessPurchaseRoute(context.permissions, route)) return purchaseAccessDenied(route);
    const needsCatalog = route !== "overview";
    const [products, suppliers, pages] = await Promise.all([
      needsCatalog ? repository.listProducts() : Promise.resolve({ items: [] }),
      needsCatalog ? repository.listSuppliers() : Promise.resolve({ items: [] }),
      Promise.all(PURCHASE_DOCUMENT_TYPES.map((type) => listAllOperationDocuments(repository, type))),
    ]);
    return { state: "ready", data: { context, products: products.items, suppliers: suppliers.items, documents: pages.flat().sort(newestDocumentsFirst) } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadPurchaseDocument(documentId: string): Promise<LiveSnapshot<PurchaseDocumentWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessPurchaseRoute(context.permissions, "overview")) return purchaseAccessDenied("overview");
    const [document, products, suppliers, pages] = await Promise.all([
      repository.getOperationDocument(documentId),
      repository.listProducts(),
      repository.listSuppliers(),
      Promise.all(PURCHASE_DOCUMENT_TYPES.map((type) => listAllOperationDocuments(repository, type))),
    ]);
    if (!PURCHASE_DOCUMENT_TYPES.includes(document.type as typeof PURCHASE_DOCUMENT_TYPES[number])) {
      return { state: "unavailable", problem: { type: "about:blank", title: "Purchase document not found", status: 404, code: "purchase_document_not_found", detail: "The requested record is not a Purchasing document in this operating scope." } };
    }
    return { state: "ready", data: { context, document, products: products.items, suppliers: suppliers.items, documents: pages.flat().sort(newestDocumentsFirst) } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadCustomerAccount(customerId: string): Promise<LiveSnapshot<CustomerAccountWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, account] = await Promise.all([repository.getWorkingContext(), repository.getCustomerAccount(customerId)]);
    return { state: "ready", data: { context, account } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadSalesBootstrap(): Promise<LiveSnapshot<SalesBootstrap>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessSalesRoute(context.permissions, "new")) return salesAccessDenied("new");
    const [customers, products, documents] = await Promise.all([
      repository.listCustomers(),
      repository.listProducts(),
      context.permissions.includes("operations.read")
        ? listAllOperationDocuments(repository, "SALES_ORDER")
        : Promise.resolve([] as OperationsWorkspace["documents"]),
    ]);
    return { state: "ready", data: { context, customers: customers.items, products: products.items, documents: documents.filter((document) => document.status === "APPROVED").sort(newestDocumentsFirst) } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadDeviceManagementWorkspace(cursor?: string): Promise<LiveSnapshot<DeviceManagementWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, devices, products] = await Promise.all([
      repository.getWorkingContext(), repository.listManagedDevices(cursor), repository.listProducts(),
    ]);
    return { state: "ready", data: { context, devices: devices.items, products: products.items, nextCursor: devices.next_cursor ?? null } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadReconciliationWorkspace(status: MobileReconciliationStatus | "", cursor?: string): Promise<LiveSnapshot<ReconciliationWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, cases] = await Promise.all([
      repository.getWorkingContext(),
      repository.listReconciliationCases(status || undefined, cursor),
    ]);
    return { state: "ready", data: { context, cases: cases.items, nextCursor: cases.next_cursor ?? null, status } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadReconciliationDetail(caseId: string): Promise<LiveSnapshot<ReconciliationDetailWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, reconciliationCase] = await Promise.all([
      repository.getWorkingContext(),
      repository.getReconciliationCase(caseId),
    ]);
    return { state: "ready", data: { context, reconciliationCase } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadSalesWorkspace(cursor?: string): Promise<LiveSnapshot<SalesWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessSalesRoute(context.permissions, "overview")) return salesAccessDenied("overview");
    const [customers, products, sales, documentPages] = await Promise.all([
      repository.listCustomers(),
      repository.listProducts(),
      repository.listSales(cursor),
      canAccessSalesRoute(context.permissions, "documents")
        ? Promise.all([listAllOperationDocuments(repository, "SALES_ORDER"), listAllOperationDocuments(repository, "QUOTATION")])
        : Promise.resolve([[] as OperationsWorkspace["documents"], [] as OperationsWorkspace["documents"]]),
    ]);
    return {
      state: "ready",
      data: {
        context,
        customers: customers.items,
        products: products.items,
        sales: sales.items,
        documents: documentPages.flat().sort(newestDocumentsFirst),
        nextCursor: sales.next_cursor ?? null,
      },
    };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadSalesRegisterWorkspace(route: "transactions" | "returns", cursor?: string): Promise<LiveSnapshot<SalesRegisterWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessSalesRoute(context.permissions, route)) return salesAccessDenied(route);
    const [customers, sales] = await Promise.all([
      repository.listCustomers(),
      repository.listSales(cursor),
    ]);
    return {
      state: "ready",
      data: {
        context,
        customers: customers.items,
        sales: sales.items,
        nextCursor: sales.next_cursor ?? null,
      },
    };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadSalesPaymentWorkspace(): Promise<LiveSnapshot<SalesRegisterWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessSalesRoute(context.permissions, "payments")) return salesAccessDenied("payments");
    const customers = await repository.listCustomers();
    return { state: "ready", data: { context, customers: customers.items, sales: [], nextCursor: null } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadSalesDocumentsWorkspace(): Promise<LiveSnapshot<OperationsWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    if (!canAccessSalesRoute(context.permissions, "documents")) return salesAccessDenied("documents");
    const [customers, products, quotations, orders] = await Promise.all([
      repository.listCustomers(),
      repository.listProducts(),
      listAllOperationDocuments(repository, "QUOTATION"),
      listAllOperationDocuments(repository, "SALES_ORDER"),
    ]);
    return {
      state: "ready",
      data: {
        context,
        customers: customers.items,
        products: products.items,
        suppliers: [],
        documents: [...quotations, ...orders].sort(newestDocumentsFirst),
        nextCursor: null,
      },
    };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadSaleDetail(saleId: string): Promise<LiveSnapshot<SaleDetailWorkspace>> {
  try {
    const repository = await createServerRepository();
    const context = await repository.getWorkingContext();
    const [customers, products, sale] = await Promise.all([
      repository.listCustomers(),
      repository.listProducts(),
      repository.getSale(saleId),
    ]);
    return { state: "ready", data: { context, customers: customers.items, products: products.items, documents: [], sale } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}
