import { safeIntegerProduct, safeIntegerSum } from "@/live-api/integer-safety";

interface CartMathLine {
  productId: string;
  quantity: number;
}

interface CartMathProduct {
  unit_price_minor: number;
}

export function cartLineTotalMinor(unitPriceMinor: number, quantity: number): number | null {
  if (!Number.isSafeInteger(unitPriceMinor) || unitPriceMinor < 0 || !Number.isSafeInteger(quantity) || quantity < 1) return null;
  return safeIntegerProduct(unitPriceMinor, quantity);
}

export function cartEstimatedTotalMinor(lines: readonly CartMathLine[], productsById: ReadonlyMap<string, CartMathProduct>): number | null {
  const lineTotals: number[] = [];
  for (const line of lines) {
    const product = productsById.get(line.productId);
    if (!product) return null;
    const lineTotal = cartLineTotalMinor(product.unit_price_minor, line.quantity);
    if (lineTotal === null) return null;
    lineTotals.push(lineTotal);
  }
  return safeIntegerSum(lineTotals);
}
