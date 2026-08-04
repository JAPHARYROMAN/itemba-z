import type { Metadata, Viewport } from "next";
import { GeistSans } from "geist/font/sans";
import { GeistMono } from "geist/font/mono";
import "./globals.css";
import { AppShell } from "@/components/app-shell";
import { LanguageProvider } from "@/components/language-provider";

export const metadata: Metadata = {
  title: { default: "Control Center", template: "%s · ITEMBA-Z" },
  description: "Bilingual enterprise operations for Itemba Group.",
  applicationName: "ITEMBA-Z Control Center",
};

export const viewport: Viewport = { width: "device-width", initialScale: 1, themeColor: "#0b2b26" };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en" data-scroll-behavior="smooth" className={`${GeistSans.variable} ${GeistMono.variable}`}><body><LanguageProvider><AppShell>{children}</AppShell></LanguageProvider></body></html>;
}
