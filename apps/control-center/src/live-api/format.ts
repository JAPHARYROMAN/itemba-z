import type { Locale } from "@/domain/erp";

export const TZS_MINOR_UNITS_PER_MAJOR = BigInt(100);

const integerFormatters: Record<Locale, Intl.NumberFormat> = {
  en: new Intl.NumberFormat("en-TZ", { useGrouping: true, maximumFractionDigits: 0 }),
  sw: new Intl.NumberFormat("sw-TZ", { useGrouping: true, maximumFractionDigits: 0 }),
};

export function formatMinorUnits(value: number, currency: string, locale: Locale): string {
  if (currency !== "TZS") throw new RangeError(`Unsupported minor-unit currency: ${currency}`);
  if (!Number.isSafeInteger(value)) throw new RangeError("Minor-unit value must be a JavaScript safe integer.");

  const minorValue = BigInt(value);
  const negative = minorValue < BigInt(0);
  const absoluteValue = negative ? -minorValue : minorValue;
  const majorUnits = absoluteValue / TZS_MINOR_UNITS_PER_MAJOR;
  const fractionalUnits = (absoluteValue % TZS_MINOR_UNITS_PER_MAJOR).toString().padStart(2, "0");
  const groupedMajorUnits = integerFormatters[locale].format(majorUnits);
  return `${negative ? "-" : ""}TZS ${groupedMajorUnits}.${fractionalUnits}`;
}

export function formatTimestamp(value: string, locale: Locale, timeZone = "Africa/Dar_es_Salaam"): string {
  return new Intl.DateTimeFormat(locale === "sw" ? "sw-TZ" : "en-TZ", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone,
  }).format(new Date(value));
}

export function compactId(value: string): string {
  return value.length > 12 ? `${value.slice(0, 8)}…${value.slice(-4)}` : value;
}
