import type { Sale } from "@/live-api/types";
import { safeIntegerSum } from "@/live-api/integer-safety";

type SaleMetricInput = Pick<Sale, "record_type" | "total_minor">;

export function grossOriginalSaleMinor(sales: readonly SaleMetricInput[]): number | null {
  return safeIntegerSum(sales.flatMap((sale) => sale.record_type === "SALE" ? [sale.total_minor] : []));
}
