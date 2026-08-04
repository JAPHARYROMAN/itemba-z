import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function GET(_request: Request, { params }: { params: Promise<{ saleId: string }> }) {
  try {
    const { saleId } = await params;
    const repository = await createServerRepository();
    return liveResponse(await repository.getSale(saleId));
  } catch (error) {
    return problemResponse(error);
  }
}
