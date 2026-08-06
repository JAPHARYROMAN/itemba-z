import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { SalesModuleNav } from "@/components/sales/sales-module-nav";

const navigation = vi.hoisted(() => ({ pathname: "/sales" }));
vi.mock("next/navigation", () => ({ usePathname: () => navigation.pathname }));

const permissions = ["sales.read", "sales.complete", "sales.orders.manage", "sales.reverse", "customers.read", "customers.accounts.read", "customers.collections.post", "products.read", "operations.read"];

describe("SalesModuleNav", () => {
  beforeEach(() => {
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" }));
  });

  afterEach(() => cleanup());

  it.each([
    ["/sales", "Overview"],
    ["/sales/transactions", "Transactions"],
    ["/sales/documents", "Quotes & orders"],
    ["/sales/payments", "Payments"],
    ["/sales/returns", "Returns"],
  ])("marks %s as the active Sales task", (pathname, label) => {
    navigation.pathname = pathname;
    render(<LanguageProvider><SalesModuleNav permissions={permissions} /></LanguageProvider>);
    expect(screen.getByRole("link", { name: label })).toHaveAttribute("aria-current", "page");
  });

  it("requires every read dependency before exposing a task", () => {
    render(<LanguageProvider><SalesModuleNav permissions={["customers.collections.post", "customers.read"]} /></LanguageProvider>);
    expect(screen.queryByRole("link", { name: "Payments" })).not.toBeInTheDocument();
  });
});
