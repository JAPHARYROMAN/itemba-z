import type { CreateInventoryPolicyCommand, InventoryPolicyStatus, RegisterReceiptLotsCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";
type Command =
	| { action: "create_policy"; command: CreateInventoryPolicyCommand }
	| { action: "transition_policy"; id: string; status: InventoryPolicyStatus; reason: string }
	| { action: "register_lots"; receiptId: string; command: RegisterReceiptLotsCommand };

export async function POST(request: Request) {
	try {
		assertSameOrigin(request);
		const key = requireIdempotencyKey(request);
		const body = await requireJsonBody<Command>(request);
		const repository = await createServerRepository();
		switch (body.action) {
			case "create_policy": return liveResponse(await repository.createInventoryPolicy(body.command, key), 201);
			case "transition_policy": return liveResponse(await repository.transitionInventoryPolicy(body.id, body.status, body.reason, key));
			case "register_lots": return liveResponse(await repository.registerReceiptLots(body.receiptId, body.command, key), 201);
		}
	} catch (error) { return problemResponse(error); }
}
