import { notFound } from "next/navigation";
import { CreateRecordView } from "@/components/create-record-view";
import { getModule, isModuleKey } from "@/data/erp-repository";

export default async function NewRecordPage({ params }: { params: Promise<{ module: string }> }) {
  const { module } = await params;
  if (!isModuleKey(module)) notFound();
  return <CreateRecordView module={await getModule(module)} />;
}
