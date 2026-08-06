export type OidcClientAuthMethod = "none" | "client_secret_basic" | "client_secret_post";

export interface OidcRuntimeConfig {
  deploymentEnvironment: string;
  issuer: URL;
  clientId: string;
  clientSecret?: string;
  clientAuthMethod: OidcClientAuthMethod;
  controlCenterOrigin: URL;
  callbackUrl: URL;
  postLogoutUrl: URL;
  scopes: string;
  audience?: string;
  idleTimeoutSeconds: number;
  absoluteTimeoutSeconds: number;
  authenticationMaxAgeSeconds: number;
  refreshLeewaySeconds: number;
  transactionTimeoutSeconds: number;
  secureCookies: boolean;
  accessCookieName: string;
  sessionCookieName: string;
  transactionCookieName: string;
  encryptionKeys: string;
}

export class OidcConfigurationError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "OidcConfigurationError";
    this.code = code;
  }
}

function required(environment: NodeJS.ProcessEnv, name: string): string {
  const value = environment[name]?.trim();
  if (!value) throw new OidcConfigurationError("oidc_configuration_missing", `${name} is required.`);
  return value;
}

function positiveInteger(environment: NodeJS.ProcessEnv, name: string, defaultValue: number): number {
  const raw = environment[name]?.trim();
  if (!raw) return defaultValue;
  if (!/^\d+$/.test(raw)) {
    throw new OidcConfigurationError("oidc_configuration_invalid", `${name} must be a positive integer.`);
  }
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new OidcConfigurationError("oidc_configuration_invalid", `${name} must be a positive integer.`);
  }
  return value;
}

function parseOrigin(value: string, deploymentEnvironment: string): URL {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_CONTROL_CENTER_ORIGIN must be an absolute URL.");
  }
  if (url.origin !== value.replace(/\/$/, "") || url.username || url.password || url.search || url.hash) {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_CONTROL_CENTER_ORIGIN must contain only a URL origin.");
  }
  if (deploymentEnvironment !== "development" && url.protocol !== "https:") {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_CONTROL_CENTER_ORIGIN must use HTTPS outside development.");
  }
  return new URL(url.origin);
}

function parseIssuer(value: string, deploymentEnvironment: string): URL {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_OIDC_ISSUER_URL must be an absolute URL.");
  }
  if (url.username || url.password || url.search || url.hash) {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_OIDC_ISSUER_URL must not contain credentials, a query, or a fragment.");
  }
  if (deploymentEnvironment !== "development" && url.protocol !== "https:") {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_OIDC_ISSUER_URL must use HTTPS outside development.");
  }
  return url;
}

function parseClientAuthMethod(environment: NodeJS.ProcessEnv, clientSecret?: string): OidcClientAuthMethod {
  const raw = environment.ITEMBA_OIDC_CLIENT_AUTH_METHOD?.trim() || (clientSecret ? "client_secret_basic" : "none");
  if (raw !== "none" && raw !== "client_secret_basic" && raw !== "client_secret_post") {
    throw new OidcConfigurationError(
      "oidc_configuration_invalid",
      "ITEMBA_OIDC_CLIENT_AUTH_METHOD must be none, client_secret_basic, or client_secret_post.",
    );
  }
  if (raw !== "none" && !clientSecret) {
    throw new OidcConfigurationError("oidc_configuration_missing", "ITEMBA_OIDC_CLIENT_SECRET is required for confidential client authentication.");
  }
  if (raw === "none" && clientSecret) {
    throw new OidcConfigurationError("oidc_configuration_invalid", "A public OIDC client must not configure ITEMBA_OIDC_CLIENT_SECRET.");
  }
  return raw;
}

function cookieName(environment: NodeJS.ProcessEnv, name: string, fallback: string): string {
  const value = environment[name]?.trim() || fallback;
  if (!/^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(value)) {
    throw new OidcConfigurationError("oidc_configuration_invalid", `${name} is not a valid cookie name.`);
  }
  return value;
}

