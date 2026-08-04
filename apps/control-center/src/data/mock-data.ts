import type {
  DashboardData,
  FormField,
  ModuleData,
  ModuleKey,
  ModuleRecord,
  Status,
  StatusTone,
} from "@/domain/erp";
import { text } from "@/lib/i18n";

const status = (en: string, sw: string, tone: StatusTone): Status => ({ label: text(en, sw), tone });

const standardControls = (approval = "Approved") => [
  {
    label: text("Company & branch scope", "Upeo wa kampuni na tawi"),
    detail: text("Verified against the active operating context", "Imethibitishwa kwa muktadha wa shughuli"),
    status: status("Passed", "Imepita", "success"),
  },
  {
    label: text("Approval policy", "Sera ya idhini"),
    detail: text(approval, approval === "Approved" ? "Imeidhinishwa" : "Inasubiri idhini"),
    status: approval === "Approved" ? status("Passed", "Imepita", "success") : status("Pending", "Inasubiri", "warning"),
  },
];

const timeline = (creator: string, final = "Posted") => [
  { title: text("Created", "Imeundwa"), detail: creator, time: text("Today, 08:42", "Leo, 08:42"), tone: "info" as const },
  { title: text("Validated", "Imethibitishwa"), detail: "System controls", time: text("Today, 08:44", "Leo, 08:44"), tone: "success" as const },
  { title: text(final, final === "Posted" ? "Imechapishwa" : "Imesasishwa"), detail: "Amina Msuya", time: text("Today, 09:06", "Leo, 09:06"), tone: "success" as const },
];

const fields = (...items: Array<[string, string, string, string, FormField["type"], boolean?]>): FormField[] =>
  items.map(([name, en, sw, placeholder, type, required]) => ({
    name,
    label: text(en, sw),
    placeholder: text(placeholder, placeholder),
    type,
    required,
  }));

const customerRecords: ModuleRecord[] = [
  {
    id: "CUST-00184",
    primary: text("Kijiji Supermarket Ltd", "Kijiji Supermarket Ltd"),
    secondary: "TIN 118-442-901 · Wholesale",
    values: { balance: "TZS 12.4m", segment: "Wholesale", branch: "Dar es Salaam HQ" },
    status: status("Active", "Hai", "success"),
    updated: text("18 min ago", "Dakika 18 zilizopita"),
    details: [
      { label: text("Credit limit", "Kikomo cha mkopo"), value: "TZS 20,000,000" },
      { label: text("Payment terms", "Masharti ya malipo"), value: "Net 30" },
      { label: text("Account owner", "Msimamizi wa akaunti"), value: "Neema Kweka" },
      { label: text("Last sale", "Mauzo ya mwisho"), value: "03 Aug 2026" },
    ],
    controls: standardControls(),
    timeline: timeline("Neema Kweka", "Updated"),
  },
  {
    id: "CUST-00231",
    primary: text("Mlimani Mini Mart", "Mlimani Mini Mart"),
    secondary: "TIN 146-882-330 · Retail",
    values: { balance: "TZS 1.8m", segment: "Retail", branch: "Arusha" },
    status: status("Review", "Kagua", "warning"),
    updated: text("Yesterday", "Jana"),
    details: [
      { label: text("Credit limit", "Kikomo cha mkopo"), value: "TZS 3,000,000" },
      { label: text("Payment terms", "Masharti ya malipo"), value: "Net 14" },
      { label: text("Account owner", "Msimamizi wa akaunti"), value: "Yusuf Hamisi" },
      { label: text("Last sale", "Mauzo ya mwisho"), value: "29 Jul 2026" },
    ],
    controls: standardControls("Credit review pending"),
    timeline: timeline("Yusuf Hamisi", "Updated"),
  },
  {
    id: "CUST-00309",
    primary: text("Upendo General Traders", "Upendo General Traders"),
    secondary: "Cash customer · Retail",
    values: { balance: "TZS 0", segment: "Cash retail", branch: "Mwanza" },
    status: status("Active", "Hai", "success"),
    updated: text("2 days ago", "Siku 2 zilizopita"),
    details: [
      { label: text("Credit limit", "Kikomo cha mkopo"), value: "Cash only" },
      { label: text("Payment terms", "Masharti ya malipo"), value: "Due on receipt" },
      { label: text("Account owner", "Msimamizi wa akaunti"), value: "Rehema Joseph" },
      { label: text("Last sale", "Mauzo ya mwisho"), value: "02 Aug 2026" },
    ],
    controls: standardControls(),
    timeline: timeline("Rehema Joseph", "Updated"),
  },
];

