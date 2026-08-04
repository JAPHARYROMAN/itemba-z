// Package httpapi exposes the versioned REST boundary for the core application.
// Authentication is delegated to an explicit adapter that returns a verified
// actor and organizational scope. Production never derives identity from X-* headers.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
)

const maxBodyBytes = 1 << 20

var correlationIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Handler struct {
	sales         *sales.Service
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

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.health)
	mux.HandleFunc("POST /v1/sales", h.completeSale)
	mux.HandleFunc("GET /v1/sales/{saleID}", h.getSale)
	mux.HandleFunc("POST /v1/sales/{saleID}/reversals", h.reverseSale)
	return securityHeaders(correlationIDs(identity.UUIDGenerator{}, mux))
}

func (h *Handler) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "healthy", "version": "dev"})
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
	if idempotencyKey == "" {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header is required")
		return
	}
	var body completeSaleRequest
	if !decodeJSON(writer, request, &body) {
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
	writeJSON(writer, http.StatusCreated, result)
}

func (h *Handler) getSale(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.requestContext(writer, request)
	if !ok {
		return
	}
	result, err := h.sales.Get(request.Context(), principal.Scope, principal.ActorID, request.PathValue("saleID"))
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
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
	if idempotencyKey == "" {
		writeProblem(writer, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header is required")
		return
	}
	var body reverseSaleRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	result, err := h.sales.Reverse(request.Context(), sales.ReverseCommand{
		Scope: principal.Scope, SaleID: request.PathValue("saleID"), Reason: body.Reason,
		ActorID: principal.ActorID, CorrelationID: correlationID(writer), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		h.writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (h *Handler) requestContext(writer http.ResponseWriter, request *http.Request) (Principal, bool) {
	principal, err := h.authenticator.Authenticate(request.Context(), request)
	if err != nil || principal.Validate() != nil {
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
	if decoder.Decode(&extra) == nil {
		writeProblem(writer, http.StatusBadRequest, "invalid_json", "request body must contain one JSON value")
		return false
	}
	return true
}

func (h *Handler) writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, sales.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, sales.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, sales.ErrIdempotencyConflict):
		status, code = http.StatusConflict, "idempotency_conflict"
	case errors.Is(err, sales.ErrInsufficientStock):
		status, code = http.StatusConflict, "insufficient_stock"
	case errors.Is(err, sales.ErrFiscalPeriodClosed):
		status, code = http.StatusConflict, "fiscal_period_closed"
	case errors.Is(err, sales.ErrAlreadyReversed):
		status, code = http.StatusConflict, "already_reversed"
	case errors.Is(err, sales.ErrGeneralCustomerCredit), errors.Is(err, sales.ErrCustomerCreditDisabled), errors.Is(err, sales.ErrCreditLimitExceeded), errors.Is(err, sales.ErrCustomerInactive), errors.Is(err, sales.ErrProductInactive):
		status, code = http.StatusUnprocessableEntity, "business_rule_violation"
	case errors.Is(err, sales.ErrInvalidCommand), errors.Is(err, sales.ErrInvalidLine), errors.Is(err, sales.ErrInvalidSaleKind), errors.Is(err, sales.ErrPaymentMethodRequired):
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
