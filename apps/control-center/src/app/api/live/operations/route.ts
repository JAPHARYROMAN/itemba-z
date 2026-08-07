import type { CreateOperationCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
export async function GET(request: Request) { try { const url = new URL(request.url); const repository = await createServerRepository(); return liveResponse(await repository.listOperationDocuments((url.searchParams.get("type") || undefined) as never, url.searchParams.get("cursor") || undefined)); } catch (error) { return problemResponse(error); } }
export async function POST(request: Request) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<CreateOperationCommand>(request); const repository = await createServerRepository(); return liveResponse(await repository.createOperationDocument(body, key), 201); } catch (error) { return problemResponse(error); } }