const supplierRecords: ModuleRecord[] = [
  {
    id: "SUP-00047",
    primary: text("Mwanza Packaging Works", "Mwanza Packaging Works"),
    secondary: "TIN 104-708-221 · Packaging",
    values: { payable: "TZS 22.6m", category: "Packaging", leadTime: "5 days" },
    status: status("Active", "Hai", "success"),
    updated: text("Today", "Leo"),
    details: [
      { label: text("Payment terms", "Masharti ya malipo"), value: "Net 30" },
      { label: text("Open purchase orders", "Oda za manunuzi wazi"), value: "3" },
      { label: text("On-time delivery", "Uwasilishaji kwa wakati"), value: "94%" },
      { label: text("Buyer", "Mnunuzi"), value: "Godfrey Mushi" },
    ], controls: standardControls(), timeline: timeline("Godfrey Mushi", "Updated"),
  },
  {
    id: "SUP-00112",
    primary: text("Kilimanjaro Foods Supply", "Kilimanjaro Foods Supply"),
    secondary: "TIN 127-330-846 · Food ingredients",
    values: { payable: "TZS 8.1m", category: "Ingredients", leadTime: "3 days" },
    status: status("Active", "Hai", "success"),
    updated: text("Yesterday", "Jana"),
    details: [
      { label: text("Payment terms", "Masharti ya malipo"), value: "Net 14" },
      { label: text("Open purchase orders", "Oda za manunuzi wazi"), value: "1" },
      { label: text("On-time delivery", "Uwasilishaji kwa wakati"), value: "97%" },
      { label: text("Buyer", "Mnunuzi"), value: "Godfrey Mushi" },
    ], controls: standardControls(), timeline: timeline("Godfrey Mushi", "Updated"),
  },
  {
    id: "SUP-00158",
    primary: text("Bahari Logistics Tanzania", "Bahari Logistics Tanzania"),
    secondary: "TIN 139-601-772 · Logistics",
    values: { payable: "TZS 3.4m", category: "Logistics", leadTime: "2 days" },
    status: status("Documents due", "Nyaraka zinahitajika", "warning"),
    updated: text("3 days ago", "Siku 3 zilizopita"),
    details: [
      { label: text("Payment terms", "Masharti ya malipo"), value: "Net 30" },
      { label: text("Open purchase orders", "Oda za manunuzi wazi"), value: "2" },
      { label: text("On-time delivery", "Uwasilishaji kwa wakati"), value: "89%" },
      { label: text("Buyer", "Mnunuzi"), value: "Farida Ali" },
    ], controls: standardControls("Tax clearance renewal due"), timeline: timeline("Farida Ali", "Updated"),
  },
];

const saleRecords: ModuleRecord[] = [
  {
    id: "SO-2026-01428",
    primary: text("Kijiji Supermarket Ltd", "Kijiji Supermarket Ltd"),
    secondary: "Sales order · 12 lines",
    values: { amount: "TZS 8.46m", payment: "Credit · Net 30", branch: "Dar es Salaam HQ" },
    status: status("Ready to invoice", "Tayari kutolewa ankara", "info"),
    updated: text("8 min ago", "Dakika 8 zilizopita"),
    details: [
      { label: text("Order date", "Tarehe ya oda"), value: "04 Aug 2026" },
      { label: text("Warehouse", "Ghala"), value: "DSM Central" },
      { label: text("Salesperson", "Muuzaji"), value: "Neema Kweka" },
      { label: text("Tax", "Kodi"), value: "TZS 1,290,712" },
    ],
    ledgerImpact: [
      { account: "1100 · Trade receivables", debit: "8,461,000", credit: "—" },
      { account: "4100 · Product revenue", debit: "—", credit: "7,170,288" },
      { account: "2205 · VAT output", debit: "—", credit: "1,290,712" },
    ], controls: standardControls(), timeline: timeline("Neema Kweka"),
  },
  {
    id: "SO-2026-01427",
    primary: text("Upendo General Traders", "Upendo General Traders"),
    secondary: "POS sale · 5 lines",
    values: { amount: "TZS 1.28m", payment: "Cash", branch: "Mwanza" },
    status: status("Paid", "Imelipwa", "success"),
    updated: text("24 min ago", "Dakika 24 zilizopita"),
    details: [
      { label: text("Order date", "Tarehe ya oda"), value: "04 Aug 2026" },
      { label: text("Warehouse", "Ghala"), value: "Mwanza Store" },
      { label: text("Salesperson", "Muuzaji"), value: "Rehema Joseph" },
      { label: text("Fiscal receipt", "Risiti ya fiskali"), value: "RCP-668201" },
    ], ledgerImpact: [
      { account: "1010 · Cash on hand", debit: "1,280,000", credit: "—" },
      { account: "4100 · Product revenue", debit: "—", credit: "1,084,746" },
      { account: "2205 · VAT output", debit: "—", credit: "195,254" },
    ], controls: standardControls(), timeline: timeline("Rehema Joseph"),
  },
  {
    id: "SO-2026-01421",
    primary: text("Mlimani Mini Mart", "Mlimani Mini Mart"),
    secondary: "Sales order · 8 lines",
    values: { amount: "TZS 3.92m", payment: "Credit · Net 14", branch: "Arusha" },
    status: status("On hold", "Imesimamishwa", "warning"),
    updated: text("2 hours ago", "Saa 2 zilizopita"),
    details: [
      { label: text("Order date", "Tarehe ya oda"), value: "04 Aug 2026" },
      { label: text("Warehouse", "Ghala"), value: "Arusha Depot" },
      { label: text("Salesperson", "Muuzaji"), value: "Yusuf Hamisi" },
      { label: text("Hold reason", "Sababu ya kusimamisha"), value: "Credit review" },
    ], controls: standardControls("Credit approval pending"), timeline: timeline("Yusuf Hamisi", "Updated"),
  },
];

