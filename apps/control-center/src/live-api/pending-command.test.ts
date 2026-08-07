import { beforeEach, describe, expect, it } from "vitest";
import {
  clearPendingCommandAfterSuccess,
  markPendingCommandRejected,
  PENDING_COMMAND_REVIEW_AFTER_MS,
  PendingCommandConflictError,
  readPendingCommand,
  reservePendingCommand,
  saleCompletionPendingScope,
  saleReversalPendingScope,
} from "@/live-api/pending-command";

const scope = "sales:create";
const firstPayload = { customer_id: "customer-1", lines: [{ product_id: "product-1", quantity: 1 }] };

describe("pending command recovery", () => {
  beforeEach(() => window.sessionStorage.clear());

  it("retains the original key after the review threshold instead of silently expiring it", () => {
    const first = reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(firstPayload),
      payload: firstPayload,
      now: 1_000,
      createKey: () => "00000000-0000-4000-8000-000000000001",
    });
    const recovered = reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(firstPayload),
      payload: firstPayload,
      now: 1_000 + PENDING_COMMAND_REVIEW_AFTER_MS + 1,
      createKey: () => "00000000-0000-4000-8000-000000000002",
    });

    expect(recovered.expired).toBe(true);
    expect(recovered.record.key).toBe(first.record.key);
    expect(readPendingCommand(window.sessionStorage, scope, recovered.record.expiresAt + 1)?.record.key).toBe(first.record.key);
  });

  it("blocks a changed ambiguous command but permits correction after a definite rejection with the same key", () => {
    const first = reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(firstPayload),
      payload: firstPayload,
      createKey: () => "00000000-0000-4000-8000-000000000003",
    });
    const correctedPayload = { ...firstPayload, lines: [{ product_id: "product-1", quantity: 2 }] };

    expect(() => reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(correctedPayload),
      payload: correctedPayload,
    })).toThrow(PendingCommandConflictError);

    markPendingCommandRejected(window.sessionStorage, scope, first.record.key, 422);
    const corrected = reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(correctedPayload),
      payload: correctedPayload,
      createKey: () => "00000000-0000-4000-8000-000000000004",
    });
    expect(corrected.record.key).toBe(first.record.key);
    expect(corrected.record.payload).toEqual(correctedPayload);
  });

  it("clears only the matching successful command so an intentional identical command gets a new key", () => {
    const first = reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(firstPayload),
      payload: firstPayload,
      createKey: () => "00000000-0000-4000-8000-000000000005",
    });
    clearPendingCommandAfterSuccess(window.sessionStorage, scope, first.record.key);
    expect(readPendingCommand(window.sessionStorage, scope)).toBeNull();

    const next = reservePendingCommand({
      storage: window.sessionStorage,
      scope,
      fingerprint: JSON.stringify(firstPayload),
      payload: firstPayload,
      createKey: () => "00000000-0000-4000-8000-000000000006",
    });
    expect(next.record.key).not.toBe(first.record.key);
  });

  it("namespaces completion and reversal commands by actor and every organization boundary", () => {
    const owner = {
      actor_id: "00000000-0000-4000-8000-000000000001",
      tenant_id: "00000000-0000-4000-8000-000000000002",
      company_id: "00000000-0000-4000-8000-000000000003",
      branch_id: "00000000-0000-4000-8000-000000000004",
      warehouse_id: "00000000-0000-4000-8000-000000000005",
    };
    const baseScope = saleCompletionPendingScope(owner);
    const switchedScopes = [
      saleCompletionPendingScope({ ...owner, actor_id: "00000000-0000-4000-8000-000000000011" }),
      saleCompletionPendingScope({ ...owner, tenant_id: "00000000-0000-4000-8000-000000000012" }),
      saleCompletionPendingScope({ ...owner, company_id: "00000000-0000-4000-8000-000000000013" }),
      saleCompletionPendingScope({ ...owner, branch_id: "00000000-0000-4000-8000-000000000014" }),
      saleCompletionPendingScope({ ...owner, warehouse_id: "00000000-0000-4000-8000-000000000015" }),
    ];

    expect(new Set([baseScope, ...switchedScopes])).toHaveLength(6);
    expect(saleReversalPendingScope(owner, "sale-1")).not.toBe(saleReversalPendingScope(owner, "sale-2"));
  });
});
