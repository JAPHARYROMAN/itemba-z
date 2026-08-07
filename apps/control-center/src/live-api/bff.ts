import { publicProblem } from "@/live-api/errors";
import { LiveApiError } from "@/live-api/api-client";

function response(problem: ReturnType<typeof publicProblem>): Response {
  return Response.json(problem, {
    status: Math.min(Math.max(problem.status, 400), 599),
    headers: { "Cache-Control": "no-store", "Content-Type": "application/problem+json" },
  });
}

export function problemResponse(error: unknown): Response {
  return response(publicProblem(error));
}

export function assertSameOrigin(request: Request): void {
  const origin = request.headers.get("origin");
  const fetchSite = request.headers.get("sec-fetch-site");
  const configuredOrigin = process.env.ITEMBA_CONTROL_CENTER_ORIGIN?.replace(/\/$/, "");
  const expectedOrigin = configuredOrigin || new URL(request.url).origin;

  if (fetchSite && fetchSite !== "same-origin" && fetchSite !== "none") {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:csrf",
      title: "Cross-site request rejected",
      status: 403,
      code: "cross_site_request_rejected",
      detail: "Live transaction commands must originate from the Control Center.",
    });
  }
  if ((process.env.NODE_ENV === "production" && !origin) || (origin && origin.replace(/\/$/, "") !== expectedOrigin)) {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:csrf",
      title: "Request origin rejected",
      status: 403,
      code: "request_origin_rejected",
      detail: "The transaction command did not include the expected Control Center origin.",
    });
  }
}

export function requireIdempotencyKey(request: Request): string {
  const key = request.headers.get("idempotency-key")?.trim();
  if (!key || key.length < 16 || key.length > 128 || /[\r\n]/.test(key)) {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:idempotency",
      title: "Idempotency key required",
      status: 400,
      code: "idempotency_key_required",
      detail: "A 16 to 128 character Idempotency-Key is required for transaction commands.",
    });
  }
  return key;
}

export async function requireJsonBody<T>(request: Request): Promise<T> {
  if (!request.headers.get("content-type")?.toLowerCase().startsWith("application/json")) {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:request",
      title: "JSON body required",
      status: 415,
      code: "content_type_invalid",
      detail: "Transaction commands must use application/json.",
    });
  }
  try {
    return await request.json() as T;
  } catch {
    throw new LiveApiError({
      type: "urn:itemba-z:control-center:request",
      title: "JSON body invalid",
      status: 400,
      code: "request_json_invalid",
      detail: "The transaction command body is not valid JSON.",
    });
  }
}

export function liveResponse<T>(value: T, status = 200): Response {
  return Response.json(value, { status, headers: { "Cache-Control": "no-store" } });
}
