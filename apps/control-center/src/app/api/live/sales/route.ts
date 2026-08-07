import type { CompleteSaleCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  try {
    const cursor = new URL(request.url).searchParams.get("cursor") || undefined;
    const repository = await createServerRepository();
    return liveResponse(await repository.listSales(cursor));
  } catch (error) {
    return problemResponse(error);
  }
}

export async function POST(request: Request) {
  try {
    assertSameOrigin(request);
    const idempotencyKey = requireIdempotencyKey(request);
    const command = await requireJsonBody<CompleteSaleCommand>(request);
    const repository = await createServerRepository();
    return liveResponse(await repository.completeSale(command, idempotencyKey), 201);
  } catch (error) {
    return problemResponse(error);
  }
}