export function loadOidcRuntimeConfig(environment: NodeJS.ProcessEnv = process.env): OidcRuntimeConfig {
  const deploymentEnvironment = environment.ITEMBA_ENV?.trim() || "production";
  const issuer = parseIssuer(required(environment, "ITEMBA_OIDC_ISSUER_URL"), deploymentEnvironment);
  const controlCenterOrigin = parseOrigin(required(environment, "ITEMBA_CONTROL_CENTER_ORIGIN"), deploymentEnvironment);
  const clientSecret = environment.ITEMBA_OIDC_CLIENT_SECRET?.trim() || undefined;
  const clientAuthMethod = parseClientAuthMethod(environment, clientSecret);
  const scopes = environment.ITEMBA_OIDC_SCOPES?.trim() || "openid profile email offline_access";
  if (!scopes.split(/\s+/).includes("openid")) {
    throw new OidcConfigurationError("oidc_configuration_invalid", "ITEMBA_OIDC_SCOPES must include openid.");
  }

  const idleTimeoutSeconds = positiveInteger(environment, "ITEMBA_SESSION_IDLE_TIMEOUT_SECONDS", 1_800);
  const absoluteTimeoutSeconds = positiveInteger(environment, "ITEMBA_SESSION_ABSOLUTE_TIMEOUT_SECONDS", 28_800);
  if (idleTimeoutSeconds > absoluteTimeoutSeconds) {
    throw new OidcConfigurationError("oidc_configuration_invalid", "The session idle timeout cannot exceed the absolute timeout.");
  }

  const accessCookieName = cookieName(environment, "ITEMBA_OIDC_COOKIE_NAME", "itemba_oidc_access_token");
  const sessionCookieName = cookieName(environment, "ITEMBA_OIDC_SESSION_COOKIE_NAME", "itemba_oidc_session");
  const transactionCookieName = cookieName(environment, "ITEMBA_OIDC_TRANSACTION_COOKIE_NAME", "itemba_oidc_transaction");
  if (new Set([accessCookieName, sessionCookieName, transactionCookieName]).size !== 3) {
    throw new OidcConfigurationError("oidc_configuration_invalid", "OIDC access, session, and transaction cookie names must be distinct.");
  }

  return {
    deploymentEnvironment,
    issuer,
    clientId: required(environment, "ITEMBA_OIDC_CLIENT_ID"),
    clientSecret,
    clientAuthMethod,
    controlCenterOrigin,
    callbackUrl: new URL("/api/auth/callback", controlCenterOrigin),
    postLogoutUrl: new URL("/login?signedOut=1", controlCenterOrigin),
    scopes,
    audience: environment.ITEMBA_OIDC_AUDIENCE?.trim() || undefined,
    idleTimeoutSeconds,
    absoluteTimeoutSeconds,
    authenticationMaxAgeSeconds: positiveInteger(environment, "ITEMBA_OIDC_AUTH_MAX_AGE_SECONDS", 28_800),
    refreshLeewaySeconds: positiveInteger(environment, "ITEMBA_SESSION_REFRESH_LEEWAY_SECONDS", 90),
    transactionTimeoutSeconds: positiveInteger(environment, "ITEMBA_OIDC_TRANSACTION_TIMEOUT_SECONDS", 600),
    secureCookies: deploymentEnvironment !== "development",
    accessCookieName,
    sessionCookieName,
    transactionCookieName,
    encryptionKeys: required(environment, "ITEMBA_SESSION_ENCRYPTION_KEYS"),
  };
}

export function sanitizeReturnTo(value: string | null | undefined): string {
  if (!value) return "/";
  if (!value.startsWith("/") || value.startsWith("//") || /[\u0000-\u001f\u007f\\]/.test(value)) return "/";
  try {
    const parsed = new URL(value, "https://itemba.invalid");
    return parsed.origin === "https://itemba.invalid" ? `${parsed.pathname}${parsed.search}${parsed.hash}` : "/";
  } catch {
    return "/";
  }
}
