import { describe, expect, it } from "vitest";
import { buildIdentityHeaders, IdentityError, resolveBackendIdentity } from "@/live-api/auth";

const developmentIds = {
  actorId: "00000000-0000-4000-8000-000000000001",
  tenantId: "00000000-0000-4000-8000-000000000002",
  companyId: "00000000-0000-4000-8000-000000000003",
  branchId: "00000000-0000-4000-8000-000000000004",
  warehouseId: "00000000-0000-4000-8000-000000000005",
};

describe("live API identity", () => {
  it("fails closed outside ITEMBA_ENV=development when no bearer session exists", () => {
    for (const deploymentEnvironment of [undefined, "staging", "production"]) {
      expect(() => resolveBackendIdentity({ deploymentEnvironment, developmentEnabled: "true", ...developmentIds }))
        .toThrowError(IdentityError);
    }
  });

  it("uses a bearer session in production and never adds development scope headers", () => {
    const identity = resolveBackendIdentity({ deploymentEnvironment: "production", bearerToken: "signed.oidc.jwt" });
    const headers = buildIdentityHeaders(identity);
    expect(headers.get("Authorization")).toBe("Bearer signed.oidc.jwt");
    expect(headers.get("X-Tenant-ID")).toBeNull();
  });

  it("allows an explicitly configured development identity in a standalone NODE_ENV=production runtime", () => {
    const identity = resolveBackendIdentity({ deploymentEnvironment: "development", developmentEnabled: "true", ...developmentIds });
    const headers = buildIdentityHeaders(identity);
    expect(headers.get("Authorization")).toBeNull();
    expect(headers.get("X-Actor-ID")).toBe(developmentIds.actorId);
    expect(headers.get("X-Tenant-ID")).toBe(developmentIds.tenantId);
    expect(headers.get("X-Company-ID")).toBe(developmentIds.companyId);
    expect(headers.get("X-Branch-ID")).toBe(developmentIds.branchId);
    expect(headers.get("X-Warehouse-ID")).toBe(developmentIds.warehouseId);
  });
});
