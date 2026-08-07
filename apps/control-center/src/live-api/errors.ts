import { IdentityError } from "@/live-api/auth";
import { LiveApiError } from "@/live-api/api-client";
import type { PublicProblem } from "@/live-api/types";

export function publicProblem(error: unknown): PublicProblem {
  if (error instanceof LiveApiError || error instanceof IdentityError) return error.problem;
  return {
    type: "urn:itemba-z:control-center:unexpected",
    title: "Live workspace unavailable",
    status: 500,
    code: "live_workspace_unavailable",
    detail: "The live workspace could not be loaded. No demonstration data was substituted.",
  };
}
