import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";
import { loadOidcRuntimeConfig } from "@/auth/config";
import { clearBrowserSession, readBrowserSession, setBrowserSession } from "@/auth/browser-session";
import { refreshBrowserSession, revokeBrowserSession } from "@/auth/oidc";
import { isTrustedMutationOrigin, sessionExpiryReason, shouldRefreshAccessToken } from "@/auth/session";

export const dynamic = "force-dynamic";

function json(body: object, status = 200): NextResponse {
  const response = NextResponse.json(body, { status });
  response.headers.set("Cache-Control", "no-store");
  return response;
}

export async function POST(request: NextRequest): Promise<NextResponse> {
  if (process.env.ITEMBA_ENV === "development" && !process.env.ITEMBA_OIDC_ISSUER_URL?.trim()) {
    return json({ authenticated: true, mode: "development", displayName: "Development operator" });
  }

  let config;
  try {
    config = loadOidcRuntimeConfig();
  } catch {
    return json({ authenticated: false, reason: "oidc_configuration_invalid" }, 503);
  }
  if (!isTrustedMutationOrigin(request, config.controlCenterOrigin.origin)) {
    return json({ authenticated: false, reason: "origin_rejected" }, 403);
  }

  const cookieStore = await cookies();
  const accessToken = cookieStore.get(config.accessCookieName)?.value;
  let session;
  try {
    session = readBrowserSession(cookieStore, config);
  } catch {
    const response = json({ authenticated: false, reason: "session_invalid" }, 401);
    clearBrowserSession(response, config);
    return response;
  }
  if (!session) return json({ authenticated: false, reason: "session_missing" }, 401);

  const now = Date.now();
  const expiryReason = sessionExpiryReason(session, now, config.idleTimeoutSeconds);
  if (expiryReason) {
    await revokeBrowserSession(config, session, accessToken).catch(() => undefined);
    const response = json({ authenticated: false, reason: expiryReason }, 401);
    clearBrowserSession(response, config);
    return response;
  }

  try {
    let issued = { accessToken: accessToken ?? "", session: { ...session, lastActivityAt: now } };
    let refreshed = false;
    if (!accessToken || shouldRefreshAccessToken(session, now, config.refreshLeewaySeconds)) {
      issued = await refreshBrowserSession(config, session, now);
      refreshed = true;
    }
    if (!issued.accessToken) throw new Error("access token missing");
    const response = json({
      authenticated: true,
      mode: "oidc",
      displayName: issued.session.displayName,
      email: issued.session.email,
      expiresAt: issued.session.accessExpiresAt,
      absoluteExpiresAt: issued.session.absoluteExpiresAt,
      assuranceLevel: issued.session.assuranceLevel,
      authenticationMethods: issued.session.authenticationMethods,
      refreshed,
    });
    setBrowserSession(response, config, issued.session, issued.accessToken, now);
    return response;
  } catch {
    await revokeBrowserSession(config, session, accessToken).catch(() => undefined);
    const response = json({ authenticated: false, reason: "renewal_failed" }, 401);
    clearBrowserSession(response, config);
    return response;
  }
}
