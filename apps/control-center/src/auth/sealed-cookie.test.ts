import { describe, expect, it } from "vitest";
import { parseEncryptionKeys, sealCookie, SealedCookieError, unsealCookie } from "@/auth/sealed-cookie";

const oldKey = `old:${Buffer.alloc(32, 3).toString("base64url")}`;
const currentKey = `current:${Buffer.alloc(32, 9).toString("base64url")}`;

describe("authenticated session cookies", () => {
  it("round-trips a payload without exposing its plaintext", () => {
    const sealed = sealCookie({ refreshToken: "sensitive-token", subject: "actor-1" }, "browser-session", currentKey);
    expect(sealed).not.toContain("sensitive-token");
    expect(unsealCookie(sealed, "browser-session", currentKey)).toEqual({ refreshToken: "sensitive-token", subject: "actor-1" });
  });

  it("supports decrypt-only previous keys during rotation", () => {
    const sealedWithOldKey = sealCookie({ subject: "actor-1" }, "browser-session", oldKey);
    expect(unsealCookie(sealedWithOldKey, "browser-session", `${currentKey},${oldKey}`)).toEqual({ subject: "actor-1" });
    expect(sealCookie({ subject: "actor-2" }, "browser-session", `${currentKey},${oldKey}`).split(".")[1]).toBe("current");
  });

  it("rejects tampering and cross-purpose replay", () => {
    const sealed = sealCookie({ subject: "actor-1" }, "browser-session", currentKey);
    const parts = sealed.split(".");
    parts[3] = `${parts[3][0] === "A" ? "B" : "A"}${parts[3].slice(1)}`;
    const tampered = parts.join(".");
    expect(() => unsealCookie(tampered, "browser-session", currentKey)).toThrowError(SealedCookieError);
    expect(() => unsealCookie(sealed, "login-transaction", currentKey)).toThrowError(SealedCookieError);
  });

  it("rejects weak, duplicate, and malformed key-ring entries", () => {
    expect(() => parseEncryptionKeys("current:c2hvcnQ")).toThrowError(SealedCookieError);
    expect(() => parseEncryptionKeys(`${currentKey},${currentKey}`)).toThrowError(SealedCookieError);
    expect(() => parseEncryptionKeys("missing-separator")).toThrowError(SealedCookieError);
  });
});
