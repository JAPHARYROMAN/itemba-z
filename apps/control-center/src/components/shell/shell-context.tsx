"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { Dashboard, PublicProblem, WorkingContext } from "@/live-api/types";

export type ShellResource<T> =
  | { state: "loading" }
  | { state: "ready"; data: T }
  | { state: "unavailable"; problem: PublicProblem }
  | { state: "not-permitted" };

interface ShellContextValue {
  context: ShellResource<WorkingContext>;
  dashboard: ShellResource<Dashboard>;
  refresh: () => void;
}

const unavailableProblem: PublicProblem = {
  type: "about:blank",
  title: "Working context unavailable",
  status: 503,
  code: "context_unavailable",
  detail: "The active company, branch, and warehouse could not be loaded.",
};

const ShellContext = createContext<ShellContextValue | null>(null);

async function readLiveResponse<T>(response: Response): Promise<T> {
  const payload = await response.json() as T | PublicProblem;
  if (!response.ok) throw payload;
  return payload as T;
}

function isPublicProblem(value: unknown): value is PublicProblem {
  return typeof value === "object" && value !== null && "code" in value && "status" in value;
}

export function ShellContextProvider({ children }: Readonly<{ children: React.ReactNode }>) {
  const [context, setContext] = useState<ShellResource<WorkingContext>>({ state: "loading" });
  const [dashboard, setDashboard] = useState<ShellResource<Dashboard>>({ state: "loading" });
  const activeRequest = useRef<AbortController | null>(null);

  const loadShellResources = useCallback(async () => {
    activeRequest.current?.abort();
    const controller = new AbortController();
    activeRequest.current = controller;

    try {
      const contextResponse = await fetch("/api/live/context", {
        cache: "no-store",
        signal: controller.signal,
        headers: { Accept: "application/json" },
      });
      const workingContext = await readLiveResponse<WorkingContext>(contextResponse);
      if (controller.signal.aborted) return;
      setContext({ state: "ready", data: workingContext });

      if (!workingContext.permissions.includes("dashboard.read")) {
        setDashboard({ state: "not-permitted" });
        return;
      }

      try {
        const dashboardResponse = await fetch("/api/live/dashboard", {
          cache: "no-store",
          signal: controller.signal,
          headers: { Accept: "application/json" },
        });
        const dashboardData = await readLiveResponse<Dashboard>(dashboardResponse);
        if (!controller.signal.aborted) setDashboard({ state: "ready", data: dashboardData });
      } catch (error) {
        if (controller.signal.aborted) return;
        setDashboard({
          state: "unavailable",
          problem: isPublicProblem(error) ? error : { ...unavailableProblem, code: "dashboard_unavailable", title: "Approvals unavailable" },
        });
      }
    } catch (error) {
      if (controller.signal.aborted) return;
      setContext({ state: "unavailable", problem: isPublicProblem(error) ? error : unavailableProblem });
      setDashboard({ state: "not-permitted" });
    }
  }, []);

  useEffect(() => {
    queueMicrotask(() => void loadShellResources());
    const refreshWhenActive = () => {
      if (document.visibilityState === "visible") void loadShellResources();
    };
    const refreshOnFocus = () => void loadShellResources();
    window.addEventListener("focus", refreshOnFocus);
    window.addEventListener("itemba:shell-refresh", refreshOnFocus);
    document.addEventListener("visibilitychange", refreshWhenActive);
    return () => {
      activeRequest.current?.abort();
      window.removeEventListener("focus", refreshOnFocus);
      window.removeEventListener("itemba:shell-refresh", refreshOnFocus);
      document.removeEventListener("visibilitychange", refreshWhenActive);
    };
  }, [loadShellResources]);

  const value = useMemo(() => ({ context, dashboard, refresh: () => void loadShellResources() }), [context, dashboard, loadShellResources]);
  return <ShellContext.Provider value={value}>{children}</ShellContext.Provider>;
}

export function useShellContext(): ShellContextValue {
  const value = useContext(ShellContext);
  if (!value) throw new Error("useShellContext must be used within ShellContextProvider");
  return value;
}
