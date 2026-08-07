import { describe, expect, it } from "vitest";
import { loadOidcRuntimeConfig, OidcConfigurationError, sanitizeReturnTo } from "@/auth/config";

function environment(overrides: Partial<NodeJS.ProcessEnv> = {}): NodeJS.ProcessEnv {
  return {
    NODE_ENV: "test",
    ITEMBA_ENV: "production",
    ITEMBA_OIDC_ISSUER_URL: "https://identity.example.com/tenant",
    ITEMBA_OIDC_CLIENT_ID: "itemba-control-center",
    ITEMBA_OIDC_CLIENT_SECRET: "not-a-real-secret",
    ITEMBA_OIDC_REQUIRED_ACR: "urn:itemba:loa:2",
    ITEMBA_OIDC_REQUIRED_AMR: "pwd otp",
    ITEMBA_CONTROL_CENTER_ORIGIN: "https://erp.example.com",
    ITEMBA_SESSION_ENCRYPTION_KEYS: `current:${Buffer.alloc(32, 7).toString("base64url")}`,
    ...overrides,
  };
}

describe("OIDC runtime configuration", () => {
  it("builds an HTTPS confidential-client configuration with bounded session policy", () => {
    const config = loadOidcRuntimeConfig(environment());
    expect(config.clientAuthMethod).toBe("client_secret_basic");
    expect(config.callbackUrl.href).toBe("https://erp.example.com/api/auth/callback");
    expect(config.idleTimeoutSeconds).toBe(1_800);
    expect(config.absoluteTimeoutSeconds).toBe(28_800);
    expect(config.requiredAssuranceLevel).toBe("urn:itemba:loa:2");
    expect(config.requiredAuthenticationMethods).toEqual(["pwd", "otp"]);
    expect(config.secureCookies).toBe(true);
  });

  it("fails closed for insecure production origins and issuers", () => {
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_CONTROL_CENTER_ORIGIN: "http://erp.example.com" })))
      .toThrowError(OidcConfigurationError);
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_OIDC_ISSUER_URL: "http://identity.example.com" })))
      .toThrowError(OidcConfigurationError);
  });

  it("requires a secret for confidential authentication and forbids one for a public client", () => {
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_OIDC_CLIENT_SECRET: "", ITEMBA_OIDC_CLIENT_AUTH_METHOD: "client_secret_post" })))
      .toThrowError(OidcConfigurationError);
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_OIDC_CLIENT_AUTH_METHOD: "none" })))
      .toThrowError(OidcConfigurationError);
  });

  it("rejects an idle timeout longer than the absolute session lifetime", () => {
    expect(() => loadOidcRuntimeConfig(environment({
      ITEMBA_SESSION_IDLE_TIMEOUT_SECONDS: "3600",
      ITEMBA_SESSION_ABSOLUTE_TIMEOUT_SECONDS: "1800",
    }))).toThrowError(OidcConfigurationError);
  });

  it("requires distinct cookies for access, session, and login transaction state", () => {
    expect(() => loadOidcRuntimeConfig(environment({
      ITEMBA_OIDC_COOKIE_NAME: "same-cookie",
      ITEMBA_OIDC_SESSION_COOKIE_NAME: "same-cookie",
    }))).toThrowError(OidcConfigurationError);
  });

  it("requires an explicit production assurance class and authentication methods", () => {
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_OIDC_REQUIRED_ACR: "" })))
      .toThrowError(OidcConfigurationError);
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_OIDC_REQUIRED_AMR: "" })))
      .toThrowError(OidcConfigurationError);
    expect(() => loadOidcRuntimeConfig(environment({ ITEMBA_OIDC_REQUIRED_AMR: "otp,<unsafe>" })))
      .toThrowError(OidcConfigurationError);
  });
});

describe("post-login return paths", () => {
  it("preserves local paths including query and fragment", () => {
    expect(sanitizeReturnTo("/sales/new?kind=cash#lines")).toBe("/sales/new?kind=cash#lines");
  });

  it("rejects protocol-relative, external, slash-confused, and control-character redirects", () => {
    for (const unsafe of ["//evil.example", "https://evil.example", "/\\evil", "/sales\nadmin"]) {
      expect(sanitizeReturnTo(unsafe)).toBe("/");
    }
  });
});
