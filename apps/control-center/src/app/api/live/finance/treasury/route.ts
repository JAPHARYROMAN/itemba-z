import type { CreateTreasuryFacilityCommand, PostTreasuryTransactionCommand, TreasuryTransitionCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";
type Command =
  | { action: "create_facility"; command: CreateTreasuryFacilityCommand }
  | { action: "transition_facility"; id: string; command: TreasuryTransitionCommand }
  | { action: "post_transaction"; id: string; command: PostTreasuryTransactionCommand };

export async function POST(request: Request) {
  try {
    assertSameOrigin(request);
    const key = requireIdempotencyKey(request);
    const body = await requireJsonBody<Command>(request);
    const repository = await createServerRepository();
    if (body.action === "create_facility") return liveResponse(await repository.createTreasuryFacility(body.command, key), 201);
    if (body.action === "transition_facility") return liveResponse(await repository.transitionTreasuryFacility(body.id, body.command, key));
    return liveResponse(await repository.postTreasuryTransaction(body.id, body.command, key), 201);
  } catch (error) { return problemResponse(error); }
}