const purchaseRecords: ModuleRecord[] = [
  {
    id: "PO-2026-00412",
    primary: text("Mwanza Packaging Works", "Mwanza Packaging Works"), secondary: "Purchase order · 6 lines",
    values: { amount: "TZS 14.8m", delivery: "08 Aug 2026", branch: "Dar es Salaam HQ" },
    status: status("Approval needed", "Idhini inahitajika", "warning"), updated: text("32 min ago", "Dakika 32 zilizopita"),
    details: [{ label: text("Buyer", "Mnunuzi"), value: "Godfrey Mushi" }, { label: text("Destination", "Mahali pa kupeleka"), value: "DSM Central" }, { label: text("Terms", "Masharti"), value: "Net 30" }, { label: text("Budget available", "Bajeti iliyopo"), value: "TZS 24.2m" }],
    controls: standardControls("Director approval pending"), timeline: timeline("Godfrey Mushi", "Updated"),
  },
  {
    id: "PO-2026-00409",
    primary: text("Kilimanjaro Foods Supply", "Kilimanjaro Foods Supply"), secondary: "Purchase order · 4 lines",
    values: { amount: "TZS 6.25m", delivery: "05 Aug 2026", branch: "Arusha" },
    status: status("Confirmed", "Imethibitishwa", "success"), updated: text("Yesterday", "Jana"),
    details: [{ label: text("Buyer", "Mnunuzi"), value: "Godfrey Mushi" }, { label: text("Destination", "Mahali pa kupeleka"), value: "Arusha Depot" }, { label: text("Terms", "Masharti"), value: "Net 14" }, { label: text("Budget available", "Bajeti iliyopo"), value: "TZS 11.8m" }],
    controls: standardControls(), timeline: timeline("Godfrey Mushi"),
  },
  {
    id: "PO-2026-00403",
    primary: text("Bahari Logistics Tanzania", "Bahari Logistics Tanzania"), secondary: "Service order · 2 lines",
    values: { amount: "TZS 3.4m", delivery: "04 Aug 2026", branch: "Mwanza" },
    status: status("Part received", "Sehemu imepokelewa", "info"), updated: text("2 days ago", "Siku 2 zilizopita"),
    details: [{ label: text("Buyer", "Mnunuzi"), value: "Farida Ali" }, { label: text("Destination", "Mahali pa kupeleka"), value: "Mwanza Store" }, { label: text("Terms", "Masharti"), value: "Net 30" }, { label: text("Received", "Iliyopokelewa"), value: "62%" }],
    controls: standardControls(), timeline: timeline("Farida Ali", "Updated"),
  },
];

const inventoryRecords: ModuleRecord[] = [
  {
    id: "SKU-BEV-0018", primary: text("Itemba Pure Drinking Water 1.5L", "Maji Safi ya Itemba 1.5L"), secondary: "BEV-0018 · Case of 12",
    values: { onHand: "1,842", allocated: "318", reorder: "600" }, status: status("Healthy", "Nzuri", "success"), updated: text("6 min ago", "Dakika 6 zilizopita"),
    details: [{ label: text("Warehouse", "Ghala"), value: "DSM Central" }, { label: text("Available", "Inayopatikana"), value: "1,524 cases" }, { label: text("Average cost", "Gharama wastani"), value: "TZS 7,860" }, { label: text("Stock value", "Thamani ya bidhaa"), value: "TZS 14.48m" }], controls: standardControls(), timeline: timeline("System sync", "Updated"),
  },
  {
    id: "SKU-FOD-0042", primary: text("Premium Maize Flour 2kg", "Unga wa Mahindi Bora 2kg"), secondary: "FOD-0042 · Bale of 10",
    values: { onHand: "214", allocated: "146", reorder: "180" }, status: status("Low stock", "Bidhaa chache", "warning"), updated: text("12 min ago", "Dakika 12 zilizopita"),
    details: [{ label: text("Warehouse", "Ghala"), value: "Arusha Depot" }, { label: text("Available", "Inayopatikana"), value: "68 bales" }, { label: text("Average cost", "Gharama wastani"), value: "TZS 18,420" }, { label: text("Incoming", "Zinazokuja"), value: "300 · 05 Aug" }], controls: standardControls(), timeline: timeline("Inventory worker", "Updated"),
  },
  {
    id: "SKU-HOM-0091", primary: text("Multipurpose Liquid Soap 5L", "Sabuni ya Maji 5L"), secondary: "HOM-0091 · Each",
    values: { onHand: "38", allocated: "31", reorder: "40" }, status: status("Reorder now", "Agiza sasa", "danger"), updated: text("21 min ago", "Dakika 21 zilizopita"),
    details: [{ label: text("Warehouse", "Ghala"), value: "Mwanza Store" }, { label: text("Available", "Inayopatikana"), value: "7 units" }, { label: text("Average cost", "Gharama wastani"), value: "TZS 12,640" }, { label: text("Incoming", "Zinazokuja"), value: "None" }], controls: standardControls("Replenishment approval pending"), timeline: timeline("Inventory worker", "Updated"),
  },
];

const financeRecords: ModuleRecord[] = [
  {
    id: "JV-2026-00861", primary: text("Daily POS settlement", "Makazi ya kila siku ya POS"), secondary: "Sales · Dar es Salaam HQ",
    values: { date: "04 Aug 2026", debit: "TZS 18.24m", credit: "TZS 18.24m" }, status: status("Posted", "Imechapishwa", "success"), updated: text("14 min ago", "Dakika 14 zilizopita"),
    details: [{ label: text("Fiscal period", "Kipindi cha fedha"), value: "Aug 2026" }, { label: text("Source", "Chanzo"), value: "POS settlement" }, { label: text("Lines", "Mistari"), value: "14" }, { label: text("Posted by", "Aliyechapisha"), value: "Amina Msuya" }], ledgerImpact: [{ account: "1020 · Mobile money clearing", debit: "18,240,000", credit: "—" }, { account: "1105 · POS receivable", debit: "—", credit: "18,240,000" }], controls: standardControls(), timeline: timeline("System worker"),
  },
  {
    id: "JV-2026-00860", primary: text("Supplier invoice · MPW-8841", "Ankara ya msambazaji · MPW-8841"), secondary: "Purchases · Mwanza Packaging Works",
    values: { date: "04 Aug 2026", debit: "TZS 6.70m", credit: "TZS 6.70m" }, status: status("Posted", "Imechapishwa", "success"), updated: text("39 min ago", "Dakika 39 zilizopita"),
    details: [{ label: text("Fiscal period", "Kipindi cha fedha"), value: "Aug 2026" }, { label: text("Source", "Chanzo"), value: "Supplier invoice" }, { label: text("Lines", "Mistari"), value: "4" }, { label: text("Posted by", "Aliyechapisha"), value: "Amina Msuya" }], ledgerImpact: [{ account: "1300 · Inventory", debit: "5,677,966", credit: "—" }, { account: "2210 · VAT input", debit: "1,022,034", credit: "—" }, { account: "2100 · Trade payables", debit: "—", credit: "6,700,000" }], controls: standardControls(), timeline: timeline("Amina Msuya"),
  },
  {
    id: "JV-2026-00858", primary: text("Bank charges · July sweep", "Makato ya benki · Julai"), secondary: "Treasury · CRDB operating account",
    values: { date: "03 Aug 2026", debit: "TZS 482k", credit: "TZS 482k" }, status: status("Review", "Kagua", "warning"), updated: text("Yesterday", "Jana"),
    details: [{ label: text("Fiscal period", "Kipindi cha fedha"), value: "Aug 2026" }, { label: text("Source", "Chanzo"), value: "Bank import" }, { label: text("Lines", "Mistari"), value: "2" }, { label: text("Prepared by", "Aliyeandaa"), value: "Kelvin Maro" }], ledgerImpact: [{ account: "6205 · Bank charges", debit: "482,000", credit: "—" }, { account: "1005 · CRDB operating", debit: "—", credit: "482,000" }], controls: standardControls("Controller review pending"), timeline: timeline("Kelvin Maro", "Updated"),
  },
];

