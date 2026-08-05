import type { CreateMasterRevisionCommand, CreateRFQCommand, CreateSupplierQuoteCommand, MasterRevisionStatus, RFQStatus } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic="force-dynamic";
type Command=
 |{action:"create_master";command:CreateMasterRevisionCommand}
 |{action:"transition_master";id:string;status:MasterRevisionStatus;reason:string}
 |{action:"create_rfq";command:CreateRFQCommand}
 |{action:"transition_rfq";id:string;status:RFQStatus;reason:string}
 |{action:"create_quote";command:CreateSupplierQuoteCommand}
 |{action:"submit_quote";id:string;reason:string}
 |{action:"award_rfq";id:string;quoteId:string;reason:string};
export async function POST(request:Request){try{assertSameOrigin(request);const key=requireIdempotencyKey(request);const body=await requireJsonBody<Command>(request);const repository=await createServerRepository();switch(body.action){case"create_master":return liveResponse(await repository.createMasterRevision(body.command,key),201);case"transition_master":return liveResponse(await repository.transitionMasterRevision(body.id,body.status,body.reason,key));case"create_rfq":return liveResponse(await repository.createRFQ(body.command,key),201);case"transition_rfq":return liveResponse(await repository.transitionRFQ(body.id,body.status,body.reason,key));case"create_quote":return liveResponse(await repository.createSupplierQuote(body.command,key),201);case"submit_quote":return liveResponse(await repository.submitSupplierQuote(body.id,body.reason,key));case"award_rfq":return liveResponse(await repository.awardRFQ(body.id,body.quoteId,body.reason,key),201)}}catch(error){return problemResponse(error)}}
