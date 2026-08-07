const catalog = {
  purchases: { en: "Purchases", sw: "Manunuzi" },
  overview: { en: "Overview", sw: "Muhtasari" },
  requests: { en: "Requests", sw: "Maombi" },
  sourcing: { en: "Sourcing", sw: "Utafutaji wa bei" },
  orders: { en: "Orders", sw: "Oda" },
  receipts: { en: "Receipts", sw: "Mapokezi" },
  bills: { en: "Bills", sw: "Ankara" },
  payments: { en: "Payments", sw: "Malipo" },
  returns: { en: "Returns", sw: "Marejesho" },
} as const;

export type PurchaseCopyKey = keyof typeof catalog;

export function purchaseCopy(locale: string, key: PurchaseCopyKey): string {
  return catalog[key][locale.startsWith("sw") ? "sw" : "en"];
}
