import type { GovernanceDecisionCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export async function POST(request: Request, context: { params: Promise<{ accountId: string }> }) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<GovernanceDecisionCommand>(request); const { accountId } = await context.params; const repository = await createServerRepository(); return liveResponse(await repository.decideGLAccount(accountId, body, key)); } catch (error) { return problemResponse(error); } }
