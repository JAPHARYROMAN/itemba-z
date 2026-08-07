import type { ExportFinancialReportCommand } from "@/live-api/types";
import { assertSameOrigin, liveResponse, problemResponse, requireIdempotencyKey, requireJsonBody } from "@/live-api/bff";
import { createServerRepository } from "@/live-api/server-repository";

export const dynamic = "force-dynamic";
export async function GET(request: Request, context: { params: Promise<{ report: string }> }) {
  try { const repository = await createServerRepository(); const { report } = await context.params; const query = new URL(request.url).searchParams; const from = query.get("from") ?? ""; const to = query.get("to") ?? ""; const asOf = query.get("as_of") ?? "";
    if (report === "trial-balance") return liveResponse(await repository.trialBalance(asOf));
    if (report === "general-ledger") return liveResponse(await repository.generalLedger(from, to, query.get("account_id") ?? ""));
    if (report === "profit-and-loss") return liveResponse(await repository.profitAndLoss(from, to));
    if (report === "balance-sheet") return liveResponse(await repository.balanceSheet(asOf));
    if (report === "cash-flow") return liveResponse(await repository.cashFlow(from, to));
    return Response.json({ code: "not_found", detail: "Financial report not found" }, { status: 404 });
  } catch (error) { return problemResponse(error); }
}
export async function POST(request: Request, context: { params: Promise<{ report: string }> }) {
  try { assertSameOrigin(request); const { report } = await context.params; if (report !== "exports") return Response.json({ code: "not_found", detail: "Financial report command not found" }, { status: 404 }); const key = requireIdempotencyKey(request); const body = await requireJsonBody<ExportFinancialReportCommand>(request); const repository = await createServerRepository(); return liveResponse(await repository.exportFinancialReport(body, key), 201); }
  catch (error) { return problemResponse(error); }
}
