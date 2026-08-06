import type { OidcRuntimeConfig } from "@/auth/config";

export interface LoginTransaction {
  version: 1;
  state: string;
  nonce: string;
  codeVerifier: string;
  returnTo: string;
  createdAt: number;
  maxAgeSeconds: number;
}

export interface BrowserSession {
  version: 1;
  sessionId: string;
  subject: string;
  displayName: string;
  email?: string;
  refreshToken?: string;
  issuedAt: number;
  lastActivityAt: number;
  absoluteExpiresAt: number;
  accessExpiresAt: number;
  authenticationTime?: number;
  assuranceLevel?: string;
  authenticationMethods: string[];
}

export type SessionExpiryReason = "idle_timeout" | "absolute_timeout" | null;

export function sessionExpiryReason(session: BrowserSession, now = Date.now(), idleTimeoutSeconds: number): SessionExpiryReason {
  if (now >= session.absoluteExpiresAt) return "absolute_timeout";
  if (now - session.lastActivityAt >= idleTimeoutSeconds * 1_000) return "idle_timeout";
  return null;
}

export function shouldRefreshAccessToken(session: BrowserSession, now: number, leewaySeconds: number): boolean {
  return session.accessExpiresAt - now <= leewaySeconds * 1_000;
}

export function baseCookieOptions(config: OidcRuntimeConfig) {
  return { httpOnly: true, sameSite: "lax" as const, secure: config.secureCookies, path: "/" };
}

export function cookieMaxAge(expiresAt: number, now = Date.now()): number {
  return Math.max(1, Math.floor((expiresAt - now) / 1_000));
}

export function validateBrowserSession(value: unknown): BrowserSession {
  if (!value || typeof value !== "object") throw new Error("session payload is not an object");
  const session = value as Partial<BrowserSession>;
  if (
    session.version !== 1 ||
    typeof session.sessionId !== "string" || !session.sessionId ||
    typeof session.subject !== "string" || !session.subject ||
    typeof session.displayName !== "string" || !session.displayName ||
    typeof session.issuedAt !== "number" ||
    typeof session.lastActivityAt !== "number" ||
    typeof session.absoluteExpiresAt !== "number" ||
    typeof session.accessExpiresAt !== "number" ||
    !Array.isArray(session.authenticationMethods) || !session.authenticationMethods.every((method) => typeof method === "string")
  ) {
    throw new Error("session payload is malformed");
  }
  return session as BrowserSession;
}

export function validateLoginTransaction(value: unknown, now: number, timeoutSeconds: number): LoginTransaction {
  if (!value || typeof value !== "object") throw new Error("login transaction is not an object");
  const transaction = value as Partial<LoginTransaction>;
  if (
    transaction.version !== 1 ||
    typeof transaction.state !== "string" || !transaction.state ||
    typeof transaction.nonce !== "string" || !transaction.nonce ||
    typeof transaction.codeVerifier !== "string" || !transaction.codeVerifier ||
    typeof transaction.returnTo !== "string" || !transaction.returnTo.startsWith("/") ||
    typeof transaction.createdAt !== "number" ||
    typeof transaction.maxAgeSeconds !== "number" || !Number.isSafeInteger(transaction.maxAgeSeconds) || transaction.maxAgeSeconds < 0 ||
    now - transaction.createdAt < 0 ||
    now - transaction.createdAt > timeoutSeconds * 1_000
  ) {
    throw new Error("login transaction is malformed or expired");
  }
  return transaction as LoginTransaction;
}

export function isTrustedMutationOrigin(request: Request, expectedOrigin: string): boolean {
  const origin = request.headers.get("origin");
  if (origin) return origin === expectedOrigin;
  return request.headers.get("sec-fetch-site") === "same-origin";
}
