import type { GovernanceDecisionCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export async function POST(request: Request, context: { params: Promise<{ mappingId: string }> }) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<GovernanceDecisionCommand>(request); const { mappingId } = await context.params; const repository = await createServerRepository(); return liveResponse(await repository.decidePostingMapping(mappingId, body, key)); } catch (error) { return problemResponse(error); } }
