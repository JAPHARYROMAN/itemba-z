import type { WorkingContext } from "@/live-api/types";

export const PENDING_COMMAND_REVIEW_AFTER_MS = 24 * 60 * 60 * 1_000;

const STORAGE_PREFIX = "itemba-z:pending-command:v1:";
const STORAGE_CHANGE_EVENT = "itemba-z:pending-command-change";

export type PendingCommandOutcome = "pending" | "rejected";

export interface PendingCommandRecord<T> {
  version: 1;
  key: string;
  fingerprint: string;
  payload: T;
  createdAt: number;
  expiresAt: number;
  outcome: PendingCommandOutcome;
  lastStatus?: number;
}

export interface PendingCommandSnapshot<T> {
  record: PendingCommandRecord<T>;
  expired: boolean;
}

type PendingCommandOwnerContext = Pick<WorkingContext, "actor_id" | "tenant_id" | "company_id" | "branch_id" | "warehouse_id">;

interface ReservePendingCommandOptions<T> {
  storage: Storage;
  scope: string;
  fingerprint: string;
  payload: T;
  now?: number;
  createKey?: () => string;
}

export class PendingCommandConflictError<T = unknown> extends Error {
  constructor(public readonly pending: PendingCommandSnapshot<T>) {
    super("A different command is awaiting an authoritative response.");
    this.name = "PendingCommandConflictError";
  }
}

export class PendingCommandStorageError extends Error {
  constructor(message = "The browser could not preserve the command idempotency key.") {
    super(message);
    this.name = "PendingCommandStorageError";
  }
}

function commandOwnerScope(context: PendingCommandOwnerContext): string {
  return [
    ["actor", context.actor_id],
    ["tenant", context.tenant_id],
    ["company", context.company_id],
    ["branch", context.branch_id],
    ["warehouse", context.warehouse_id],
  ].map(([label, value]) => `${label}=${encodeURIComponent(value)}`).join(":");
}

export function saleCompletionPendingScope(context: PendingCommandOwnerContext): string {
  return `sales:create:${commandOwnerScope(context)}`;
}

export function saleReversalPendingScope(context: PendingCommandOwnerContext, saleId: string): string {
  return `sales:reverse:${commandOwnerScope(context)}:sale=${encodeURIComponent(saleId)}`;
}

export function reconciliationResolutionPendingScope(context: PendingCommandOwnerContext, caseId: string): string {
  return `mobile:reconciliation:resolve:${commandOwnerScope(context)}:case=${encodeURIComponent(caseId)}`;
}

export function deviceStatusPendingScope(context: PendingCommandOwnerContext, deviceId: string): string {
  return `mobile:device:status:${commandOwnerScope(context)}:device=${encodeURIComponent(deviceId)}`;
}

export function deviceAllocationPendingScope(context: PendingCommandOwnerContext, deviceId: string): string {
  return `mobile:device:allocation:${commandOwnerScope(context)}:device=${encodeURIComponent(deviceId)}`;
}

export function pendingCommandStorageKey(scope: string): string {
  return `${STORAGE_PREFIX}${scope}`;
}

function notifySubscribers(storage: Storage, scope: string): void {
  if (typeof window === "undefined" || storage !== window.sessionStorage) return;
  window.dispatchEvent(new CustomEvent(STORAGE_CHANGE_EVENT, { detail: scope }));
}

function isRecord(value: unknown): value is PendingCommandRecord<unknown> {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Record<string, unknown>;
  return candidate.version === 1
    && typeof candidate.key === "string"
    && candidate.key.length >= 16
    && candidate.key.length <= 128
    && !/[\r\n]/.test(candidate.key)
    && typeof candidate.fingerprint === "string"
    && "payload" in candidate
    && candidate.fingerprint === JSON.stringify(candidate.payload)
    && typeof candidate.createdAt === "number"
    && Number.isSafeInteger(candidate.createdAt)
    && typeof candidate.expiresAt === "number"
    && Number.isSafeInteger(candidate.expiresAt)
    && candidate.expiresAt >= candidate.createdAt
    && (candidate.outcome === "pending" || candidate.outcome === "rejected")
    && (candidate.lastStatus === undefined || (typeof candidate.lastStatus === "number" && Number.isSafeInteger(candidate.lastStatus)));
}

