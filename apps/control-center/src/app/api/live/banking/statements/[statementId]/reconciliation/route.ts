import type { ReconcileBankStatementCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export async function POST(request: Request, { params }: { params: Promise<{ statementId: string }> }) { try { assertSameOrigin(request); const key = requireIdempotencyKey(request); const body = await requireJsonBody<ReconcileBankStatementCommand>(request); const { statementId } = await params; const repository = await createServerRepository(); return liveResponse(await repository.reconcileBankStatement(statementId, body, key), 201); } catch (error) { return problemResponse(error); } }
