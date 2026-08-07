export function parseMajorUnitsToMinor(value: string): number | null {
  const normalized = value.trim();
  const match = /^(0|[1-9][0-9]*)(?:\.([0-9]{1,2}))?$/.exec(normalized);
  if (!match) return null;

  const whole = BigInt(match[1]);
  const fraction = BigInt((match[2] ?? "").padEnd(2, "0") || "0");
  const minor = whole * BigInt(100) + fraction;
  if (minor > BigInt(Number.MAX_SAFE_INTEGER)) return null;
  return Number(minor);
}

export function minorUnitsToMajorInput(value: number): string {
  if (!Number.isSafeInteger(value) || value < 0) return "";
  const minor = BigInt(value);
  const whole = minor / BigInt(100);
  const fraction = (minor % BigInt(100)).toString().padStart(2, "0");
  return `${whole}.${fraction}`;
}
