import "server-only";

import { createServerRepository } from "@/live-api/server-repository";
import { publicProblem } from "@/live-api/errors";
import type { LiveSnapshot, SaleDetailWorkspace, SalesBootstrap, SalesWorkspace } from "@/live-api/types";

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
