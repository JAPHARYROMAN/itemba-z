import type { TransitionOperationCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export async function POST(request: Request, { params }: { params: Promise<{ documentId: string }> }) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<TransitionOperationCommand>(request); const { documentId } = await params; const repository = await createServerRepository(); return liveResponse(await repository.transitionOperationDocument(documentId, body, key)); } catch (error) { return problemResponse(error); } }
