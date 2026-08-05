import type { ChangeMobileDeviceAllocationCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";

export async function POST(request: Request, { params }: { params: Promise<{ deviceId: string }> }) {
  try {
    assertSameOrigin(request);
    const idempotencyKey = requireIdempotencyKey(request);
    const command = await requireJsonBody<ChangeMobileDeviceAllocationCommand>(request);
    const { deviceId } = await params;
    const repository = await createServerRepository();
    return liveResponse(await repository.changeManagedDeviceAllocation(deviceId, command, idempotencyKey));
  } catch (error) {
    return problemResponse(error);
  }
}
