import "server-only";

import { createServerRepository } from "@/live-api/server-repository";
import { publicProblem } from "@/live-api/errors";
import type {
  CustomerAccountWorkspace, CustomerAccountsWorkspace, DeviceManagementWorkspace, LiveSnapshot, MobileReconciliationStatus, ReconciliationDetailWorkspace,
  ReconciliationWorkspace, SaleDetailWorkspace, SalesBootstrap, SalesWorkspace,
	OperationsWorkspace,
} from "@/live-api/types";

export async function loadCustomerAccounts(): Promise<LiveSnapshot<CustomerAccountsWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, customers] = await Promise.all([repository.getWorkingContext(), repository.listCustomers()]);
    return { state: "ready", data: { context, customers: customers.items } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}

export async function loadOperationsWorkspace(): Promise<LiveSnapshot<OperationsWorkspace>> {
  try {
    const repository = await createServerRepository();
    const [context, customers, products, suppliers, documents] = await Promise.all([repository.getWorkingContext(), repository.listCustomers(), repository.listProducts(), repository.listSuppliers(), repository.listOperationDocuments()]);
    return { state: "ready", data: { context, customers: customers.items, products: products.items, suppliers: suppliers.items, documents: documents.items, nextCursor: documents.next_cursor ?? null } };
  } catch (error) { return { state: "unavailable", problem: publicProblem(error) }; }
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
    const [customers, products] = await Promise.all([repository.listCustomers(), repository.listProducts()]);
    return { state: "ready", data: { context, customers: customers.items, products: products.items } };
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
    const [customers, products, sales] = await Promise.all([
      repository.listCustomers(),
      repository.listProducts(),
      repository.listSales(cursor),
    ]);
    return {
      state: "ready",
      data: {
        context,
        customers: customers.items,
        products: products.items,
        sales: sales.items,
        nextCursor: sales.next_cursor ?? null,
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
    return { state: "ready", data: { context, customers: customers.items, products: products.items, sale } };
  } catch (error) {
    return { state: "unavailable", problem: publicProblem(error) };
  }
}
