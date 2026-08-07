import type {
  AdvancedFinanceTransitionCommand,
  CreateBudgetCommand,
  CreateFixedAssetCommand,
  CreatePurchasedFixedAssetCommand,
  DepreciateFixedAssetCommand,
  DisposeFixedAssetCommand,
} from "@/live-api/types";
import {
  assertSameOrigin,
  liveResponse,
  problemResponse,
  requireIdempotencyKey,
  requireJsonBody,
} from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";
export async function GET(request: Request) {
  try {
    const id = new URL(request.url).searchParams.get("budget_id");
    if (!id) throw new Error("budget_id is required");
    const repository = await createServerRepository();
    return liveResponse(await repository.budgetActual(id));
  } catch (error) {
    return problemResponse(error);
  }
}
type Command =
  | { action: "create_budget"; command: CreateBudgetCommand }
  | {
      action: "transition_budget";
      id: string;
      command: AdvancedFinanceTransitionCommand;
    }
  | { action: "create_asset"; command: CreateFixedAssetCommand }
  | { action: "create_purchased_asset"; command: CreatePurchasedFixedAssetCommand }
  | {
      action: "transition_asset";
      id: string;
      command: AdvancedFinanceTransitionCommand;
    }
  | {
      action: "depreciate_asset";
      id: string;
      command: DepreciateFixedAssetCommand;
    }
  | { action: "dispose_asset"; id: string; command: DisposeFixedAssetCommand };
export async function POST(request: Request) {
  try {
    assertSameOrigin(request);
    const key = requireIdempotencyKey(request);
    const body = await requireJsonBody<Command>(request);
    const repository = await createServerRepository();
    if (body.action === "create_budget")
      return liveResponse(
        await repository.createBudget(body.command, key),
        201,
      );
    if (body.action === "transition_budget")
      return liveResponse(
        await repository.transitionBudget(body.id, body.command, key),
      );
    if (body.action === "create_asset")
      return liveResponse(
        await repository.createFixedAsset(body.command, key),
        201,
      );
    if (body.action === "create_purchased_asset")
      return liveResponse(await repository.createFixedAssetFromPurchase(body.command, key), 201);
    if (body.action === "transition_asset")
      return liveResponse(
        await repository.transitionFixedAsset(body.id, body.command, key),
      );
    if (body.action === "depreciate_asset")
      return liveResponse(
        await repository.depreciateFixedAsset(body.id, body.command, key),
        201,
      );
    return liveResponse(
      await repository.disposeFixedAsset(body.id, body.command, key),
    );
  } catch (error) {
    return problemResponse(error);
  }
}
