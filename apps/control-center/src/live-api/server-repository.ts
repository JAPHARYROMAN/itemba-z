import "server-only";

import { cookies, headers } from "next/headers";
import { resolveBackendIdentity } from "@/live-api/auth";
import { ItembaApiClient, LiveApiError } from "@/live-api/api-client";

const DEFAULT_COOKIE_NAME = "itemba_oidc_access_token";

function bearerFromAuthorization(value: string | null): string | undefined {
  if (!value) return undefined;
  const match = /^Bearer ([^\s]+)$/i.exec(value);
  return match?.[1];
}

export async function createServerRepository(): Promise<ItembaApiClient> {
  const [cookieStore, requestHeaders] = await Promise.all([cookies(), headers()]);
  const cookieName = process.env.ITEMBA_OIDC_COOKIE_NAME?.trim() || DEFAULT_COOKIE_NAME;
  const bearerToken = bearerFromAuthorization(requestHeaders.get("authorization")) ?? cookieStore.get(cookieName)?.value;
  const identity = resolveBackendIdentity({
    deploymentEnvironment: process.env.ITEMBA_ENV,
    bearerToken,
    developmentEnabled: process.env.ITEMBA_DEV_IDENTITY_ENABLED,
    actorId: process.env.ITEMBA_DEV_ACTOR_ID,
    tenantId: process.env.ITEMBA_DEV_TENANT_ID,
    companyId: process.env.ITEMBA_DEV_COMPANY_ID,
    branchId: process.env.ITEMBA_DEV_BRANCH_ID,
    warehouseId: process.env.ITEMBA_DEV_WAREHOUSE_ID,
  });

  const baseUrl = process.env.ITEMBA_API_BASE_URL?.trim();
  if (!baseUrl) {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:configuration",
      title: "Live API not configured",
      status: 503,
      code: "api_base_url_missing",
      detail: "ITEMBA_API_BASE_URL is required before live ERP data can be loaded.",
    });
  }
  return new ItembaApiClient({ baseUrl, identity });
}