const hrRecords: ModuleRecord[] = [
  { id: "EMP-0024", primary: text("Rehema Joseph", "Rehema Joseph"), secondary: "Sales attendant · Mwanza", values: { department: "Sales", attendance: "Present · 07:54", payroll: "Ready" }, status: status("Active", "Hai", "success"), updated: text("Today", "Leo"), details: [{ label: text("Employee no.", "Namba ya mfanyakazi"), value: "EMP-0024" }, { label: text("Start date", "Tarehe ya kuanza"), value: "12 Feb 2023" }, { label: text("Manager", "Meneja"), value: "David Mwita" }, { label: text("Leave balance", "Salio la likizo"), value: "14 days" }], controls: standardControls(), timeline: timeline("HR administrator", "Updated") },
  { id: "EMP-0041", primary: text("Yusuf Hamisi", "Yusuf Hamisi"), secondary: "Account manager · Arusha", values: { department: "Commercial", attendance: "Present · 08:11", payroll: "Ready" }, status: status("Active", "Hai", "success"), updated: text("Today", "Leo"), details: [{ label: text("Employee no.", "Namba ya mfanyakazi"), value: "EMP-0041" }, { label: text("Start date", "Tarehe ya kuanza"), value: "08 May 2024" }, { label: text("Manager", "Meneja"), value: "Neema Kweka" }, { label: text("Leave balance", "Salio la likizo"), value: "9 days" }], controls: standardControls(), timeline: timeline("HR administrator", "Updated") },
  { id: "EMP-0063", primary: text("Kelvin Maro", "Kelvin Maro"), secondary: "Treasury officer · Dar es Salaam", values: { department: "Finance", attendance: "Remote", payroll: "Exception" }, status: status("Review", "Kagua", "warning"), updated: text("Yesterday", "Jana"), details: [{ label: text("Employee no.", "Namba ya mfanyakazi"), value: "EMP-0063" }, { label: text("Start date", "Tarehe ya kuanza"), value: "17 Jan 2025" }, { label: text("Manager", "Meneja"), value: "Amina Msuya" }, { label: text("Payroll exception", "Tatizo la malipo"), value: "Loan deduction review" }], controls: standardControls("Payroll review pending"), timeline: timeline("HR administrator", "Updated") },
];

const reportRecords: ModuleRecord[] = [
  { id: "RPT-SALES-01", primary: text("Daily sales summary", "Muhtasari wa mauzo ya kila siku"), secondary: "Commercial performance", values: { owner: "Finance", cadence: "Daily", lastRun: "04 Aug · 09:15" }, status: status("Ready", "Tayari", "success"), updated: text("Just now", "Sasa hivi"), details: [{ label: text("Data through", "Data hadi"), value: "04 Aug 2026 · 09:10" }, { label: text("Format", "Muundo"), value: "Interactive · PDF · XLSX" }, { label: text("Recipients", "Wapokeaji"), value: "8" }, { label: text("Next schedule", "Ratiba ijayo"), value: "05 Aug · 06:30" }], controls: standardControls(), timeline: timeline("Reporting worker", "Updated") },
  { id: "RPT-STOCK-04", primary: text("Stock valuation by warehouse", "Thamani ya bidhaa kwa ghala"), secondary: "Inventory & costing", values: { owner: "Operations", cadence: "Weekly", lastRun: "03 Aug · 18:00" }, status: status("Ready", "Tayari", "success"), updated: text("Yesterday", "Jana"), details: [{ label: text("Data through", "Data hadi"), value: "03 Aug 2026 · 17:45" }, { label: text("Format", "Muundo"), value: "Interactive · XLSX" }, { label: text("Recipients", "Wapokeaji"), value: "5" }, { label: text("Next schedule", "Ratiba ijayo"), value: "10 Aug · 18:00" }], controls: standardControls(), timeline: timeline("Reporting worker", "Updated") },
  { id: "RPT-FIN-09", primary: text("Trial balance", "Mizania ya majaribio"), secondary: "Financial statements", values: { owner: "Finance", cadence: "On demand", lastRun: "04 Aug · 08:20" }, status: status("1 exception", "Tatizo 1", "warning"), updated: text("55 min ago", "Dakika 55 zilizopita"), details: [{ label: text("Data through", "Data hadi"), value: "04 Aug 2026 · 08:15" }, { label: text("Format", "Muundo"), value: "Interactive · PDF · XLSX" }, { label: text("Exception", "Tatizo"), value: "JV-2026-00858 pending" }, { label: text("Prepared for", "Imeandaliwa kwa"), value: "August close" }], controls: standardControls("Journal review pending"), timeline: timeline("Amina Msuya", "Updated") },
];

