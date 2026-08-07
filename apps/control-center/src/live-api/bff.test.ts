import { describe, expect, it } from "vitest";
import { assertSameOrigin, requireIdempotencyKey } from "@/live-api/bff";
import { LiveApiError } from "@/live-api/api-client";

describe("live BFF command controls", () => {
  it("requires and preserves a client retry idempotency key", () => {
    const key = "00000000-0000-4000-8000-000000000020";
    const request = new Request("http://localhost:3000/api/live/sales", { headers: { "Idempotency-Key": key } });
    expect(requireIdempotencyKey(request)).toBe(key);
    expect(() => requireIdempotencyKey(new Request("http://localhost:3000/api/live/sales"))).toThrowError(LiveApiError);
  });

  it("rejects cross-origin mutation requests before API access", () => {
    const request = new Request("http://localhost:3000/api/live/sales", { headers: { Origin: "https://attacker.example", "Sec-Fetch-Site": "cross-site" } });
    expect(() => assertSameOrigin(request)).toThrowError(LiveApiError);
  });
});
