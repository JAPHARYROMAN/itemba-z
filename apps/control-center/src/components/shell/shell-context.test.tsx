import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ShellContextProvider, useShellContext } from "@/components/shell/shell-context";
import type { WorkingContext } from "@/live-api/types";

const workingContext: WorkingContext = {
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
  permissions: ["sales.read"],
  master_data_version: 1,
  price_version: 1,
  catalog_snapshot_token: "00000000-0000-4000-8000-000000000006",
};

function Probe() {
  const resources = useShellContext();
  return <p>{resources.context.state}:{resources.dashboard.state}</p>;
}

describe("ShellContextProvider", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads the operating context without requesting an unauthorized dashboard", async () => {
    const fetchMock = vi.fn(async () => Response.json(workingContext));
    vi.stubGlobal("fetch", fetchMock);
    render(<ShellContextProvider><Probe /></ShellContextProvider>);
    await screen.findByText("ready:not-permitted");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith("/api/live/context", expect.objectContaining({ cache: "no-store" }));
  });

  it("keeps valid scope available when the approvals resource fails", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(Response.json({ ...workingContext, permissions: ["sales.read", "dashboard.read"] }))
      .mockResolvedValueOnce(Response.json({ code: "dashboard_unavailable", status: 503, title: "Unavailable", detail: "Unavailable", type: "about:blank" }, { status: 503 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ShellContextProvider><Probe /></ShellContextProvider>);
    await screen.findByText("ready:unavailable");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("fails closed when the working context cannot be loaded", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Response.json({ code: "forbidden", status: 403, title: "Forbidden", detail: "Forbidden", type: "about:blank" }, { status: 403 })));
    render(<ShellContextProvider><Probe /></ShellContextProvider>);
    await waitFor(() => expect(screen.getByText("unavailable:not-permitted")).toBeInTheDocument());
  });

  it("revalidates permission-shaped shell data when the app regains focus", async () => {
    const fetchMock = vi.fn(async () => Response.json(workingContext));
    vi.stubGlobal("fetch", fetchMock);
    render(<ShellContextProvider><Probe /></ShellContextProvider>);
    await screen.findByText("ready:not-permitted");
    fireEvent.focus(window);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });
});
