import type { PublicProblem } from "@/live-api/types";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export type BackendIdentity =
  | { kind: "bearer"; token: string }
  | {
      kind: "development";
      actorId: string;
      tenantId: string;
      companyId: string;
      branchId: string;
      warehouseId: string;
    };

export interface IdentityEnvironment {
  deploymentEnvironment: string | undefined;
  bearerToken?: string;
  developmentEnabled?: string;
  actorId?: string;
  tenantId?: string;
  companyId?: string;
  branchId?: string;
  warehouseId?: string;
}

export class IdentityError extends Error {
  readonly problem: PublicProblem;

  constructor(code: string, detail: string, status = 401) {
    super(detail);
    this.name = "IdentityError";
    this.problem = {
      type: "urn:itemba-z:control-center:identity",
      title: status === 401 ? "Authentication required" : "Identity configuration invalid",
      status,
      code,
      detail,
    };
  }
}

function requiredUuid(value: string | undefined, name: string): string {
  if (!value || !UUID_PATTERN.test(value)) {
    throw new IdentityError(
      "development_identity_invalid",
      `${name} must be configured as a UUID when development identity headers are enabled.`,
      500,
    );
  }
  return value;
}

export function resolveBackendIdentity(environment: IdentityEnvironment): BackendIdentity {
  const token = environment.bearerToken?.trim();
  if (token) {
    if (/\s/.test(token)) {
      throw new IdentityError("bearer_token_invalid", "The server-side bearer session token is malformed.");
    }
    return { kind: "bearer", token };
  }

  if (environment.deploymentEnvironment !== "development") {
    throw new IdentityError(
      "oidc_session_required",
      "A server-side OIDC bearer session is required for live ERP access.",
    );
  }

  if (environment.developmentEnabled !== "true") {
    throw new IdentityError(
      "live_identity_unavailable",
      "Live ERP identity is not configured. Enable explicit development identity headers or sign in with OIDC.",
    );
  }

  return {
    kind: "development",
    actorId: requiredUuid(environment.actorId, "ITEMBA_DEV_ACTOR_ID"),
    tenantId: requiredUuid(environment.tenantId, "ITEMBA_DEV_TENANT_ID"),
    companyId: requiredUuid(environment.companyId, "ITEMBA_DEV_COMPANY_ID"),
    branchId: requiredUuid(environment.branchId, "ITEMBA_DEV_BRANCH_ID"),
    warehouseId: requiredUuid(environment.warehouseId, "ITEMBA_DEV_WAREHOUSE_ID"),
  };
}

export function buildIdentityHeaders(identity: BackendIdentity): Headers {
  const headers = new Headers();
  if (identity.kind === "bearer") {
    headers.set("Authorization", `Bearer ${identity.token}`);
    return headers;
  }

  headers.set("X-Actor-ID", identity.actorId);
  headers.set("X-Tenant-ID", identity.tenantId);
  headers.set("X-Company-ID", identity.companyId);
  headers.set("X-Branch-ID", identity.branchId);
  headers.set("X-Warehouse-ID", identity.warehouseId);
  return headers;
}
