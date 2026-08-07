import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";

const KEY_BYTES = 32;
const IV_BYTES = 12;
const TAG_BYTES = 16;
const MAX_COOKIE_VALUE_BYTES = 3_800;

export class SealedCookieError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "SealedCookieError";
    this.code = code;
  }
}

interface EncryptionKey {
  id: string;
  bytes: Buffer;
}

function decodeBase64Url(value: string): Buffer {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) throw new SealedCookieError("session_key_invalid", "Session encryption key must use base64url encoding.");
  return Buffer.from(value, "base64url");
}

export function parseEncryptionKeys(serialized: string): EncryptionKey[] {
  const entries = serialized.split(",").map((entry) => entry.trim()).filter(Boolean);
  if (entries.length === 0) throw new SealedCookieError("session_key_missing", "At least one session encryption key is required.");
  const seen = new Set<string>();
  return entries.map((entry) => {
    const separator = entry.indexOf(":");
    if (separator <= 0 || separator === entry.length - 1) {
      throw new SealedCookieError("session_key_invalid", "Session encryption keys must use kid:base64url-key format.");
    }
    const id = entry.slice(0, separator);
    if (!/^[A-Za-z0-9_-]{1,32}$/.test(id) || seen.has(id)) {
      throw new SealedCookieError("session_key_invalid", "Session encryption key IDs must be unique URL-safe values.");
    }
    seen.add(id);
    const bytes = decodeBase64Url(entry.slice(separator + 1));
    if (bytes.length !== KEY_BYTES) throw new SealedCookieError("session_key_invalid", "Each session encryption key must decode to exactly 32 bytes.");
    return { id, bytes };
  });
}

function associatedData(purpose: string, keyId: string): Buffer {
  return Buffer.from(`itemba-z:v1:${purpose}:${keyId}`, "utf8");
}

export function sealCookie<T>(payload: T, purpose: string, serializedKeys: string): string {
  const [activeKey] = parseEncryptionKeys(serializedKeys);
  const iv = randomBytes(IV_BYTES);
  const cipher = createCipheriv("aes-256-gcm", activeKey.bytes, iv, { authTagLength: TAG_BYTES });
  cipher.setAAD(associatedData(purpose, activeKey.id));
  const plaintext = Buffer.from(JSON.stringify(payload), "utf8");
  const ciphertext = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const value = ["v1", activeKey.id, iv.toString("base64url"), ciphertext.toString("base64url"), cipher.getAuthTag().toString("base64url")].join(".");
  if (Buffer.byteLength(value, "utf8") > MAX_COOKIE_VALUE_BYTES) {
    throw new SealedCookieError("session_cookie_too_large", "Encrypted session state exceeds the safe cookie size limit.");
  }
  return value;
}

export function unsealCookie<T>(value: string, purpose: string, serializedKeys: string): T {
  const [version, keyId, encodedIv, encodedCiphertext, encodedTag, ...remainder] = value.split(".");
  if (version !== "v1" || !keyId || !encodedIv || !encodedCiphertext || !encodedTag || remainder.length > 0) {
    throw new SealedCookieError("session_cookie_invalid", "Encrypted session state has an invalid envelope.");
  }
  const key = parseEncryptionKeys(serializedKeys).find((candidate) => candidate.id === keyId);
  if (!key) throw new SealedCookieError("session_cookie_key_unknown", "Encrypted session state references an unavailable key.");
  try {
    const iv = decodeBase64Url(encodedIv);
    const tag = decodeBase64Url(encodedTag);
    if (iv.length !== IV_BYTES || tag.length !== TAG_BYTES) throw new Error("invalid authenticated encryption parameters");
    const decipher = createDecipheriv("aes-256-gcm", key.bytes, iv, { authTagLength: TAG_BYTES });
    decipher.setAAD(associatedData(purpose, key.id));
    decipher.setAuthTag(tag);
    const plaintext = Buffer.concat([decipher.update(decodeBase64Url(encodedCiphertext)), decipher.final()]);
    return JSON.parse(plaintext.toString("utf8")) as T;
  } catch {
    throw new SealedCookieError("session_cookie_invalid", "Encrypted session state failed authentication.");
  }
}
