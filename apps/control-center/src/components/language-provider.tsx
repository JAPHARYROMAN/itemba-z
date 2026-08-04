"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useSyncExternalStore } from "react";
import type { Locale, LocalizedText } from "@/domain/erp";
import { localize, translate, type TranslationKey } from "@/lib/i18n";

const STORAGE_KEY = "itemba-z:preferences:v1";
const CHANGE_EVENT = "itemba-z:preferences-change";

interface LanguageContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey) => string;
  l: (value: LocalizedText) => string;
}

const LanguageContext = createContext<LanguageContextValue | null>(null);

function readLocale(): Locale | null {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { locale?: unknown };
    return parsed.locale === "sw" || parsed.locale === "en" ? parsed.locale : null;
  } catch {
    return null;
  }
}

export function LanguageProvider({ children }: Readonly<{ children: React.ReactNode }>) {
  const subscribe = useCallback((onStoreChange: () => void) => {
    window.addEventListener("storage", onStoreChange);
    window.addEventListener(CHANGE_EVENT, onStoreChange);
    return () => {
      window.removeEventListener("storage", onStoreChange);
      window.removeEventListener(CHANGE_EVENT, onStoreChange);
    };
  }, []);
  const locale = useSyncExternalStore<Locale>(subscribe, () => readLocale() ?? "en", () => "en");

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const setLocale = useCallback((nextLocale: Locale) => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ locale: nextLocale }));
    window.dispatchEvent(new Event(CHANGE_EVENT));
  }, []);

  const value = useMemo<LanguageContextValue>(() => ({
    locale,
    setLocale,
    t: (key) => translate(locale, key),
    l: (copy) => localize(copy, locale),
  }), [locale, setLocale]);

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

export function useLanguage(): LanguageContextValue {
  const context = useContext(LanguageContext);
  if (!context) throw new Error("useLanguage must be used within LanguageProvider");
  return context;
}
