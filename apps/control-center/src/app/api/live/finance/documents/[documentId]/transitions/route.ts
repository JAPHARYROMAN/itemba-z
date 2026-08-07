import type { TransitionFinancialDocumentCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
export async function POST(request: Request, context: { params: Promise<{ documentId: string }> }) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<TransitionFinancialDocumentCommand>(request); const { documentId } = await context.params; const repository = await createServerRepository(); return liveResponse(await repository.transitionFinancialDocument(documentId, body, key), 201); } catch (error) { return problemResponse(error); } }
