import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "@/components/app-shell";
import { LanguageProvider } from "@/components/language-provider";
import type { WorkingContext } from "@/live-api/types";

const navigation = vi.hoisted(() => ({ pathname: "/sales" }));
vi.mock("next/navigation", () => ({ usePathname: () => navigation.pathname }));
vi.mock("@/components/session-control", () => ({ SessionControl: () => <button type="button">Profile</button> }));

let desktopMediaChange: ((event: MediaQueryListEvent) => void) | null = null;

const context: WorkingContext = {
  actor_id: "00000000-0000-4000-8000-000000000001",
  tenant_id: "00000000-0000-4000-8000-000000000002",
  company_id: "00000000-0000-4000-8000-000000000003",
  company_name: "Itemba Trading",
  branch_id: "00000000-0000-4000-8000-000000000004",
  branch_name: "Dar es Salaam",
  warehouse_id: "00000000-0000-4000-8000-000000000005",
  warehouse_name: "Main Warehouse",
  currency: "TZS",
  locale: "en-TZ",
  timezone: "Africa/Dar_es_Salaam",
  permissions: ["sales.read", "customers.read", "products.read"],
  master_data_version: 1,
  price_version: 1,
  catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
};

describe("AppShell", () => {
  beforeEach(() => {
    navigation.pathname = "/sales";
    desktopMediaChange = null;
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      media: "(min-width: 1025px)",
      onchange: null,
      addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => { desktopMediaChange = listener; },
      removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => { if (desktopMediaChange === listener) desktopMediaChange = null; },
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })));
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" }));
    vi.stubGlobal("fetch", vi.fn(async () => Response.json(context)));
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders permission-shaped navigation and a single main landmark", async () => {
    render(<LanguageProvider><AppShell><h1>Sales content</h1></AppShell></LanguageProvider>);
    expect(await screen.findByRole("link", { name: "Sales" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Inventory" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Finance" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("main")).toHaveLength(1);
    expect(screen.getAllByText("Itemba Trading")).toHaveLength(2);
  });

  it("exposes mobile drawer state, Escape dismissal, and focus restoration", async () => {
    render(<LanguageProvider><AppShell><h1>Sales content</h1></AppShell></LanguageProvider>);
    await screen.findByRole("link", { name: "Sales" });
    const menu = screen.getByRole("button", { name: "Open navigation" });
    fireEvent.click(menu);
    expect(menu).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("button", { name: "Close navigation" })[0]).toHaveFocus();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(menu).toHaveAttribute("aria-expanded", "false");
  });

  it("does not trap keyboard focus in the permanent desktop sidebar", async () => {
    render(<LanguageProvider><AppShell><h1>Sales content</h1></AppShell></LanguageProvider>);
    const primaryNavigation = await screen.findByRole("navigation", { name: "Primary navigation" });
    expect(fireEvent.keyDown(primaryNavigation, { key: "Tab" })).toBe(true);
  });

  it("closes an open mobile drawer when the viewport crosses into desktop", async () => {
    render(<LanguageProvider><AppShell><h1>Sales content</h1></AppShell></LanguageProvider>);
    await screen.findByRole("link", { name: "Sales" });
    const menu = screen.getByRole("button", { name: "Open navigation" });
    fireEvent.click(menu);
    expect(menu).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("main").parentElement).toHaveAttribute("inert");

    act(() => desktopMediaChange?.({ matches: true } as MediaQueryListEvent));

    expect(menu).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("main").parentElement).not.toHaveAttribute("inert");
  });

  it("moves focus to the main content after client-side route changes", async () => {
    const view = render(<LanguageProvider><AppShell><h1>Sales content</h1></AppShell></LanguageProvider>);
    await screen.findByRole("link", { name: "Sales" });
    navigation.pathname = "/customers";
    view.rerender(<LanguageProvider><AppShell><h1>Customer content</h1></AppShell></LanguageProvider>);
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("bypasses shell resources on the login route", () => {
    navigation.pathname = "/login";
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    render(<LanguageProvider><AppShell><h1>Sign in</h1></AppShell></LanguageProvider>);
    expect(screen.queryByRole("navigation", { name: "Primary navigation" })).not.toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
