import type { ReverseSaleCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function POST(request: Request, { params }: { params: Promise<{ saleId: string }> }) {
  try {
    assertSameOrigin(request);
    const idempotencyKey = requireIdempotencyKey(request);
    const command = await requireJsonBody<ReverseSaleCommand>(request);
    const { saleId } = await params;
    const repository = await createServerRepository();
    return liveResponse(await repository.reverseSale(saleId, command, idempotencyKey), 201);
  } catch (error) {
    return problemResponse(error);
  }
}
