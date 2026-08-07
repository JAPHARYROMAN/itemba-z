import { NextRequest, NextResponse } from "next/server";
import { loadOidcRuntimeConfig, sanitizeReturnTo } from "@/auth/config";
import { setLoginTransaction } from "@/auth/browser-session";
import { beginAuthorization } from "@/auth/oidc";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest): Promise<NextResponse> {
  try {
    const config = loadOidcRuntimeConfig();
    const returnTo = sanitizeReturnTo(request.nextUrl.searchParams.get("returnTo"));
    const forceReauthentication = request.nextUrl.searchParams.get("force") === "1";
    const { redirectUrl, transaction } = await beginAuthorization(config, returnTo, forceReauthentication);
    const response = NextResponse.redirect(redirectUrl);
    response.headers.set("Cache-Control", "no-store");
    setLoginTransaction(response, config, transaction);
    return response;
  } catch (error) {
    console.error("Control Center OIDC login could not start", { error: error instanceof Error ? error.name : "UnknownError" });
    return NextResponse.redirect(new URL("/login?error=oidc_unavailable", request.url));
  }
}
