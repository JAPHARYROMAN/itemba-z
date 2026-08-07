import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function GET(_request: Request, { params }: { params: Promise<{ caseId: string }> }) {
  try {
    const { caseId } = await params;
    const repository = await createServerRepository();
    return liveResponse(await repository.getReconciliationCase(caseId));
  } catch (error) {
    return problemResponse(error);
  }
}
