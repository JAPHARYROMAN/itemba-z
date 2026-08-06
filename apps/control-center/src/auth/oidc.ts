import "server-only";

import { randomUUID } from "node:crypto";
import * as oidc from "openid-client";
import type { OidcRuntimeConfig } from "@/auth/config";
import type { BrowserSession, LoginTransaction } from "@/auth/session";

interface IssuedSession {
  accessToken: string;
  session: BrowserSession;
}

const configurations = new Map<string, Promise<oidc.Configuration>>();

function clientAuthentication(config: OidcRuntimeConfig): oidc.ClientAuth {
  switch (config.clientAuthMethod) {
    case "none": return oidc.None();
    case "client_secret_post": return oidc.ClientSecretPost(config.clientSecret);
    case "client_secret_basic": return oidc.ClientSecretBasic(config.clientSecret);
  }
}

async function discover(config: OidcRuntimeConfig): Promise<oidc.Configuration> {
  const metadata: Partial<oidc.ClientMetadata> = {
    redirect_uris: [config.callbackUrl.href],
    response_types: ["code"],
    token_endpoint_auth_method: config.clientAuthMethod,
  };
  if (config.clientSecret) metadata.client_secret = config.clientSecret;
  const discovered = await oidc.discovery(config.issuer, config.clientId, metadata, clientAuthentication(config));
  discovered.timeout = 10;
  return discovered;
}

export async function getOidcConfiguration(config: OidcRuntimeConfig): Promise<oidc.Configuration> {
  const key = `${config.issuer.href}|${config.clientId}|${config.clientAuthMethod}`;
  let pending = configurations.get(key);
  if (!pending) {
    pending = discover(config);
    configurations.set(key, pending);
    pending.catch(() => configurations.delete(key));
  }
  return pending;
}

export async function beginAuthorization(
  config: OidcRuntimeConfig,
  returnTo: string,
  forceReauthentication: boolean,
  now = Date.now(),
): Promise<{ redirectUrl: URL; transaction: LoginTransaction }> {
  const provider = await getOidcConfiguration(config);
  const codeVerifier = oidc.randomPKCECodeVerifier();
  const state = oidc.randomState();
  const nonce = oidc.randomNonce();
  const parameters: Record<string, string> = {
    redirect_uri: config.callbackUrl.href,
    response_type: "code",
    scope: config.scopes,
    code_challenge: await oidc.calculatePKCECodeChallenge(codeVerifier),
    code_challenge_method: "S256",
    state,
    nonce,
    max_age: forceReauthentication ? "0" : String(config.authenticationMaxAgeSeconds),
    acr_values: config.requiredAssuranceLevel,
  };
  if (forceReauthentication) parameters.prompt = "login";
  if (config.audience) parameters.audience = config.audience;
  return {
    redirectUrl: oidc.buildAuthorizationUrl(provider, parameters),
    transaction: {
      version: 1,
      state,
      nonce,
      codeVerifier,
      returnTo,
      createdAt: now,
      maxAgeSeconds: forceReauthentication ? 0 : config.authenticationMaxAgeSeconds,
    },
  };
}

function requireBearerTokens(tokens: oidc.TokenEndpointResponse & oidc.TokenEndpointResponseHelpers): { accessToken: string; expiresIn: number } {
  const expiresIn = tokens.expiresIn();
  if (!tokens.access_token || tokens.token_type?.toLowerCase() !== "bearer" || !expiresIn || expiresIn <= 0) {
    throw new Error("OIDC provider did not issue a usable expiring Bearer access token");
  }
  return { accessToken: tokens.access_token, expiresIn };
}

function stringClaim(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

export async function completeAuthorization(
  config: OidcRuntimeConfig,
  currentUrl: URL,
  transaction: LoginTransaction,
  now = Date.now(),
): Promise<IssuedSession> {
  const provider = await getOidcConfiguration(config);
  const tokens = await oidc.authorizationCodeGrant(provider, currentUrl, {
    pkceCodeVerifier: transaction.codeVerifier,
    expectedState: transaction.state,
    expectedNonce: transaction.nonce,
    idTokenExpected: true,
    maxAge: transaction.maxAgeSeconds,
  });
  const { accessToken, expiresIn } = requireBearerTokens(tokens);
  const claims = tokens.claims();
  const subject = stringClaim(claims?.sub);
  if (!subject) throw new Error("OIDC ID token is missing a subject");
  const displayName = stringClaim(claims?.name) ?? stringClaim(claims?.preferred_username) ?? stringClaim(claims?.email) ?? subject;
  const authenticationMethods = Array.isArray(claims?.amr) ? claims.amr.filter((method): method is string => typeof method === "string") : [];
  if (stringClaim(claims?.acr) !== config.requiredAssuranceLevel) {
    throw new Error("OIDC authentication did not satisfy the required assurance level");
  }
  if (config.requiredAuthenticationMethods.some((method) => !authenticationMethods.includes(method))) {
    throw new Error("OIDC authentication did not satisfy the required authentication methods");
  }
  if (typeof claims?.auth_time !== "number" || now - claims.auth_time * 1_000 > config.authenticationMaxAgeSeconds * 1_000) {
    throw new Error("OIDC authentication time is missing or too old");
  }

  return {
    accessToken,
    session: {
      version: 1,
      sessionId: randomUUID(),
      subject,
      displayName,
      email: stringClaim(claims?.email),
      refreshToken: tokens.refresh_token,
      issuedAt: now,
      lastActivityAt: now,
      absoluteExpiresAt: now + config.absoluteTimeoutSeconds * 1_000,
      accessExpiresAt: now + expiresIn * 1_000,
      authenticationTime: typeof claims?.auth_time === "number" ? claims.auth_time * 1_000 : undefined,
      assuranceLevel: config.requiredAssuranceLevel,
      authenticationMethods,
    },
  };
}

export async function refreshBrowserSession(
  config: OidcRuntimeConfig,
  session: BrowserSession,
  now = Date.now(),
): Promise<IssuedSession> {
  if (!session.refreshToken) throw new Error("OIDC session has no refresh token");
  const provider = await getOidcConfiguration(config);
  const tokens = await oidc.refreshTokenGrant(provider, session.refreshToken);
  const { accessToken, expiresIn } = requireBearerTokens(tokens);
  const claims = tokens.claims();
  if (claims?.sub && claims.sub !== session.subject) throw new Error("OIDC refresh changed the authenticated subject");
  return {
    accessToken,
    session: {
      ...session,
      refreshToken: tokens.refresh_token ?? session.refreshToken,
      lastActivityAt: now,
      accessExpiresAt: now + expiresIn * 1_000,
    },
  };
}

export async function revokeBrowserSession(config: OidcRuntimeConfig, session: BrowserSession, accessToken?: string): Promise<void> {
  const provider = await getOidcConfiguration(config);
  if (!provider.serverMetadata().revocation_endpoint) return;
  const revocations: Promise<void>[] = [];
  if (session.refreshToken) revocations.push(oidc.tokenRevocation(provider, session.refreshToken, { token_type_hint: "refresh_token" }));
  if (accessToken) revocations.push(oidc.tokenRevocation(provider, accessToken, { token_type_hint: "access_token" }));
  await Promise.allSettled(revocations);
}

export async function providerLogoutUrl(config: OidcRuntimeConfig): Promise<URL | null> {
  const provider = await getOidcConfiguration(config);
  if (!provider.serverMetadata().end_session_endpoint) return null;
  return oidc.buildEndSessionUrl(provider, { post_logout_redirect_uri: config.postLogoutUrl.href });
}
