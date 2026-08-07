import type { Metadata } from "next";
import { LiveUnavailable } from "@/components/live-sales/live-state";
import { PurchaseDocumentDetail } from "@/components/purchases/purchase-document-detail";
import { loadPurchaseDocument } from "@/live-api/snapshots";
export const metadata: Metadata = { title: "Purchase Document" }; export const dynamic = "force-dynamic";
export default async function Page({ params }: { params: Promise<{ documentId: string }> }) { const { documentId } = await params; const snapshot = await loadPurchaseDocument(documentId); return snapshot.state === "unavailable" ? <LiveUnavailable problem={snapshot.problem} /> : <PurchaseDocumentDetail workspace={snapshot.data} />; }