const settingsRecords: ModuleRecord[] = [
  { id: "CFG-ORG-01", primary: text("Organisation structure", "Muundo wa shirika"), secondary: "Tenant · companies · branches · warehouses", values: { owner: "System admin", scope: "Global", changed: "28 Jul 2026" }, status: status("Configured", "Imesanidiwa", "success"), updated: text("7 days ago", "Siku 7 zilizopita"), details: [{ label: text("Legal companies", "Kampuni za kisheria"), value: "2" }, { label: text("Branches", "Matawi"), value: "4" }, { label: text("Warehouses", "Maghala"), value: "6" }, { label: text("Last changed by", "Aliyebadilisha mwisho"), value: "System administrator" }], controls: standardControls(), timeline: timeline("System administrator", "Updated") },
  { id: "CFG-ACC-04", primary: text("Roles & permissions", "Majukumu na ruhusa"), secondary: "Access control · approval limits", values: { owner: "Security admin", scope: "Global", changed: "02 Aug 2026" }, status: status("Review due", "Ukaguzi unahitajika", "warning"), updated: text("2 days ago", "Siku 2 zilizopita"), details: [{ label: text("Active roles", "Majukumu hai"), value: "18" }, { label: text("Assigned users", "Watumiaji waliopangiwa"), value: "74" }, { label: text("Review cycle", "Mzunguko wa ukaguzi"), value: "Quarterly" }, { label: text("Next review", "Ukaguzi ujao"), value: "05 Aug 2026" }], controls: standardControls("Quarterly access review due"), timeline: timeline("Security administrator", "Updated") },
  { id: "CFG-TAX-02", primary: text("Taxes & fiscal devices", "Kodi na vifaa vya fiskali"), secondary: "VAT · EFD/VFD · receipt numbering", values: { owner: "Finance admin", scope: "Tanzania Mainland", changed: "01 Aug 2026" }, status: status("Connected", "Imeunganishwa", "success"), updated: text("3 days ago", "Siku 3 zilizopita"), details: [{ label: text("Tax profiles", "Wasifu wa kodi"), value: "4" }, { label: text("Registered devices", "Vifaa vilivyosajiliwa"), value: "7" }, { label: text("API status", "Hali ya API"), value: "Healthy" }, { label: text("Certificate expiry", "Mwisho wa cheti"), value: "18 Mar 2027" }], controls: standardControls(), timeline: timeline("Finance administrator", "Updated") },
];

const commonFormFields = fields(
  ["name", "Name / reference", "Jina / rejea", "Enter a clear name or reference", "text", true],
  ["date", "Effective date", "Tarehe ya kuanza", "", "date", true],
  ["branch", "Branch", "Tawi", "Dar es Salaam HQ", "text", true],
  ["notes", "Notes", "Maelezo", "Add context for reviewers", "textarea"]
);

