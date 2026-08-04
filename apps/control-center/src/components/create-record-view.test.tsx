import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CreateRecordView } from "@/components/create-record-view";
import { LanguageProvider } from "@/components/language-provider";
import { modules } from "@/data/mock-data";
import { text } from "@/lib/i18n";

describe("CreateRecordView", () => {
  it("renders localized select options and validates the draft", () => {
    const moduleData = {
      ...modules.settings,
      formFields: [
        {
          name: "branch",
          label: text("Branch", "Tawi"),
          placeholder: text("Select branch", "Chagua tawi"),
          type: "select" as const,
          required: true,
          options: [text("Dar es Salaam HQ", "Makao Makuu Dar es Salaam")],
        },
      ],
    };

    render(<LanguageProvider><CreateRecordView module={moduleData} /></LanguageProvider>);

    const branch = screen.getByRole("combobox", { name: /Branch/ });
    expect(branch).toHaveTextContent("Select branch");
    fireEvent.change(branch, { target: { value: "Dar es Salaam HQ" } });
    fireEvent.click(screen.getByRole("button", { name: "Validate draft" }));
    expect(screen.getByRole("status")).toHaveTextContent("ready for backend submission");
  });
});
