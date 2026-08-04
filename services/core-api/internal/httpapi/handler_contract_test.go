package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestV1FoundationWireShape(t *testing.T) {
	var command completeSaleRequest
	payload := `{"customer_id":"customer-1","kind":"CASH","payment_method":"CASH","lines":[{"product_id":"product-1","quantity":2}]}`
	if err := json.NewDecoder(strings.NewReader(payload)).Decode(&command); err != nil {
		t.Fatal(err)
	}
	if command.Kind != sales.KindCash || command.PaymentMethod != "CASH" || command.Lines[0].Quantity != 2 {
		t.Fatalf("command=%+v", command)
	}
	encoded, err := json.Marshal(sales.Sale{ID: "sale-1", Scope: tenancy.Scope{TenantID: "tenant-1", CompanyID: "company-1", BranchID: "branch-1", WarehouseID: "warehouse-1"},
		RecordType: sales.RecordSale, Kind: sales.KindCash, Status: sales.StatusPosted, Currency: "TZS", SubtotalMinor: 10000, TaxMinor: 1800, TotalMinor: 11800, CreatedAt: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, required := range []string{`"kind":"CASH"`, `"subtotal_minor":10000`, `"tax_minor":1800`, `"total_minor":11800`} {
		if !strings.Contains(text, required) {
			t.Errorf("response missing %s: %s", required, text)
		}
	}
}

func TestHealthContractAndVersionedRoutePrefix(t *testing.T) {
	handler := (&Handler{}).Routes()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"healthy"`) {
		t.Fatalf("health response: %d %s", recorder.Code, recorder.Body.String())
	}
	if !correlationIDPattern.MatchString(recorder.Header().Get("X-Correlation-ID")) {
		t.Fatalf("generated correlation id=%q", recorder.Header().Get("X-Correlation-ID"))
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/sales", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("legacy route unexpectedly served: %d", recorder.Code)
	}
}

func TestCorrelationIDIsPropagatedAndIncludedInProblems(t *testing.T) {
	handler := (&Handler{authenticator: fixedAuthenticator{err: ErrUnauthenticated}}).Routes()
	const correlationID = "019fcb0e-dc88-4372-8ee4-379e29e8f003"
	request := httptest.NewRequest(http.MethodPost, "/v1/sales", strings.NewReader(`{}`))
	request.Header.Set("X-Correlation-ID", correlationID)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Correlation-ID") != correlationID {
		t.Fatalf("correlation header=%q", recorder.Header().Get("X-Correlation-ID"))
	}
	if !strings.Contains(recorder.Body.String(), `"correlation_id":"`+correlationID+`"`) {
		t.Fatalf("problem lacks correlation: %s", recorder.Body.String())
	}
}
