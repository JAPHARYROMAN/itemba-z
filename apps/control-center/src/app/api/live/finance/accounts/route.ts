import type { CreateGLAccountCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";
export async function GET() { try { const repository = await createServerRepository(); return liveResponse(await repository.listGLAccounts()); } catch (error) { return problemResponse(error); } }
export async function POST(request: Request) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<CreateGLAccountCommand>(request); const repository = await createServerRepository(); return liveResponse(await repository.createGLAccount(body, key), 201); } catch (error) { return problemResponse(error); } }
