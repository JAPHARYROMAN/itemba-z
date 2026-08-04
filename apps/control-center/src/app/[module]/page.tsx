import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ModulePageView } from "@/components/module-page-view";
import { getModule, isModuleKey, moduleKeys } from "@/data/erp-repository";

export function generateStaticParams() { return moduleKeys.map((module) => ({ module })); }

export async function generateMetadata({ params }: { params: Promise<{ module: string }> }): Promise<Metadata> {
  const { module } = await params;
  if (!isModuleKey(module)) return {};
  const data = await getModule(module);
  return { title: data.title.en, description: data.description.en };
}

export default async function ModulePage({ params }: { params: Promise<{ module: string }> }) {
  const { module } = await params;
  if (!isModuleKey(module)) notFound();
  return <ModulePageView module={await getModule(module)} />;
}