export const modules: Record<ModuleKey, ModuleData> = {
  customers: { key: "customers", title: text("Customers", "Wateja"), singular: text("customer", "mteja"), eyebrow: text("Commercial", "Biashara"), description: text("Customer accounts, credit exposure and relationship health.", "Akaunti za wateja, mikopo na afya ya mahusiano."), primaryAction: text("Add customer", "Ongeza mteja"), metrics: [{ label: text("Active customers", "Wateja hai"), value: "1,284", detail: text("Across 4 branches", "Katika matawi 4"), change: "+3.8%", direction: "up" }, { label: text("Receivables", "Madeni ya kupokea"), value: "TZS 18.2m", detail: text("7 accounts overdue", "Akaunti 7 zimechelewa"), change: "-5.2%", direction: "down" }, { label: text("Credit utilisation", "Matumizi ya mkopo"), value: "63%", detail: text("Within policy", "Ndani ya sera"), change: "+1.1%", direction: "flat" }, { label: text("Collection rate", "Kiwango cha makusanyo"), value: "94.6%", detail: text("Rolling 30 days", "Siku 30"), change: "+2.4%", direction: "up" }], columns: [{ key: "balance", label: text("Balance", "Salio"), align: "right" }, { key: "segment", label: text("Segment", "Kundi") }, { key: "branch", label: text("Branch", "Tawi") }], records: customerRecords, insight: { title: text("Credit exposure is improving", "Hali ya mikopo inaimarika"), detail: text("Collections reduced overdue receivables by TZS 1.0m this week.", "Makusanyo yamepunguza madeni yaliyochelewa kwa TZS 1.0m wiki hii."), tone: "success" }, formFields: fields(["name", "Legal / trading name", "Jina la kisheria / biashara", "e.g. Kijiji Supermarket Ltd", "text", true], ["tin", "TIN", "TIN", "000-000-000", "text", true], ["phone", "Phone number", "Namba ya simu", "+255…", "text", true], ["credit", "Credit limit (TZS)", "Kikomo cha mkopo (TZS)", "0", "number"], ["notes", "Notes", "Maelezo", "Commercial context", "textarea"] ) },
  suppliers: { key: "suppliers", title: text("Suppliers", "Wasambazaji"), singular: text("supplier", "msambazaji"), eyebrow: text("Procurement", "Manunuzi"), description: text("Supplier performance, obligations and procurement relationships.", "Utendaji wa wasambazaji, wajibu na mahusiano ya manunuzi."), primaryAction: text("Add supplier", "Ongeza msambazaji"), metrics: [{ label: text("Active suppliers", "Wasambazaji hai"), value: "86", detail: text("12 strategic", "12 wa kimkakati") }, { label: text("Payables", "Madeni ya kulipa"), value: "TZS 34.1m", detail: text("TZS 5.6m due this week", "TZS 5.6m wiki hii") }, { label: text("On-time delivery", "Uwasilishaji kwa wakati"), value: "92.8%", detail: text("Last 90 days", "Siku 90") }, { label: text("Documents due", "Nyaraka zinahitajika"), value: "3", detail: text("Compliance renewals", "Usasishaji wa utii") }], columns: [{ key: "payable", label: text("Payable", "Deni"), align: "right" }, { key: "category", label: text("Category", "Aina") }, { key: "leadTime", label: text("Lead time", "Muda wa kuwasili") }], records: supplierRecords, insight: { title: text("One supplier needs compliance review", "Msambazaji mmoja anahitaji ukaguzi"), detail: text("Bahari Logistics tax clearance expires before the next payment run.", "Kibali cha kodi cha Bahari Logistics kinaisha kabla ya malipo yajayo."), tone: "warning" }, formFields: commonFormFields },
  sales: { key: "sales", title: text("Sales", "Mauzo"), singular: text("sales order", "oda ya mauzo"), eyebrow: text("Order to cash", "Oda hadi malipo"), description: text("Orders, invoices, returns, fiscal receipts and collections.", "Oda, ankara, marejesho, risiti za fiskali na makusanyo."), primaryAction: text("New sales order", "Oda mpya ya mauzo"), metrics: [{ label: text("Net sales today", "Mauzo halisi leo"), value: "TZS 84.6m", detail: text("246 transactions", "Miamala 246"), change: "+12.4%", direction: "up" }, { label: text("Gross margin", "Faida ghafi"), value: "28.7%", detail: text("Target 27.5%", "Lengo 27.5%"), change: "+1.2 pts", direction: "up" }, { label: text("Open orders", "Oda wazi"), value: "42", detail: text("12 await action", "12 zinasubiri hatua") }, { label: text("Returns", "Marejesho"), value: "1.6%", detail: text("Within threshold", "Ndani ya kiwango"), change: "-0.3 pts", direction: "down" }], columns: [{ key: "amount", label: text("Amount", "Kiasi"), align: "right" }, { key: "payment", label: text("Payment", "Malipo") }, { key: "branch", label: text("Branch", "Tawi") }], records: saleRecords, insight: { title: text("Sales are 8.2% ahead of plan", "Mauzo ni 8.2% juu ya mpango"), detail: text("Dar es Salaam wholesale is driving today’s uplift.", "Jumla ya Dar es Salaam inaongoza ongezeko la leo."), tone: "success" }, formFields: fields(["customer", "Customer", "Mteja", "Search customer", "text", true], ["date", "Order date", "Tarehe ya oda", "", "date", true], ["warehouse", "Fulfilment warehouse", "Ghala la utoaji", "DSM Central", "text", true], ["reference", "Customer reference", "Rejea ya mteja", "Optional PO or note", "text"], ["notes", "Delivery notes", "Maelezo ya usafirishaji", "Instructions for fulfilment", "textarea"]) },
  purchases: { key: "purchases", title: text("Purchases", "Manunuzi"), singular: text("purchase order", "oda ya manunuzi"), eyebrow: text("Procure to pay", "Manunuzi hadi malipo"), description: text("Requisitions, orders, receipts, invoices and supplier returns.", "Maombi, oda, mapokezi, ankara na marejesho kwa wasambazaji."), primaryAction: text("New purchase order", "Oda mpya ya manunuzi"), metrics: [{ label: text("Open commitments", "Ahadi wazi"), value: "TZS 48.7m", detail: text("19 purchase orders", "Oda 19") }, { label: text("Awaiting approval", "Zinasubiri idhini"), value: "6", detail: text("TZS 21.4m total", "Jumla TZS 21.4m") }, { label: text("Due this week", "Zinatarajiwa wiki hii"), value: "11", detail: text("4 branches", "Matawi 4") }, { label: text("Receipt variance", "Tofauti za mapokezi"), value: "1.2%", detail: text("Below 2% threshold", "Chini ya kiwango 2%") }], columns: [{ key: "amount", label: text("Amount", "Kiasi"), align: "right" }, { key: "delivery", label: text("Expected", "Inatarajiwa") }, { key: "branch", label: text("Branch", "Tawi") }], records: purchaseRecords, insight: { title: text("Approval ageing needs attention", "Muda wa idhini unahitaji umakini"), detail: text("Two purchase orders have waited more than 24 hours.", "Oda mbili zimesubiri zaidi ya saa 24."), tone: "warning" }, formFields: commonFormFields },
  inventory: { key: "inventory", title: text("Inventory", "Bidhaa"), singular: text("stock item", "bidhaa"), eyebrow: text("Stock & fulfilment", "Bidhaa na utoaji"), description: text("Stock position, movements, valuation and replenishment.", "Hali ya bidhaa, miondoko, thamani na uagizaji upya."), primaryAction: text("Record stock movement", "Rekodi mwondoko wa bidhaa"), metrics: [{ label: text("Stock value", "Thamani ya bidhaa"), value: "TZS 146.8m", detail: text("Across 6 warehouses", "Katika maghala 6"), change: "+2.1%", direction: "up" }, { label: text("Available SKUs", "SKU zinazopatikana"), value: "2,416", detail: text("98.2% active", "98.2% hai") }, { label: text("Reorder alerts", "Tahadhari za kuagiza"), value: "4", detail: text("1 critical", "1 muhimu") }, { label: text("Inventory turns", "Mizunguko ya bidhaa"), value: "7.4×", detail: text("Rolling 12 months", "Miezi 12") }], columns: [{ key: "onHand", label: text("On hand", "Iliyopo"), align: "right" }, { key: "allocated", label: text("Allocated", "Iliyotengwa"), align: "right" }, { key: "reorder", label: text("Reorder point", "Kiwango cha kuagiza"), align: "right" }], records: inventoryRecords, insight: { title: text("One SKU risks a stockout today", "SKU moja inaweza kuisha leo"), detail: text("Liquid soap in Mwanza has 7 available units and no incoming supply.", "Sabuni ya maji Mwanza ina vipande 7 bila mzigo unaokuja."), tone: "danger" }, formFields: commonFormFields },
  finance: { key: "finance", title: text("Finance", "Fedha"), singular: text("journal", "jarida"), eyebrow: text("Record to report", "Rekodi hadi ripoti"), description: text("General ledger, cash, banking, receivables and payables.", "Daftari kuu, fedha, benki, madeni ya kupokea na kulipa."), primaryAction: text("New journal", "Jarida jipya"), metrics: [{ label: text("Cash position", "Hali ya fedha"), value: "TZS 62.7m", detail: text("Bank & mobile money", "Benki na simu") }, { label: text("Receivables", "Madeni ya kupokea"), value: "TZS 18.2m", detail: text("TZS 2.7m overdue", "TZS 2.7m yamechelewa") }, { label: text("Payables", "Madeni ya kulipa"), value: "TZS 34.1m", detail: text("TZS 5.6m due", "TZS 5.6m yanadaiwa") }, { label: text("Unreconciled", "Haijapatanishwa"), value: "7", detail: text("3 bank accounts", "Akaunti 3 za benki") }], columns: [{ key: "date", label: text("Date", "Tarehe") }, { key: "debit", label: text("Debit", "Debiti"), align: "right" }, { key: "credit", label: text("Credit", "Krediti"), align: "right" }], records: financeRecords, insight: { title: text("August close is 76% ready", "Kufunga Agosti ni 76% tayari"), detail: text("Bank reconciliation and one journal review remain open.", "Upatanisho wa benki na ukaguzi wa jarida moja bado uko wazi."), tone: "info" }, formFields: commonFormFields },
  "human-resources": { key: "human-resources", title: text("Human resources", "Rasilimali watu"), singular: text("employee", "mfanyakazi"), eyebrow: text("People & payroll", "Watu na mishahara"), description: text("Employee records, attendance, leave, loans and payroll.", "Rekodi za wafanyakazi, mahudhurio, likizo, mikopo na mishahara."), primaryAction: text("Add employee", "Ongeza mfanyakazi"), metrics: [{ label: text("Active employees", "Wafanyakazi hai"), value: "74", detail: text("4 branches", "Matawi 4") }, { label: text("Present today", "Waliopo leo"), value: "69", detail: text("93.2% attendance", "Mahudhurio 93.2%") }, { label: text("On leave", "Likizoni"), value: "3", detail: text("2 annual · 1 sick", "2 mwaka · 1 ugonjwa") }, { label: text("Payroll exceptions", "Matatizo ya mishahara"), value: "2", detail: text("Resolve by 20 Aug", "Tatua kabla 20 Ago") }], columns: [{ key: "department", label: text("Department", "Idara") }, { key: "attendance", label: text("Attendance", "Mahudhurio") }, { key: "payroll", label: text("Payroll", "Mshahara") }], records: hrRecords, insight: { title: text("Payroll inputs are nearly complete", "Data za mishahara karibu zimekamilika"), detail: text("Two loan deduction exceptions need HR review.", "Matatizo mawili ya makato ya mikopo yanahitaji ukaguzi."), tone: "warning" }, formFields: commonFormFields },
  reports: { key: "reports", title: text("Reports", "Ripoti"), singular: text("report", "ripoti"), eyebrow: text("Insights & controls", "Maarifa na udhibiti"), description: text("Governed operational and financial reporting with drill-down.", "Ripoti rasmi za shughuli na fedha zenye uchambuzi wa kina."), primaryAction: text("Create report view", "Unda mwonekano wa ripoti"), metrics: [{ label: text("Published reports", "Ripoti zilizochapishwa"), value: "42", detail: text("9 report packs", "Vifurushi 9") }, { label: text("Scheduled today", "Zilizopangwa leo"), value: "18", detail: text("17 delivered", "17 zimewasilishwa") }, { label: text("Data freshness", "Uhusika wa data"), value: "5 min", detail: text("Reporting projection", "Mfumo wa ripoti") }, { label: text("Exceptions", "Matatizo"), value: "1", detail: text("Trial balance", "Mizania ya majaribio") }], columns: [{ key: "owner", label: text("Owner", "Mmiliki") }, { key: "cadence", label: text("Cadence", "Marudio") }, { key: "lastRun", label: text("Last run", "Mara ya mwisho") }], records: reportRecords, insight: { title: text("Reporting projections are current", "Data za ripoti ni za sasa"), detail: text("All operational events are processed through 09:10 EAT.", "Matukio yote yamechakatwa hadi 09:10 EAT."), tone: "success" }, formFields: commonFormFields },
  settings: { key: "settings", title: text("Settings", "Mipangilio"), singular: text("configuration", "usanidi"), eyebrow: text("Platform governance", "Utawala wa jukwaa"), description: text("Organisation, access, fiscal rules, integrations and numbering.", "Shirika, ufikiaji, sheria za fiskali, miunganisho na namba."), primaryAction: text("New configuration", "Usanidi mpya"), metrics: [{ label: text("Active users", "Watumiaji hai"), value: "74", detail: text("18 roles", "Majukumu 18") }, { label: text("Connected services", "Huduma zilizounganishwa"), value: "9", detail: text("All healthy", "Zote ni nzuri") }, { label: text("Approval policies", "Sera za idhini"), value: "14", detail: text("3 effective-dated", "3 zina tarehe") }, { label: text("Access reviews", "Ukaguzi wa ufikiaji"), value: "1 due", detail: text("Finance roles", "Majukumu ya fedha") }], columns: [{ key: "owner", label: text("Owner", "Mmiliki") }, { key: "scope", label: text("Scope", "Upeo") }, { key: "changed", label: text("Last changed", "Ilibadilishwa") }], records: settingsRecords, insight: { title: text("Quarterly access review is due", "Ukaguzi wa ufikiaji unahitajika"), detail: text("Review six privileged finance assignments by 05 August.", "Kagua ruhusa sita za fedha kabla ya 05 Agosti."), tone: "warning" }, formFields: commonFormFields },
};

