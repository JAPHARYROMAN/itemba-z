import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";
export const dynamic = "force-dynamic";
export async function GET() { try { const repository = await createServerRepository(); return liveResponse(await repository.listBankAccounts()); } catch (error) { return problemResponse(error); } }
