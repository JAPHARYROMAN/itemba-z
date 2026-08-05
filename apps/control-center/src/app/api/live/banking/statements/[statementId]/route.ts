import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export async function GET(_request: Request, { params }: { params: Promise<{ statementId: string }> }) { try { const { statementId } = await params; const repository = await createServerRepository(); return liveResponse(await repository.getBankStatement(statementId)); } catch (error) { return problemResponse(error); } }
