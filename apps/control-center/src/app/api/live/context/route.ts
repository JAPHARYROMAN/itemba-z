import { createServerRepository } from "@/live-api/server-repository";
import { liveResponse, problemResponse } from "@/live-api/bff";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const repository = await createServerRepository();
    return liveResponse(await repository.getWorkingContext());
  } catch (error) {
    return problemResponse(error);
  }
}