function persist<T>(storage: Storage, scope: string, record: PendingCommandRecord<T>): void {
  try {
    storage.setItem(pendingCommandStorageKey(scope), JSON.stringify(record));
    notifySubscribers(storage, scope);
  } catch {
    throw new PendingCommandStorageError();
  }
}

export function readPendingCommandRaw(storage: Storage, scope: string): string | null {
  try {
    return storage.getItem(pendingCommandStorageKey(scope));
  } catch {
    throw new PendingCommandStorageError();
  }
}

export function parsePendingCommand<T>(raw: string | null, now = Date.now()): PendingCommandSnapshot<T> | null {
  if (!raw) return null;

  try {
    const record: unknown = JSON.parse(raw);
    if (!isRecord(record)) throw new Error("invalid pending command record");
    return { record: record as PendingCommandRecord<T>, expired: now >= record.expiresAt };
  } catch (error) {
    if (error instanceof PendingCommandStorageError) throw error;
    throw new PendingCommandStorageError("A preserved command exists but cannot be safely recovered.");
  }
}

export function readPendingCommand<T>(storage: Storage, scope: string, now = Date.now()): PendingCommandSnapshot<T> | null {
  return parsePendingCommand<T>(readPendingCommandRaw(storage, scope), now);
}

export function subscribePendingCommand(scope: string, callback: () => void): () => void {
  if (typeof window === "undefined") return () => undefined;
  const onCustomChange = (event: Event) => {
    if ((event as CustomEvent<unknown>).detail === scope) callback();
  };
  const onStorageChange = (event: StorageEvent) => {
    if (event.storageArea === window.sessionStorage && event.key === pendingCommandStorageKey(scope)) callback();
  };
  window.addEventListener(STORAGE_CHANGE_EVENT, onCustomChange);
  window.addEventListener("storage", onStorageChange);
  return () => {
    window.removeEventListener(STORAGE_CHANGE_EVENT, onCustomChange);
    window.removeEventListener("storage", onStorageChange);
  };
}

export function reservePendingCommand<T>({
  storage,
  scope,
  fingerprint,
  payload,
  now = Date.now(),
  createKey = () => crypto.randomUUID(),
}: ReservePendingCommandOptions<T>): PendingCommandSnapshot<T> {
  const existing = readPendingCommand<T>(storage, scope, now);
  if (existing?.record.fingerprint === fingerprint) {
    const record = { ...existing.record, outcome: "pending" as const, lastStatus: undefined };
    persist(storage, scope, record);
    return { record, expired: existing.expired };
  }
  if (existing && existing.record.outcome !== "rejected") throw new PendingCommandConflictError(existing);

  const record: PendingCommandRecord<T> = existing
    ? {
        ...existing.record,
        fingerprint,
        payload,
        outcome: "pending",
        lastStatus: undefined,
      }
    : {
        version: 1,
        key: createKey(),
        fingerprint,
        payload,
        createdAt: now,
        expiresAt: now + PENDING_COMMAND_REVIEW_AFTER_MS,
        outcome: "pending",
      };
  persist(storage, scope, record);
  return { record, expired: now >= record.expiresAt };
}

export function markPendingCommandRejected<T>(storage: Storage, scope: string, key: string, status: number): PendingCommandSnapshot<T> | null {
  const existing = readPendingCommand<T>(storage, scope);
  if (!existing || existing.record.key !== key) return existing;
  const record = { ...existing.record, outcome: "rejected" as const, lastStatus: status };
  persist(storage, scope, record);
  return { record, expired: existing.expired };
}

export function clearPendingCommandAfterSuccess(storage: Storage, scope: string, key: string): void {
  const existing = readPendingCommand(storage, scope);
  if (!existing || existing.record.key !== key) return;
  try {
    storage.removeItem(pendingCommandStorageKey(scope));
    notifySubscribers(storage, scope);
  } catch {
    throw new PendingCommandStorageError("The sale posted, but its browser recovery marker could not be cleared.");
  }
}
