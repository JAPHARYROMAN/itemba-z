import type { MobileReconciliationStatus } from "@/live-api/types";
import { liveResponse, problemResponse } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  try {
    const search = new URL(request.url).searchParams;
    const status = search.get("status");
    const repository = await createServerRepository();
    return liveResponse(await repository.listReconciliationCases(
      status === "OPEN" || status === "RESOLVED" ? status as MobileReconciliationStatus : undefined,
      search.get("cursor") || undefined,
    ));
  } catch (error) {
    return problemResponse(error);
  }
}
