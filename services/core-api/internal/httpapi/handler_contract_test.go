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

	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type recordingHTTPObserver struct {
	method string
	route  string
	status int
}

func (observer *recordingHTTPObserver) ObserveHTTPRequest(method, route string, status int, _ time.Duration) {
	observer.method, observer.route, observer.status = method, route, status
}

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

func TestObservationUsesRoutePatternAndNeverRawBusinessPath(t *testing.T) {
	var logs bytes.Buffer
	observer := &recordingHTTPObserver{}
	handler := &Handler{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	handler.SetHTTPObserver(observer)
	request := httptest.NewRequest(http.MethodGet, "/healthz?token=never-log", nil)
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if observer.method != http.MethodGet || observer.route != "GET /healthz" || observer.status != http.StatusOK {
		t.Fatalf("observation=%+v", observer)
	}
	if strings.Contains(logs.String(), "never-log") || strings.Contains(logs.String(), `"path"`) {
		t.Fatalf("raw request data entered logs: %s", logs.String())
	}
}

func TestWebIdempotencyKeyBoundsAreEnforced(t *testing.T) {
	principal := Principal{ActorID: "10000000-0000-4000-8000-000000000005", Scope: tenancy.Scope{TenantID: "10000000-0000-4000-8000-000000000001", CompanyID: "10000000-0000-4000-8000-000000000002", BranchID: "10000000-0000-4000-8000-000000000003", WarehouseID: "10000000-0000-4000-8000-000000000004"}}
	handler := (&Handler{authenticator: fixedAuthenticator{principal: principal}}).Routes()
	request := httptest.NewRequest(http.MethodPost, "/v1/sales", strings.NewReader(`{}`))
	request.Header.Set("Idempotency-Key", "too-short")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "16 to 128") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicSalePresentationOmitsInternalCostFacts(t *testing.T) {
	postedAt := time.Date(2026, time.August, 6, 8, 30, 0, 0, time.UTC)
	response := presentSale(sales.Sale{
		ID: "10000000-0000-4000-8000-000000000001", COGSMinor: 700,
		DocumentAt: postedAt, ReceivedAt: postedAt, AccountingAt: postedAt,
		AccountingTimeBasis: sales.AccountingTimeServerReceipt,
		Lines: []sales.Line{{
			ID: "10000000-0000-4000-8000-000000000002", ProductID: "10000000-0000-4000-8000-000000000003",
			UnitCostMinor: 700, COGSMinor: 700,
		}},
	})
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "cogs_minor") || strings.Contains(string(encoded), "unit_cost_minor") {
		t.Fatalf("public presentation leaked cost facts: %s", encoded)
	}
	for _, field := range []string{"document_at", "received_at", "accounting_at", "accounting_time_basis"} {
		if !strings.Contains(string(encoded), `"`+field+`"`) {
			t.Fatalf("public presentation omitted required posting time field %q: %s", field, encoded)
		}
	}
}
