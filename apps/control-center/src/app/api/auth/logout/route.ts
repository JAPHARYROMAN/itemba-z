import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";
import { loadOidcRuntimeConfig } from "@/auth/config";
import { clearBrowserSession, readBrowserSession } from "@/auth/browser-session";
import { providerLogoutUrl, revokeBrowserSession } from "@/auth/oidc";
import { isTrustedMutationOrigin } from "@/auth/session";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest): Promise<NextResponse> {
  let config;
  try {
    config = loadOidcRuntimeConfig();
  } catch {
    return NextResponse.json({ error: "OIDC logout is not configured." }, { status: 503 });
  }
  if (!isTrustedMutationOrigin(request, config.controlCenterOrigin.origin)) {
    return NextResponse.json({ error: "Logout origin was rejected." }, { status: 403 });
  }

  const cookieStore = await cookies();
  const accessToken = cookieStore.get(config.accessCookieName)?.value;
  try {
    const session = readBrowserSession(cookieStore, config);
    if (session) await revokeBrowserSession(config, session, accessToken);
  } catch {
    // Local session deletion remains mandatory even if provider revocation fails.
  }
  const destination = await providerLogoutUrl(config).catch(() => null);
  const response = NextResponse.redirect(destination ?? config.postLogoutUrl, 303);
  response.headers.set("Cache-Control", "no-store");
  clearBrowserSession(response, config);
  return response;
}
