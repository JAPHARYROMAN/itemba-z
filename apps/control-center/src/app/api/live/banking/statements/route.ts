import type { ImportBankStatementCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
export async function GET(request: Request) { try { const repository = await createServerRepository(); const cursor = new URL(request.url).searchParams.get("cursor") ?? undefined; return liveResponse(await repository.listBankStatements(cursor)); } catch (error) { return problemResponse(error); } }
export async function POST(request: Request) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<ImportBankStatementCommand>(request); const repository = await createServerRepository(); return liveResponse(await repository.importBankStatement(body, key), 201); } catch (error) { return problemResponse(error); } }
