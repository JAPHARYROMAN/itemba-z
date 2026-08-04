import type { ResolveMobileReconciliationCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function POST(request: Request, { params }: { params: Promise<{ caseId: string }> }) {
  try {
    assertSameOrigin(request);
    const idempotencyKey = requireIdempotencyKey(request);
    const command = await requireJsonBody<ResolveMobileReconciliationCommand>(request);
    const { caseId } = await params;
    const repository = await createServerRepository();
    return liveResponse(await repository.resolveReconciliationCase(caseId, command, idempotencyKey), 201);
  } catch (error) {
    return problemResponse(error);
  }
}
