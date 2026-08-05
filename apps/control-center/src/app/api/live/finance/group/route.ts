import type { CreateIntercompanyCommand, IntercompanyTransitionCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
type Command = { action: "create"; command: CreateIntercompanyCommand } | { action: "transition"; id: string; command: IntercompanyTransitionCommand };
export async function POST(request: Request) { try { assertSameOrigin(request); const key=requireIdempotencyKey(request); const body=await requireJsonBody<Command>(request); const repository=await createServerRepository(); if(body.action==="create") return liveResponse(await repository.createIntercompanyTransaction(body.command,key),201); return liveResponse(await repository.transitionIntercompanyTransaction(body.id,body.command,key)); } catch(error){ return problemResponse(error); } }
