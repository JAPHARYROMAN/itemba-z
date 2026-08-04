// Package httpapi exposes the versioned REST boundary for the core application.
// Authentication is delegated to an explicit adapter that returns a verified
// actor and organizational scope. Production never derives identity from X-* headers.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
)

const maxBodyBytes = 1 << 20

var correlationIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Handler struct {
	sales         *sales.Service
	read          *readmodel.Service
	mobile        *mobile.Service
	logger        *slog.Logger
	authenticator Authenticator
}

func New(salesService *sales.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	if salesService == nil {
		return nil, errors.New("sales service is required")
	}
	if authenticator == nil {
		return nil, errors.New("authenticator is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{sales: salesService, logger: logger, authenticator: authenticator}, nil
}

func NewLive(salesService *sales.Service, readService *readmodel.Service, mobileService *mobile.Service, logger *slog.Logger, authenticator Authenticator) (*Handler, error) {
	handler, err := New(salesService, logger, authenticator)
	if err != nil {
		return nil, err
	}
	if readService == nil || mobileService == nil {
		return nil, errors.New("read and mobile services are required")
	}
	handler.read, handler.mobile = readService, mobileService
	return handler, nil
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.health)
	mux.HandleFunc("POST /v1/sales", h.completeSale)
	if h.read != nil {
		mux.HandleFunc("GET /v1/context", h.workingContext)
		mux.HandleFunc("GET /v1/customers", h.listCustomers)
		mux.HandleFunc("GET /v1/products", h.listProducts)
		mux.HandleFunc("GET /v1/sales", h.listSales)
	}
	if h.mobile != nil {
		mux.HandleFunc("POST /v1/mobile/devices/enroll", h.enrollDevice)
		mux.HandleFunc("POST /v1/mobile/sync/sales", h.syncMobileSale)
	}
	mux.HandleFunc("GET /v1/sales/{saleID}", h.getSale)
	mux.HandleFunc("POST /v1/sales/{saleID}/reversals", h.reverseSale)
	return securityHeaders(correlationIDs(identity.UUIDGenerator{}, h.observe(mux)))
}

func (h *Handler) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "healthy", "version": "dev"})
}

func (h *Handler) workingContext(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	result, err := h.read.Context(request.Context(), principal.Scope, principal.ActorID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) listCustomers(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	eligible, ok := parseOptionalBool(writer, request, "credit_eligible")
	if !ok {
		return
	}
	result, err := h.read.Customers(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("query"), request.URL.Query().Get("cursor"),
		request.URL.Query().Get("snapshot_token"), eligible, pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) listProducts(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.read.Products(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("query"), request.URL.Query().Get("cursor"),
		request.URL.Query().Get("snapshot_token"), pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) listSales(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	pageSize, ok := parsePageSize(writer, request)
	if !ok {
		return
	}
	result, err := h.read.Sales(request.Context(), principal.Scope, principal.ActorID,
		request.URL.Query().Get("cursor"), pageSize)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, presentSalePage(result))
}

type enrollDeviceRequest struct {
	DeviceID                      string  `json:"device_id"`
	DeviceName                    string  `json:"device_name"`
	AppVersion                    string  `json:"app_version"`
	InstalledMasterDataVersion    *int64  `json:"installed_master_data_version,omitempty"`
	InstalledPriceVersion         *int64  `json:"installed_price_version,omitempty"`
	InstalledCatalogSnapshotToken *string `json:"installed_catalog_snapshot_token,omitempty"`
}

func (h *Handler) enrollDevice(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	var body enrollDeviceRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	deviceID, err := identity.CanonicalUUID(body.DeviceID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "device_id must be a UUID")
		return
	}
	result, err := h.mobile.Enroll(request.Context(), mobile.EnrollCommand{
		Scope: principal.Scope, ActorID: principal.ActorID, DeviceID: deviceID,
		DeviceName: body.DeviceName, AppVersion: body.AppVersion,
		InstalledMasterDataVersion: body.InstalledMasterDataVersion, InstalledPriceVersion: body.InstalledPriceVersion,
		InstalledCatalogSnapshotToken: body.InstalledCatalogSnapshotToken,
		CorrelationID:                 correlationID(writer),
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (h *Handler) syncMobileSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	var body mobile.SyncCommand
	if !decodeJSON(writer, request, &body) {
		return
	}
	if !canonicalizeMobileCommand(&body) {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "device_id and client_transaction_id must be UUIDs")
		return
	}
	result, err := h.mobile.SyncSale(request.Context(), principal.Scope, principal.ActorID, correlationID(writer), body)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, presentMobileSync(result))
}

type completeSaleRequest struct {
	CustomerID    string              `json:"customer_id"`
	Kind          sales.Kind          `json:"kind"`
	PaymentMethod string              `json:"payment_method,omitempty"`
	Lines         []sales.CommandLine `json:"lines"`
}

func (h *Handler) completeSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return
	}
	var body completeSaleRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	if !canonicalizeSaleClaims(&body.CustomerID, body.Lines) {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "customer_id and product_id values must be UUIDs")
		return
	}
	result, err := h.sales.Complete(request.Context(), sales.CompleteCommand{
		Scope: principal.Scope, CustomerID: body.CustomerID, Kind: body.Kind,
		PaymentMethod: body.PaymentMethod, Lines: body.Lines,
		ActorID: principal.ActorID, CorrelationID: correlationID(writer), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/v1/sales/"+result.ID)
	writeJSON(writer, http.StatusCreated, presentSale(result))
}

func (h *Handler) getSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	saleID, canonicalErr := identity.CanonicalUUID(request.PathValue("saleID"))
	if canonicalErr != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "sale_id must be a UUID")
		return
	}
	result, err := h.sales.Get(request.Context(), principal.Scope, principal.ActorID, saleID)
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, presentSale(result))
}

type reverseSaleRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) reverseSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header must contain 16 to 128 characters")
		return
	}
	var body reverseSaleRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	saleID, canonicalErr := identity.CanonicalUUID(request.PathValue("saleID"))
	if canonicalErr != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "sale_id must be a UUID")
		return
	}
	result, err := h.sales.Reverse(request.Context(), sales.ReverseCommand{
		Scope: principal.Scope, SaleID: saleID, Reason: body.Reason,
		ActorID: principal.ActorID, CorrelationID: correlationID(writer), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, presentSale(result))
}

func (h *Handler) requestContext(writer http.ResponseWriter, request *http.Request) (Principal, bool) {
	principal, err := h.authenticator.Authenticate(request.Context(), request)
	if err == nil {
		principal, err = principal.canonicalized()
	}
	if err != nil {
		writeProblem(writer, http.StatusUnauthorized, "invalid_security_context", "verified actor and organizational scope are required")
		return Principal{}, false
	}
	return principal, true
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, destination any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_json", "request body must be valid JSON with known fields")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeProblem(writer, http.StatusBadRequest, "invalid_json", "request body must contain one JSON value")
		return false
	}
	return true
}

func canonicalizeSaleClaims(customerID *string, lines []sales.CommandLine) bool {
	canonical, err := identity.CanonicalUUID(*customerID)
	if err != nil {
		return false
	}
	*customerID = canonical
	for index := range lines {
		canonical, err := identity.CanonicalUUID(lines[index].ProductID)
		if err != nil {
			return false
		}
		lines[index].ProductID = canonical
	}
	return true
}

func canonicalizeMobileCommand(command *mobile.SyncCommand) bool {
	deviceID, err := identity.CanonicalUUID(command.DeviceID)
	if err != nil {
		return false
	}
	clientID, err := identity.CanonicalUUID(command.ClientTransactionID)
	snapshotToken, snapshotErr := identity.CanonicalUUID(command.CatalogSnapshotToken)
	if err != nil || snapshotErr != nil || !canonicalizeSaleClaims(&command.CustomerID, command.Lines) {
		return false
	}
	command.DeviceID, command.ClientTransactionID, command.CatalogSnapshotToken = deviceID, clientID, snapshotToken
	return true
}

func parsePageSize(writer http.ResponseWriter, request *http.Request) (int, bool) {
	value := strings.TrimSpace(request.URL.Query().Get("page_size"))
	if value == "" {
		return 50, true
	}
	pageSize, err := strconv.Atoi(value)
	if err != nil || pageSize < 1 || pageSize > 200 {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", "page_size must be between 1 and 200")
		return 0, false
	}
	return pageSize, true
}

func parseOptionalBool(writer http.ResponseWriter, request *http.Request, name string) (*bool, bool) {
	value := strings.TrimSpace(request.URL.Query().Get(name))
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_request", name+" must be true or false")
		return nil, false
	}
	return &parsed, true
}

