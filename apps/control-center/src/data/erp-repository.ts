import { cache } from "react";
import type { DashboardData, ModuleData, ModuleKey, ModuleRecord, SearchResult } from "@/domain/erp";
import { dashboard, modules } from "@/data/mock-data";

export const moduleKeys = Object.keys(modules) as ModuleKey[];

export function isModuleKey(value: string): value is ModuleKey {
  return moduleKeys.includes(value as ModuleKey);
}

export const getDashboard = cache(async (): Promise<DashboardData> => dashboard);

export const getModule = cache(async (key: ModuleKey): Promise<ModuleData> => modules[key]);

export const getRecord = cache(async (key: ModuleKey, id: string): Promise<ModuleRecord | undefined> =>
  modules[key].records.find((record) => record.id === id)
);

export const searchRecords = cache(async (query: string): Promise<SearchResult[]> => {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return [];

  return moduleKeys.flatMap((key) => {
    const moduleData = modules[key];
    return moduleData.records.flatMap((record) => {
      const haystack = [record.id, record.primary.en, record.primary.sw, record.secondary, ...Object.values(record.values)]
        .join(" ")
        .toLocaleLowerCase();

      return haystack.includes(normalized)
        ? [{ module: key, moduleLabel: moduleData.title, id: record.id, title: record.primary, subtitle: record.secondary, status: record.status, href: `/${key}/${record.id}` }]
        : [];
    });
  });
});
