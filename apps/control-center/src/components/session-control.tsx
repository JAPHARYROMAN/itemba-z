"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronDown, LogIn, LogOut, RotateCcw, ShieldCheck } from "lucide-react";
import { useLanguage } from "@/components/language-provider";
import { text } from "@/lib/i18n";

type SessionState =
  | { status: "checking" }
  | { status: "signed-out" | "error" }
  | { status: "authenticated"; mode: "oidc" | "development"; displayName: string; email?: string; assuranceLevel?: string };

interface SessionResponse {
  authenticated: boolean;
  mode?: "oidc" | "development";
  displayName?: string;
  email?: string;
  assuranceLevel?: string;
  refreshed?: boolean;
}

const HEARTBEAT_INTERVAL_MS = 60_000;
const ACTIVE_WINDOW_MS = 120_000;

function initials(displayName: string): string {
  const parts = displayName.split(/\s+/).filter(Boolean);
  return (parts.length > 1 ? `${parts[0][0]}${parts.at(-1)?.[0]}` : parts[0]?.slice(0, 2) || "ID").toUpperCase();
}

export function SessionControl() {
  const router = useRouter();
  const { l } = useLanguage();
  const [session, setSession] = useState<SessionState>({ status: "checking" });
  const lastActivityAt = useRef(0);
  const lastHeartbeatAt = useRef(0);
  const heartbeatPending = useRef(false);

  const heartbeat = useCallback(async (force = false) => {
    const now = Date.now();
    if (heartbeatPending.current || (!force && now - lastHeartbeatAt.current < HEARTBEAT_INTERVAL_MS)) return;
    heartbeatPending.current = true;
    lastHeartbeatAt.current = now;
    try {
      const response = await fetch("/api/auth/session", {
        method: "POST",
        headers: { "X-ITEMBA-Session-Intent": "heartbeat" },
        cache: "no-store",
      });
      const body = await response.json() as SessionResponse;
      if (response.ok && body.authenticated && body.mode && body.displayName) {
        setSession({
          status: "authenticated",
          mode: body.mode,
          displayName: body.displayName,
          email: body.email,
          assuranceLevel: body.assuranceLevel,
        });
        if (body.refreshed) router.refresh();
      } else {
        setSession({ status: response.status === 401 ? "signed-out" : "error" });
      }
    } catch {
      setSession({ status: "error" });
    } finally {
      heartbeatPending.current = false;
    }
  }, [router]);

  useEffect(() => {
    lastActivityAt.current = Date.now();
    const initialHeartbeat = window.setTimeout(() => void heartbeat(true), 0);
    const recordActivity = () => {
      lastActivityAt.current = Date.now();
      void heartbeat();
    };
    const handleVisibility = () => {
      if (document.visibilityState === "visible") recordActivity();
    };
    window.addEventListener("pointerdown", recordActivity, { passive: true });
    window.addEventListener("keydown", recordActivity);
    window.addEventListener("focus", recordActivity);
    document.addEventListener("visibilitychange", handleVisibility);
    const interval = window.setInterval(() => {
      if (document.visibilityState === "visible" && Date.now() - lastActivityAt.current <= ACTIVE_WINDOW_MS) void heartbeat();
    }, HEARTBEAT_INTERVAL_MS);
    return () => {
      window.removeEventListener("pointerdown", recordActivity);
      window.removeEventListener("keydown", recordActivity);
      window.removeEventListener("focus", recordActivity);
      document.removeEventListener("visibilitychange", handleVisibility);
      window.clearInterval(interval);
      window.clearTimeout(initialHeartbeat);
    };
  }, [heartbeat]);

  const signedIn = session.status === "authenticated";
  const displayName = signedIn ? session.displayName : l(text("Authentication required", "Uthibitisho unahitajika"));
  const detail = signedIn
    ? session.mode === "development"
      ? l(text("Local development identity", "Utambulisho wa maendeleo"))
      : session.email ?? l(text("Verified OIDC session", "Kikao cha OIDC kimethibitishwa"))
    : session.status === "checking"
      ? l(text("Checking secure session", "Inakagua kikao salama"))
      : l(text("Live ERP is locked", "ERP hai imefungwa"));

  return (
    <details className="header-popover profile-popover">
      <summary className="profile-summary">
        <span className={`avatar${signedIn ? "" : " avatar-locked"}`}>{signedIn ? initials(session.displayName) : "ID"}</span>
        <span className="profile-copy"><strong>{displayName}</strong><small>{detail}</small></span><ChevronDown size={15} />
      </summary>
      <div className="popover-panel profile-panel">
        <p><span>{l(text("Session status", "Hali ya kikao"))}</span><strong>{detail}</strong></p>
        {signedIn && session.mode === "oidc" ? (
          <>
            <Link href="/api/auth/login?force=1"><RotateCcw size={17} />{l(text("Reauthenticate", "Thibitisha tena"))}</Link>
            <div className="session-assurance"><ShieldCheck size={16} /><span>{session.assuranceLevel ?? l(text("Provider assurance", "Uhakikisho wa mtoa huduma"))}</span></div>
            <form action="/api/auth/logout" method="post"><button type="submit"><LogOut size={17} />{l(text("Sign out", "Toka"))}</button></form>
          </>
        ) : signedIn ? (
          <div className="session-assurance"><ShieldCheck size={16} /><span>{l(text("Development only", "Kwa maendeleo pekee"))}</span></div>
        ) : (
          <Link href="/api/auth/login"><LogIn size={17} />{l(text("Sign in securely", "Ingia kwa usalama"))}</Link>
        )}
      </div>
    </details>
  );
}
