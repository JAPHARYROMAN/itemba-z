import "server-only";

import type { NextResponse } from "next/server";
import type { OidcRuntimeConfig } from "@/auth/config";
import { sealCookie, unsealCookie } from "@/auth/sealed-cookie";
import {
  baseCookieOptions,
  cookieMaxAge,
  type BrowserSession,
  type LoginTransaction,
  validateBrowserSession,
  validateLoginTransaction,
} from "@/auth/session";

interface CookieReader {
  get(name: string): { value: string } | undefined;
}

const SESSION_PURPOSE = "browser-session";
const TRANSACTION_PURPOSE = "login-transaction";

export function readBrowserSession(cookieStore: CookieReader, config: OidcRuntimeConfig): BrowserSession | null {
  const value = cookieStore.get(config.sessionCookieName)?.value;
  if (!value) return null;
  return validateBrowserSession(unsealCookie(value, SESSION_PURPOSE, config.encryptionKeys));
}

export function readLoginTransaction(cookieStore: CookieReader, config: OidcRuntimeConfig, now = Date.now()): LoginTransaction {
  const value = cookieStore.get(config.transactionCookieName)?.value;
  if (!value) throw new Error("login transaction cookie is missing");
  return validateLoginTransaction(
    unsealCookie(value, TRANSACTION_PURPOSE, config.encryptionKeys),
    now,
    config.transactionTimeoutSeconds,
  );
}

export function setLoginTransaction(
  response: NextResponse,
  config: OidcRuntimeConfig,
  transaction: LoginTransaction,
): void {
  response.cookies.set(config.transactionCookieName, sealCookie(transaction, TRANSACTION_PURPOSE, config.encryptionKeys), {
    ...baseCookieOptions(config),
    maxAge: config.transactionTimeoutSeconds,
  });
}

export function clearLoginTransaction(response: NextResponse, config: OidcRuntimeConfig): void {
  response.cookies.set(config.transactionCookieName, "", { ...baseCookieOptions(config), maxAge: 0 });
}

export function setBrowserSession(
  response: NextResponse,
  config: OidcRuntimeConfig,
  session: BrowserSession,
  accessToken: string,
  now = Date.now(),
): void {
  const sessionMaxAge = cookieMaxAge(session.absoluteExpiresAt, now);
  const accessMaxAge = cookieMaxAge(Math.min(session.accessExpiresAt, session.absoluteExpiresAt), now);
  response.cookies.set(config.accessCookieName, accessToken, { ...baseCookieOptions(config), maxAge: accessMaxAge });
  response.cookies.set(config.sessionCookieName, sealCookie(session, SESSION_PURPOSE, config.encryptionKeys), {
    ...baseCookieOptions(config),
    maxAge: sessionMaxAge,
  });
}

export function clearBrowserSession(response: NextResponse, config: OidcRuntimeConfig): void {
  for (const name of [config.accessCookieName, config.sessionCookieName, config.transactionCookieName]) {
    response.cookies.set(name, "", { ...baseCookieOptions(config), maxAge: 0 });
  }
}
