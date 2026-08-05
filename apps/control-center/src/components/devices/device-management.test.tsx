import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DeviceManagement } from "@/components/devices/device-management";
import { LanguageProvider } from "@/components/language-provider";
import type { DeviceManagementWorkspace } from "@/live-api/types";

vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }) }));

const workspace: DeviceManagementWorkspace = {
  context: {
    actor_id: "00000000-0000-4000-8000-000000000005", tenant_id: "00000000-0000-4000-8000-000000000001",
    company_id: "00000000-0000-4000-8000-000000000002", company_name: "Itemba Trading",
    branch_id: "00000000-0000-4000-8000-000000000003", branch_name: "Dar es Salaam",
    warehouse_id: "00000000-0000-4000-8000-000000000004", warehouse_name: "Main Warehouse",
    currency: "TZS", locale: "en-TZ", timezone: "Africa/Dar_es_Salaam",
    permissions: ["mobile.devices.read", "mobile.devices.manage"], master_data_version: 1, price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000010",
  },
  products: [{
    id: "00000000-0000-4000-8000-000000000020", code: "SKU-20", name: "Itemba Water",
    unit: "CASE", currency: "TZS", unit_price_minor: 12000, available_quantity: 40,
    price_version: 1, master_data_version: 1, tax_basis_points: 0,
  }],
  devices: [{
    device_id: "00000000-0000-4000-8000-000000000030", status: "ACTIVE",
    actor_id: "00000000-0000-4000-8000-000000000005",
    scope: { tenant_id: "00000000-0000-4000-8000-000000000001", company_id: "00000000-0000-4000-8000-000000000002", branch_id: "00000000-0000-4000-8000-000000000003", warehouse_id: "00000000-0000-4000-8000-000000000004" },
    device_name: "Route POS 1", app_version: "1.0.0", master_data_version: 1, price_version: 1,
    catalog_snapshot_token: "00000000-0000-4000-8000-000000000010",
    available_master_data_version: 1, available_price_version: 1,
    available_catalog_snapshot_token: "00000000-0000-4000-8000-000000000010",
    timezone: "Africa/Dar_es_Salaam", offline_enabled: true, transaction_value_limit_minor: 100000,
    daily_value_limit_minor: 500000, remaining_daily_value_minor: 500000,
    offline_sales_valid_until: "2026-08-04T13:00:00Z", enrolled_at: "2026-08-04T08:00:00Z",
    last_seen_at: "2026-08-04T09:00:00Z",
    stock_allocations: [{ product_id: "00000000-0000-4000-8000-000000000020", allocated_quantity: 10, remaining_quantity: 7 }],
  }],
  nextCursor: null,
};

describe("DeviceManagement", () => {
  beforeEach(() => window.localStorage.setItem("itemba-z:preferences:v1", JSON.stringify({ locale: "en" })));
  afterEach(cleanup);

  it("shows governed status and allocation controls from live evidence", () => {
    render(<LanguageProvider><DeviceManagement workspace={workspace} /></LanguageProvider>);
    expect(screen.getByRole("heading", { name: "POS device governance" })).toBeInTheDocument();
    expect(screen.getByText("Route POS 1")).toBeInTheDocument();
    expect(screen.getByText("7 / 10")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Suspend now" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Record allocation" })).toBeDisabled();
  });
});
