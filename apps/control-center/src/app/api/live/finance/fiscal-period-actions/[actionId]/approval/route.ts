import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
export async function POST(request: Request, context: { params: Promise<{ actionId: string }> }) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<{ reason: string }>(request); const { actionId } = await context.params; const repository = await createServerRepository(); return liveResponse(await repository.approveFiscalPeriodAction(actionId, body.reason, key), 201); } catch (error) { return problemResponse(error); } }
