import { cookies } from "next/headers";
import { NextRequest, NextResponse } from "next/server";
import { loadOidcRuntimeConfig } from "@/auth/config";
import {
  clearBrowserSession,
  clearLoginTransaction,
  readLoginTransaction,
  setBrowserSession,
} from "@/auth/browser-session";
import { completeAuthorization } from "@/auth/oidc";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest): Promise<NextResponse> {
  let config;
  try {
    config = loadOidcRuntimeConfig();
    const cookieStore = await cookies();
    const transaction = readLoginTransaction(cookieStore, config);
    const callbackUrl = new URL(config.callbackUrl);
    callbackUrl.search = request.nextUrl.search;
    const issued = await completeAuthorization(config, callbackUrl, transaction);
    const response = NextResponse.redirect(new URL(transaction.returnTo, config.controlCenterOrigin), 303);
    response.headers.set("Cache-Control", "no-store");
    clearLoginTransaction(response, config);
    setBrowserSession(response, config, issued.session, issued.accessToken);
    return response;
  } catch (error) {
    console.error("Control Center OIDC callback was rejected", { error: error instanceof Error ? error.name : "UnknownError" });
    const response = NextResponse.redirect(new URL("/login?error=authentication_failed", request.url), 303);
    response.headers.set("Cache-Control", "no-store");
    if (config) clearBrowserSession(response, config);
    return response;
  }
}
