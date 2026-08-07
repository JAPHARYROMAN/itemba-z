package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestDashboardRejectsScopeExpansionBeforeReading(t *testing.T) {
	scope := tenancy.Scope{
		TenantID: "30000000-0000-4000-8000-000000000001", CompanyID: "30000000-0000-4000-8000-000000000002",
		BranchID: "30000000-0000-4000-8000-000000000003", WarehouseID: "30000000-0000-4000-8000-000000000004",
	}
	handler := &Handler{authenticator: fixedAuthenticator{principal: Principal{ActorID: "30000000-0000-4000-8000-000000000005", Scope: scope}}}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/dashboard?legal_company_id=30000000-0000-4000-8000-000000000099", nil)
	handler.dashboard(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"code":"scope_mismatch"`) {
		t.Fatalf("scope expansion response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestDashboardRequiresCompleteOptionalRange(t *testing.T) {
	scope := tenancy.Scope{
		TenantID: "30000000-0000-4000-8000-000000000001", CompanyID: "30000000-0000-4000-8000-000000000002",
		BranchID: "30000000-0000-4000-8000-000000000003", WarehouseID: "30000000-0000-4000-8000-000000000004",
	}
	handler := &Handler{authenticator: fixedAuthenticator{principal: Principal{ActorID: "30000000-0000-4000-8000-000000000005", Scope: scope}}}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/dashboard?from=2026-08-01", nil)
	handler.dashboard(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_report_query"`) {
		t.Fatalf("partial range response: %d %s", recorder.Code, recorder.Body.String())
	}
}
