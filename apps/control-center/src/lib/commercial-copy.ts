export type CommercialLocale = "en" | "sw";

const catalog = {
  customers: { en: "Customers", sw: "Wateja" },
  customerDirectory: { en: "Customer directory", sw: "Orodha ya wateja" },
  customerAccounts: { en: "Accounts", sw: "Akaunti" },
  collections: { en: "Collections", sw: "Makusanyo" },
  suppliers: { en: "Suppliers", sw: "Wasambazaji" },
  supplierDirectory: { en: "Supplier directory", sw: "Orodha ya wasambazaji" },
  approvals: { en: "Approvals", sw: "Idhini" },
  purchasing: { en: "Purchasing", sw: "Ununuzi" },
} as const;

export type CommercialCopyKey = keyof typeof catalog;

export function commercialCopy(locale: string, key: CommercialCopyKey): string {
  return catalog[key][locale.startsWith("sw") ? "sw" : "en"];
}
