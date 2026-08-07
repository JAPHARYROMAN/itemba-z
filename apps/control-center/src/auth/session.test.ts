import { describe, expect, it } from "vitest";
import {
  isTrustedMutationOrigin,
  sessionExpiryReason,
  shouldRefreshAccessToken,
  validateLoginTransaction,
  type BrowserSession,
} from "@/auth/session";

const now = 1_800_000_000_000;
const session: BrowserSession = {
  version: 1,
  sessionId: "session-1",
  subject: "actor-1",
  displayName: "Finance Operator",
  issuedAt: now - 1_000,
  lastActivityAt: now - 1_000,
  absoluteExpiresAt: now + 10_000,
  accessExpiresAt: now + 60_000,
  authenticationMethods: ["pwd", "otp"],
};

describe("browser session policy", () => {
  it("enforces absolute expiry before idle expiry", () => {
    expect(sessionExpiryReason({ ...session, absoluteExpiresAt: now }, now, 1_800)).toBe("absolute_timeout");
    expect(sessionExpiryReason({ ...session, lastActivityAt: now - 1_800_000 }, now, 1_800)).toBe("idle_timeout");
    expect(sessionExpiryReason(session, now, 1_800)).toBeNull();
  });

  it("renews only inside the configured access-token leeway", () => {
    expect(shouldRefreshAccessToken(session, now, 90)).toBe(true);
    expect(shouldRefreshAccessToken(session, now, 30)).toBe(false);
  });

  it("accepts only an exact mutation origin or same-origin browser metadata", () => {
    expect(isTrustedMutationOrigin(new Request("https://erp.example.com", { headers: { origin: "https://erp.example.com" } }), "https://erp.example.com")).toBe(true);
    expect(isTrustedMutationOrigin(new Request("https://erp.example.com", { headers: { origin: "https://evil.example" } }), "https://erp.example.com")).toBe(false);
    expect(isTrustedMutationOrigin(new Request("https://erp.example.com", { headers: { "sec-fetch-site": "same-origin" } }), "https://erp.example.com")).toBe(true);
  });

  it("rejects expired login transactions", () => {
    expect(() => validateLoginTransaction({
      version: 1,
      state: "state",
      nonce: "nonce",
      codeVerifier: "verifier",
      returnTo: "/sales",
      createdAt: now - 601_000,
      maxAgeSeconds: 28_800,
    }, now, 600)).toThrowError();
  });
});
