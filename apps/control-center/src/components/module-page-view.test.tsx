import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LanguageProvider } from "@/components/language-provider";
import { ModulePageView } from "@/components/module-page-view";
import { modules } from "@/data/mock-data";

describe("ModulePageView", () => {
  it("uses the persisted Swahili locale and filters records", () => {
    window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "sw" }));
    render(<LanguageProvider><ModulePageView module={modules.sales} /></LanguageProvider>);
    fireEvent.change(screen.getByPlaceholderText("Tafuta katika moduli…"), { target: { value: "Mlimani" } });
    expect(screen.getByText("Mlimani Mini Mart")).toBeInTheDocument();
    expect(screen.queryByText("Kijiji Supermarket Ltd")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Mauzo", level: 1 })).toBeInTheDocument();
  });
});