export const dashboard: DashboardData = {
  metrics: [
    { label: text("Net sales today", "Mauzo halisi leo"), value: "TZS 84.6m", detail: text("246 transactions", "Miamala 246"), change: "+12.4%", direction: "up" },
    { label: text("Cash position", "Hali ya fedha"), value: "TZS 62.7m", detail: text("Bank & mobile money", "Benki na simu"), change: "+4.8%", direction: "up" },
    { label: text("Receivables", "Madeni ya kupokea"), value: "TZS 18.2m", detail: text("7 accounts overdue", "Akaunti 7 zimechelewa"), change: "-5.2%", direction: "down" },
    { label: text("Stock value", "Thamani ya bidhaa"), value: "TZS 146.8m", detail: text("6 warehouses", "Maghala 6"), change: "+2.1%", direction: "up" },
  ],
  revenue: [
    { month: text("May", "Mei"), value: 54, label: "TZS 54.2m" }, { month: text("Jun", "Jun"), value: 63, label: "TZS 63.1m" },
    { month: text("Jul", "Jul"), value: 72, label: "TZS 72.4m" }, { month: text("Aug", "Ago"), value: 85, label: "TZS 84.6m" },
  ],
  approvals: [
    { id: "PO-2026-00412", type: text("Purchase order", "Oda ya manunuzi"), subject: "Mwanza Packaging Works", requester: "Godfrey Mushi", amount: "TZS 14.8m", age: text("32 min", "Dakika 32"), tone: "warning", href: "/purchases/PO-2026-00412" },
    { id: "SO-2026-01421", type: text("Credit exception", "Tofauti ya mkopo"), subject: "Mlimani Mini Mart", requester: "Yusuf Hamisi", amount: "TZS 3.92m", age: text("2 hours", "Saa 2"), tone: "danger", href: "/sales/SO-2026-01421" },
    { id: "JV-2026-00858", type: text("Journal review", "Ukaguzi wa jarida"), subject: "Bank charges · July sweep", requester: "Kelvin Maro", amount: "TZS 482k", age: text("Yesterday", "Jana"), tone: "info", href: "/finance/JV-2026-00858" },
  ],
  alerts: [
    { title: text("Reorder liquid soap", "Agiza sabuni ya maji"), detail: text("Only 7 units available in Mwanza", "Vipande 7 tu vinapatikana Mwanza"), meta: "SKU-HOM-0091", tone: "danger", href: "/inventory/SKU-HOM-0091" },
    { title: text("Supplier document expires", "Hati ya msambazaji inaisha"), detail: text("Tax clearance needs renewal", "Kibali cha kodi kinahitaji kusasishwa"), meta: "SUP-00158", tone: "warning", href: "/suppliers/SUP-00158" },
    { title: text("Bank items unmatched", "Miamala ya benki haijapatanishwa"), detail: text("7 items across 3 accounts", "Miamala 7 katika akaunti 3"), meta: "Finance", tone: "info", href: "/finance" },
  ],
  activity: [
    { actor: "Rehema Joseph", action: text("completed a cash sale", "amekamilisha mauzo ya fedha"), subject: "SO-2026-01427", time: text("24 min ago", "Dakika 24 zilizopita"), tone: "success" },
    { actor: "System worker", action: text("posted POS settlement", "imechapisha malipo ya POS"), subject: "JV-2026-00861", time: text("14 min ago", "Dakika 14 zilizopita"), tone: "info" },
    { actor: "Godfrey Mushi", action: text("submitted a purchase order", "amewasilisha oda ya manunuzi"), subject: "PO-2026-00412", time: text("32 min ago", "Dakika 32 zilizopita"), tone: "warning" },
  ],
  branches: [
    { branch: "Dar es Salaam HQ", amount: "TZS 42.8m", percent: 100, change: "+14.2%" },
    { branch: "Mwanza", amount: "TZS 18.6m", percent: 62, change: "+9.8%" },
    { branch: "Arusha", amount: "TZS 14.2m", percent: 46, change: "+6.1%" },
    { branch: "Dodoma", amount: "TZS 9.0m", percent: 31, change: "+3.4%" },
  ],
  quickActions: [
    { label: text("Create sale", "Unda mauzo"), description: text("Order or cash sale", "Oda au mauzo ya fedha"), href: "/sales/new", icon: "sale" },
    { label: text("Raise purchase order", "Unda oda ya manunuzi"), description: text("Source approved stock", "Nunua bidhaa zilizoidhinishwa"), href: "/purchases/new", icon: "purchase" },
    { label: text("Record payment", "Rekodi malipo"), description: text("Customer or supplier", "Mteja au msambazaji"), href: "/finance/new", icon: "payment" },
    { label: text("Move stock", "Hamisha bidhaa"), description: text("Transfer or adjustment", "Uhamisho au marekebisho"), href: "/inventory/new", icon: "stock" },
  ],
  salesMix: [
    { label: text("Wholesale", "Jumla"), value: 52, amount: "TZS 44.0m", color: "#2f7a61" },
    { label: text("Retail", "Rejareja"), value: 31, amount: "TZS 26.2m", color: "#e4a83b" },
    { label: text("Distribution", "Usambazaji"), value: 17, amount: "TZS 14.4m", color: "#7aa6b4" },
  ],
  closeReadiness: [
    { label: text("Sales & tax", "Mauzo na kodi"), value: 100, status: status("Ready", "Tayari", "success") },
    { label: text("Stock controls", "Udhibiti wa bidhaa"), value: 82, status: status("In progress", "Inaendelea", "info") },
    { label: text("Bank reconciliation", "Upatanisho wa benki"), value: 64, status: status("7 items open", "Miamala 7 wazi", "warning") },
  ],
};
