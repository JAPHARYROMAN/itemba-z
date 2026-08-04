export type Locale = "en" | "sw";

export interface LocalizedText {
  en: string;
  sw: string;
}

export type ModuleKey =
  | "customers"
  | "suppliers"
  | "sales"
  | "purchases"
  | "inventory"
  | "finance"
  | "human-resources"
  | "reports"
  | "settings";

export type StatusTone = "success" | "warning" | "danger" | "info" | "neutral";

export interface Status {
  label: LocalizedText;
  tone: StatusTone;
}

export interface Metric {
  label: LocalizedText;
  value: string;
  detail: LocalizedText;
  change?: string;
  direction?: "up" | "down" | "flat";
}

export interface RevenuePoint {
  month: LocalizedText;
  value: number;
  label: string;
}

export interface ApprovalItem {
  id: string;
  type: LocalizedText;
  subject: string;
  requester: string;
  amount: string;
  age: LocalizedText;
  tone: StatusTone;
  href: string;
}

export interface OperationalAlert {
  title: LocalizedText;
  detail: LocalizedText;
  meta: string;
  tone: StatusTone;
  href: string;
}

export interface ActivityItem {
  actor: string;
  action: LocalizedText;
  subject: string;
  time: LocalizedText;
  tone: StatusTone;
}

export interface BranchPerformance {
  branch: string;
  amount: string;
  percent: number;
  change: string;
}

export interface QuickAction {
  label: LocalizedText;
  description: LocalizedText;
  href: string;
  icon: "sale" | "purchase" | "payment" | "stock";
}

export interface DashboardData {
  metrics: Metric[];
  revenue: RevenuePoint[];
  approvals: ApprovalItem[];
  alerts: OperationalAlert[];
  activity: ActivityItem[];
  branches: BranchPerformance[];
  quickActions: QuickAction[];
  salesMix: Array<{ label: LocalizedText; value: number; amount: string; color: string }>;
  closeReadiness: Array<{ label: LocalizedText; value: number; status: Status }>;
}

export interface TableColumn {
  key: string;
  label: LocalizedText;
  align?: "left" | "right";
}

export interface DetailField {
  label: LocalizedText;
  value: string;
}

export interface LedgerLine {
  account: string;
  debit: string;
  credit: string;
}

export interface ControlCheck {
  label: LocalizedText;
  detail: LocalizedText;
  status: Status;
}

export interface TimelineEvent {
  title: LocalizedText;
  detail: string;
  time: LocalizedText;
  tone: StatusTone;
}

export interface ModuleRecord {
  id: string;
  primary: LocalizedText;
  secondary: string;
  values: Record<string, string>;
  status: Status;
  updated: LocalizedText;
  details: DetailField[];
  ledgerImpact?: LedgerLine[];
  controls: ControlCheck[];
  timeline: TimelineEvent[];
}

export interface FormField {
  name: string;
  label: LocalizedText;
  placeholder: LocalizedText;
  type: "text" | "number" | "date" | "select" | "textarea";
  required?: boolean;
  options?: LocalizedText[];
}

export interface ModuleData {
  key: ModuleKey;
  title: LocalizedText;
  singular: LocalizedText;
  description: LocalizedText;
  eyebrow: LocalizedText;
  primaryAction: LocalizedText;
  metrics: Metric[];
  columns: TableColumn[];
  records: ModuleRecord[];
  insight: {
    title: LocalizedText;
    detail: LocalizedText;
    tone: StatusTone;
  };
  formFields: FormField[];
}

export interface SearchResult {
  module: ModuleKey;
  moduleLabel: LocalizedText;
  id: string;
  title: LocalizedText;
  subtitle: string;
  status: Status;
  href: string;
}
