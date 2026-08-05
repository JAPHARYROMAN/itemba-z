import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  try {
    const cursor = new URL(request.url).searchParams.get("cursor") || undefined;
    const repository = await createServerRepository();
    return liveResponse(await repository.listManagedDevices(cursor));
  } catch (error) {
    return problemResponse(error);
  }
}
