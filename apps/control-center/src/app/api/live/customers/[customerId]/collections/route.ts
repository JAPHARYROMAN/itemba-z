import type { ReceiveCustomerCollectionCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export async function POST(request: Request, { params }: { params: Promise<{ customerId: string }> }) { try { assertSameOrigin(request); const key=requireIdempotencyKey(request); const body=await requireJsonBody<ReceiveCustomerCollectionCommand>(request); const {customerId}=await params; const repository=await createServerRepository(); return liveResponse(await repository.receiveCustomerCollection(customerId,body,key),201); } catch(error){ return problemResponse(error); } }
