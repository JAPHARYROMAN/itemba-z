package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegistryRendersBoundedHTTPMetrics(t *testing.T) {
	registry := NewRegistry()
	registry.UpdateDatabasePool(3, 2, 5, 20)
	registry.ReplaceOperationalMetrics(map[string]map[string]float64{
		"itemba_outbox_oldest_unpublished_seconds": {"": 12},
		"itemba_integration_backlog":               {"TRA_FISCALIZATION": 4, "unsafe-tenant-label": 99},
		"unapproved_metric":                        {"": 1},
	})
	registry.ObserveHTTPRequest(http.MethodGet, "/v1/customers/{customerID}/account", http.StatusOK, 125*time.Millisecond)
	registry.ObserveHTTPRequest(http.MethodGet, "/v1/customers/{customerID}/account", http.StatusInternalServerError, 3*time.Second)
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, marker := range []string{"itemba_http_requests_total", `route="/v1/customers/{customerID}/account"`, `status_class="2xx"`, `status_class="5xx"`, "itemba_http_request_duration_seconds_bucket", "itemba_build_info", `itemba_database_pool_connections{state="acquired"} 3`, "itemba_database_pool_max_connections 20", "itemba_outbox_oldest_unpublished_seconds 12", `itemba_integration_backlog{capability="TRA_FISCALIZATION"} 4`} {
		if !strings.Contains(body, marker) {
			t.Fatalf("metrics omitted %q: %s", marker, body)
		}
	}
	if strings.Contains(body, "customer-123") {
		t.Fatal("metrics leaked a business identifier")
	}
	if strings.Contains(body, "unsafe-tenant-label") || strings.Contains(body, "unapproved_metric") {
		t.Fatalf("metrics accepted an unbounded operational label: %s", body)
	}
}

func TestMetricsEndpointIsReadOnlyAndExact(t *testing.T) {
	registry := NewRegistry()
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/metrics", nil),
		httptest.NewRequest(http.MethodGet, "/other", nil),
	} {
		recorder := httptest.NewRecorder()
		registry.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d", request.Method, request.URL.Path, recorder.Code)
		}
	}
}