func (h *Handler) writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, sales.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, sales.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, devices.ErrNotEnrolled), errors.Is(err, devices.ErrScopeMismatch):
		status, code = http.StatusForbidden, "device_binding_forbidden"
	case errors.Is(err, sales.ErrIdempotencyConflict):
		status, code = http.StatusConflict, "idempotency_conflict"
	case errors.Is(err, sales.ErrInsufficientStock):
		status, code = http.StatusConflict, "insufficient_stock"
	case errors.Is(err, sales.ErrFiscalPeriodClosed):
		status, code = http.StatusConflict, "fiscal_period_closed"
	case errors.Is(err, sales.ErrAlreadyReversed):
		status, code = http.StatusConflict, "already_reversed"
	case errors.Is(err, sales.ErrUnsafeWireInteger):
		status, code = http.StatusUnprocessableEntity, "wire_integer_out_of_range"
	case errors.Is(err, sales.ErrOfflineReconciliation):
		status, code = http.StatusConflict, "offline_reconciliation_required"
	case errors.Is(err, sales.ErrGeneralCustomerCredit), errors.Is(err, sales.ErrCustomerCreditDisabled), errors.Is(err, sales.ErrCreditLimitExceeded), errors.Is(err, sales.ErrCustomerInactive), errors.Is(err, sales.ErrProductInactive),
		errors.Is(err, sales.ErrOfflineCredit), errors.Is(err, sales.ErrOfflinePaymentMethod), errors.Is(err, sales.ErrOfflineTaxUnsupported), errors.Is(err, sales.ErrUnsupportedPayment), errors.Is(err, devices.ErrNotActive), errors.Is(err, devices.ErrOfflineDisabled), errors.Is(err, devices.ErrMobileCreditUnsupported), errors.Is(err, devices.ErrAllocationExceeded), errors.Is(err, devices.ErrOfflineLimit), errors.Is(err, devices.ErrOfflineLeaseExpired), errors.Is(err, devices.ErrStaleMasterData), errors.Is(err, devices.ErrInvalidTimeZone):
		status, code = http.StatusUnprocessableEntity, "business_rule_violation"
	case errors.Is(err, sales.ErrInvalidCommand), errors.Is(err, sales.ErrInvalidLine), errors.Is(err, sales.ErrDuplicateProductLine), errors.Is(err, sales.ErrInvalidSaleKind), errors.Is(err, sales.ErrPaymentMethodRequired):
		status, code = http.StatusBadRequest, "invalid_request"
	}
	if status == http.StatusInternalServerError {
		h.logger.ErrorContext(request.Context(), "request failed", "method", request.Method, "path", request.URL.Path, "error", err)
		writeProblem(writer, status, code, "the request could not be completed")
		return
	}
	writeProblem(writer, status, code, err.Error())
}

func writeProblem(writer http.ResponseWriter, status int, code, detail string) {
	writeJSONStatus(writer, status, map[string]any{
		"type": "about:blank", "title": http.StatusText(status), "status": status,
		"code": code, "detail": detail, "correlation_id": correlationID(writer),
	}, "application/problem+json")
}

func correlationID(writer http.ResponseWriter) string { return writer.Header().Get("X-Correlation-ID") }

func correlationIDs(generator identity.Generator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		value := strings.TrimSpace(request.Header.Get("X-Correlation-ID"))
		if !correlationIDPattern.MatchString(value) {
			generated, err := generator.New()
			if err != nil {
				http.Error(writer, "unable to establish request correlation", http.StatusInternalServerError)
				return
			}
			value = generated
		}
		writer.Header().Set("X-Correlation-ID", strings.ToLower(value))
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writeJSONStatus(writer, status, value, "application/json")
}
func writeJSONStatus(writer http.ResponseWriter, status int, value any, contentType string) {
	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(writer, request)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (writer *statusRecorder) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusRecorder) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func (h *Handler) observe(next http.Handler) http.Handler {
	logger := h.logger
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer}
		next.ServeHTTP(recorder, request)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.InfoContext(request.Context(), "http request completed",
			"method", request.Method,
			"route", request.Pattern,
			"path", request.URL.Path,
			"status", status,
			"latency_ms", time.Since(startedAt).Milliseconds(),
			"correlation_id", recorder.Header().Get("X-Correlation-ID"),
		)
	})
}
