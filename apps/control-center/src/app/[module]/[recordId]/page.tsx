import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { RecordDetailView } from "@/components/record-detail-view";
import { getModule, getRecord, isModuleKey } from "@/data/erp-repository";

type PageProps = { params: Promise<{ module: string; recordId: string }> };

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { module, recordId } = await params;
  if (!isModuleKey(module)) return {};
  const record = await getRecord(module, recordId);
  return record ? { title: `${record.id} · ${record.primary.en}` } : {};
}

export default async function RecordPage({ params }: PageProps) {
  const { module, recordId } = await params;
  if (!isModuleKey(module)) notFound();
  const [moduleData, record] = await Promise.all([getModule(module), getRecord(module, recordId)]);
  if (!record) notFound();
  return <RecordDetailView module={moduleData} record={record} />;
}
