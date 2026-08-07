import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function GET(_request: Request, { params }: { params: Promise<{ entityType: string; entityId: string }> }) {
  try {
    const { entityType, entityId } = await params;
    const repository = await createServerRepository();
    return liveResponse(await repository.getAuditTrail(entityType, entityId));
  } catch (error) { return problemResponse(error); }
}
