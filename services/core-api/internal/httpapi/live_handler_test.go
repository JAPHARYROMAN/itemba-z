package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestLiveBootstrapEnrollmentAndSyncRoutes(t *testing.T) {
	const (
		tenantID         = "30000000-0000-4000-8000-000000000001"
		companyID        = "30000000-0000-4000-8000-000000000002"
		branchID         = "30000000-0000-4000-8000-000000000003"
		warehouseID      = "30000000-0000-4000-8000-000000000004"
		actorID          = "30000000-0000-4000-8000-000000000005"
		customerID       = "30000000-0000-4000-8000-000000000006"
		productID        = "30000000-0000-4000-8000-000000000007"
		deviceID         = "30000000-0000-4000-8000-000000000008"
		clientID         = "30000000-0000-4000-8000-000000000009"
		reconciliationID = "30000000-0000-4000-8000-00000000000c"
		otherBranchID    = "30000000-0000-4000-8000-00000000000a"
		otherWarehouseID = "30000000-0000-4000-8000-00000000000b"
	)
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	scope := tenancy.Scope{TenantID: tenantID, CompanyID: companyID, BranchID: branchID, WarehouseID: warehouseID}
	store := memory.New()
	for _, permission := range []string{"sales.complete", "sales.read", "customers.read", "products.read", "mobile.devices.enroll", "mobile.sales.sync", "mobile.reconciliation.read", "mobile.reconciliation.resolve"} {
		store.SeedPermission(scope, actorID, permission)
	}
	store.SeedContext(scope, readmodel.WorkingContext{CompanyName: "Company", BranchName: "Branch", WarehouseName: "Warehouse", Currency: "TZS", Locale: "en-TZ", TimeZone: "Africa/Dar_es_Salaam", MasterDataVersion: 1, PriceVersion: 1})
	store.SeedCustomer(customers.Account{ID: customerID, Code: "GENERAL", TenantID: tenantID, CompanyID: companyID, Name: "General Customer", Active: true, General: true})
	store.SeedProduct(catalog.Product{ID: productID, TenantID: tenantID, CompanyID: companyID, SKU: "SKU", Name: "Product", BaseUnitCode: "EA", Active: true, Currency: "TZS", ListPriceMinor: 10_000, StandardCostMinor: 6_000, TaxCode: "VAT", RevenueAccountID: "revenue", COGSAccountID: "cogs", InventoryAccountID: "inventory", PriceVersion: 1, MasterDataVersion: 1})
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 1800, EffectiveFrom: at.AddDate(-1, 0, 0)})
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: tenantID, CompanyID: companyID, StartsAt: at.AddDate(-1, 0, 0), EndsAt: at.AddDate(1, 0, 0), Open: true})
	store.SeedPostingConfig(tenantID, companyID, finance.SalesPostingConfig{ReceivableAccountID: "receivable", TaxPayableAccountID: "tax", CashAccounts: map[string]string{"CASH": "cash"}})
	store.SeedStock(inventory.Movement{ID: "opening", TenantID: tenantID, CompanyID: companyID, BranchID: branchID, WarehouseID: warehouseID, ProductID: productID, SourceType: "OPENING", SourceID: "opening", Quantity: 10, OccurredAt: at.Add(-time.Hour)})
	salesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	readService, err := readmodel.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	mobileService, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler, err := NewLive(salesService, readService, mobileService, logger, fixedAuthenticator{principal: Principal{ActorID: actorID, Scope: scope}})
	if err != nil {
		t.Fatal(err)
	}
	routes := handler.Routes()

	for _, path := range []string{"/v1/context", "/v1/customers?page_size=10", "/v1/products?page_size=10", "/v1/sales?page_size=10"} {
		recorder := httptest.NewRecorder()
		routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
		if path == "/v1/context" {
			var workingContext readmodel.WorkingContext
			if err := json.Unmarshal(recorder.Body.Bytes(), &workingContext); err != nil || workingContext.ActorID != actorID {
				t.Fatalf("working context actor: %+v err=%v", workingContext, err)
			}
		}
		if path == "/v1/products?page_size=10" && !strings.Contains(recorder.Body.String(), `"tax_basis_points":1800`) {
			t.Fatalf("product bootstrap omitted effective tax data: %s", recorder.Body.String())
		}
	}

	unsupported := `{"customer_id":"` + customerID + `","kind":"CASH","payment_method":"CHEQUE","lines":[{"product_id":"` + productID + `","quantity":1}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/sales", strings.NewReader(unsupported))
	request.Header.Set("Idempotency-Key", "unsupported-payment-http")
	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), `"code":"business_rule_violation"`) {
		t.Fatalf("unsupported payment: %d %s", recorder.Code, recorder.Body.String())
	}

	unsafeInteger := `{"customer_id":"` + customerID + `","kind":"CASH","payment_method":"CASH","lines":[{"product_id":"` + productID + `","quantity":9007199254740992}]}`
	request = httptest.NewRequest(http.MethodPost, "/v1/sales", strings.NewReader(unsafeInteger))
	request.Header.Set("Idempotency-Key", "unsafe-wire-integer-http")
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), `"code":"wire_integer_out_of_range"`) {
		t.Fatalf("unsafe wire integer: %d %s", recorder.Code, recorder.Body.String())
	}

	enrollmentBody := `{"device_id":"` + deviceID + `","device_name":"POS 1","app_version":"1.0.0"}`
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/mobile/devices/enroll", strings.NewReader(enrollmentBody)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"timezone":"Africa/Dar_es_Salaam"`) || !strings.Contains(recorder.Body.String(), `"stock_allocations":[]`) || !strings.Contains(recorder.Body.String(), `"master_data_version":0`) || !strings.Contains(recorder.Body.String(), `"available_master_data_version":1`) {
		t.Fatalf("enrollment: %d %s", recorder.Code, recorder.Body.String())
	}
	enrollmentBody = `{"device_id":"` + deviceID + `","device_name":"POS 1","app_version":"1.0.0","installed_master_data_version":1,"installed_price_version":1,"installed_catalog_snapshot_token":"00000000-0000-4000-8000-000000000001"}`
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/mobile/devices/enroll", strings.NewReader(enrollmentBody)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"master_data_version":1`) {
		t.Fatalf("cache acknowledgement: %d %s", recorder.Code, recorder.Body.String())
	}

	syncBody := map[string]any{
		"customer_id": customerID, "kind": "CASH", "payment_method": "CASH",
		"lines":     []map[string]any{{"product_id": productID, "quantity": 1}},
		"device_id": deviceID, "client_transaction_id": clientID, "client_timestamp": at.Format(time.RFC3339),
		"app_version": "1.0.0", "master_data_version": 1, "price_version": 1,
		"catalog_snapshot_token": "00000000-0000-4000-8000-000000000001", "sync_attempt": 1, "offline": false,
	}
	encoded, _ := json.Marshal(syncBody)
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/mobile/sync/sales", bytes.NewReader(encoded)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"fiscal_status":"NOT_CONFIGURED"`) || !strings.Contains(recorder.Body.String(), `"idempotent_replay":false`) {
		t.Fatalf("sync: %d %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "cogs_minor") || strings.Contains(recorder.Body.String(), "unit_cost_minor") {
		t.Fatalf("public mobile response leaked internal cost fields: %s", recorder.Body.String())
	}
	var synced mobile.SyncResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &synced); err != nil || synced.Sale.ID == "" || synced.Sale.ClientTimestamp == nil || synced.Sale.AppVersion != "1.0.0" {
		t.Fatalf("sync response provenance: %+v err=%v", synced, err)
	}
	syncBody["sync_attempt"] = 2
	encoded, _ = json.Marshal(syncBody)
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/mobile/sync/sales", bytes.NewReader(encoded)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"idempotent_replay":true`) {
		t.Fatalf("replay: %d %s", recorder.Code, recorder.Body.String())
	}
	caseValue := mobile.ReconciliationCase{
		ID: reconciliationID, Scope: scope, Status: mobile.ReconciliationOpen,
		DeviceID: deviceID, ClientTransactionID: "30000000-0000-4000-8000-00000000000d",
		ClientTimestamp: at, AppVersion: "1.0.0", MasterDataVersion: 1, PriceVersion: 1,
		CatalogSnapshotToken: "00000000-0000-4000-8000-000000000001",
		FailureCode:          "offline_reconciliation_required", Command: json.RawMessage(`{"offline":true}`),
		CommandHash: strings.Repeat("a", 64), CreatedBy: actorID,
		CorrelationID: "30000000-0000-4000-8000-00000000000e", CreatedAt: at,
	}
	if _, err := store.RecordReconciliationCase(request.Context(), caseValue,
		audit.Event{ID: "30000000-0000-4000-8000-00000000000f"},
		outbox.Event{ID: "30000000-0000-4000-8000-000000000010"}); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/mobile/reconciliation-cases?status=OPEN&page_size=10", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), reconciliationID) || !strings.Contains(recorder.Body.String(), `"status":"OPEN"`) {
		t.Fatalf("reconciliation list: %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/v1/mobile/reconciliation-cases/"+reconciliationID+"/resolutions", strings.NewReader(`{"action":"CASH_REFUNDED","reason":"Cash returned to customer"}`))
	request.Header.Set("Idempotency-Key", "http-reconciliation-refund-0001")
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"status":"RESOLVED"`) || !strings.Contains(recorder.Body.String(), `"action":"CASH_REFUNDED"`) {
		t.Fatalf("reconciliation resolution: %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/v1/mobile/reconciliation-cases/"+reconciliationID+"/resolutions", strings.NewReader(`{"action":"CASH_REFUNDED","reason":"Cash returned to customer"}`))
	request.Header.Set("Idempotency-Key", "http-reconciliation-refund-0001")
	recorder = httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("resolution replay: %d %s", recorder.Code, recorder.Body.String())
	}
	otherScope := tenancy.Scope{TenantID: tenantID, CompanyID: companyID, BranchID: otherBranchID, WarehouseID: otherWarehouseID}
	store.SeedPermission(otherScope, actorID, "sales.read")
	store.SeedPermission(otherScope, actorID, "sales.reverse")
	store.SeedPermission(otherScope, actorID, "mobile.reconciliation.read")
	otherHandler, err := NewLive(salesService, readService, mobileService, logger, fixedAuthenticator{principal: Principal{ActorID: actorID, Scope: otherScope}})
	if err != nil {
		t.Fatal(err)
	}
	otherRoutes := otherHandler.Routes()
	recorder = httptest.NewRecorder()
	otherRoutes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/sales/"+synced.Sale.ID, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-branch HTTP read: %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	otherRoutes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/mobile/reconciliation-cases/"+reconciliationID, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-branch reconciliation read: %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/v1/sales/"+synced.Sale.ID+"/reversals", strings.NewReader(`{"reason":"Cross-branch attempt"}`))
	request.Header.Set("Idempotency-Key", "cross-branch-http-reversal")
	recorder = httptest.NewRecorder()
	otherRoutes.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-branch HTTP reversal: %d %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(logs.String(), `"route":"POST /v1/mobile/sync/sales"`) || !strings.Contains(logs.String(), `"correlation_id":`) || !strings.Contains(logs.String(), `"latency_ms":`) {
		t.Fatalf("structured completion log missing fields: %s", logs.String())
	}
}
