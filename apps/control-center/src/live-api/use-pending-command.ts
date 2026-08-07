"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import {
  parsePendingCommand,
  PendingCommandStorageError,
  readPendingCommandRaw,
  subscribePendingCommand,
  type PendingCommandSnapshot,
} from "@/live-api/pending-command";

const STORAGE_ERROR_SENTINEL = "__ITEMBA_PENDING_COMMAND_STORAGE_ERROR__";

export interface PendingCommandStore<T> {
  snapshot: PendingCommandSnapshot<T> | null;
  error: PendingCommandStorageError | null;
}

export function usePendingCommand<T>(scope: string): PendingCommandStore<T> {
  const subscribe = useCallback((callback: () => void) => subscribePendingCommand(scope, callback), [scope]);
  const getSnapshot = useCallback(() => {
    try {
      return readPendingCommandRaw(window.sessionStorage, scope);
    } catch {
      return STORAGE_ERROR_SENTINEL;
    }
  }, [scope]);
  const raw = useSyncExternalStore(subscribe, getSnapshot, () => null);

  return useMemo(() => {
    if (raw === STORAGE_ERROR_SENTINEL) return { snapshot: null, error: new PendingCommandStorageError() };
    try {
      return { snapshot: parsePendingCommand<T>(raw), error: null };
    } catch (error) {
      return {
        snapshot: null,
        error: error instanceof PendingCommandStorageError ? error : new PendingCommandStorageError("A preserved command exists but cannot be safely recovered."),
      };
    }
  }, [raw]);
}
