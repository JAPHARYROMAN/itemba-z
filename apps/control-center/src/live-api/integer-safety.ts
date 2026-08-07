const MAX_SAFE_BIGINT = BigInt(Number.MAX_SAFE_INTEGER);
const MIN_SAFE_BIGINT = BigInt(Number.MIN_SAFE_INTEGER);

export function safeIntegerSum(values: readonly number[]): number | null {
  let total = BigInt(0);
  for (const value of values) {
    if (!Number.isSafeInteger(value)) return null;
    total += BigInt(value);
  }
  if (total > MAX_SAFE_BIGINT || total < MIN_SAFE_BIGINT) return null;
  return Number(total);
}

export function safeIntegerProduct(left: number, right: number): number | null {
  if (!Number.isSafeInteger(left) || !Number.isSafeInteger(right)) return null;
  const product = BigInt(left) * BigInt(right);
  if (product > MAX_SAFE_BIGINT || product < MIN_SAFE_BIGINT) return null;
  return Number(product);
}

export function firstUnsafeIntegerPath(value: unknown, path = "$"): string | null {
  if (typeof value === "number") return Number.isSafeInteger(value) ? null : path;
  if (Array.isArray(value)) {
    for (let index = 0; index < value.length; index += 1) {
      const unsafePath = firstUnsafeIntegerPath(value[index], `${path}[${index}]`);
      if (unsafePath) return unsafePath;
    }
    return null;
  }
  if (value && typeof value === "object") {
    for (const [key, child] of Object.entries(value)) {
      const unsafePath = firstUnsafeIntegerPath(child, `${path}.${key}`);
      if (unsafePath) return unsafePath;
    }
  }
  return null;
}
