// Package integrations owns the provider-neutral external delivery boundary.
// Core ERP transactions remain authoritative even when an external provider is
// unavailable; delivery evidence is durable, retryable, and independently
// reconcilable.
package integrations

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Capability string

const (
	TRAFiscalization Capability = "TRA_FISCALIZATION"
	PaymentCallback  Capability = "PAYMENT_CALLBACK"
	BankStatement    Capability = "BANK_STATEMENT_IMPORT"
	PayrollExport    Capability = "PAYROLL_EXPORT"
	EmailDelivery    Capability = "EMAIL"
	WhatsAppDelivery Capability = "WHATSAPP"
	ReceiptPrint     Capability = "RECEIPT_PRINT"
)

type Status string
type RouteStatus string

const (
	Pending        Status      = "PENDING"
	InFlight       Status      = "IN_FLIGHT"
	RetryScheduled Status      = "RETRY_SCHEDULED"
	Succeeded      Status      = "SUCCEEDED"
	DeadLetter     Status      = "DEAD_LETTER"
	Cancelled      Status      = "CANCELLED"
	RouteDraft     RouteStatus = "DRAFT"
	RouteActive    RouteStatus = "ACTIVE"
	RouteSuspended RouteStatus = "SUSPENDED"
	RouteRetired   RouteStatus = "RETIRED"
)

type Route struct {
	ID                      string        `json:"id"`
	TenantID                string        `json:"tenant_id"`
	CompanyID               string        `json:"company_id"`
	Capability              Capability    `json:"capability"`
	ProviderCode            string        `json:"provider_code"`
	ContractVersion         string        `json:"contract_version"`
	EndpointURL             string        `json:"endpoint_url"`
	SecretReference         string        `json:"secret_reference"`
	Timeout                 time.Duration `json:"-"`
	TimeoutMilliseconds     int           `json:"timeout_milliseconds"`
	MaxAttempts             int           `json:"max_attempts"`
	BaseBackoff             time.Duration `json:"-"`
	BaseBackoffSeconds      int           `json:"base_backoff_seconds"`
	MaxBackoff              time.Duration `json:"-"`
	MaxBackoffSeconds       int           `json:"max_backoff_seconds"`
	CircuitFailureThreshold int           `json:"circuit_failure_threshold"`
	CircuitOpen             time.Duration `json:"-"`
	CircuitOpenSeconds      int           `json:"circuit_open_seconds"`
	Status                  RouteStatus   `json:"status"`
	ValidFrom               time.Time     `json:"valid_from"`
	ValidUntil              *time.Time    `json:"valid_until,omitempty"`
	Reason                  string        `json:"reason"`
	CreatedBy               string        `json:"created_by"`
	CreatedAt               time.Time     `json:"created_at"`
	ApprovedBy              string        `json:"approved_by,omitempty"`
	ApprovedAt              *time.Time    `json:"approved_at,omitempty"`
	Health                  RouteHealth   `json:"health"`
}

type RouteHealth struct {
	ConsecutiveFailures int        `json:"consecutive_failures"`
	OpenedUntil         *time.Time `json:"opened_until,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

type Delivery struct {
	ID                string          `json:"id"`
	TenantID          string          `json:"tenant_id"`
	CompanyID         string          `json:"company_id"`
	Route             Route           `json:"route"`
	Capability        Capability      `json:"capability"`
	Operation         string          `json:"operation"`
	SourceEventID     string          `json:"source_event_id"`
	AggregateType     string          `json:"aggregate_type"`
	AggregateID       string          `json:"aggregate_id"`
	CorrelationID     string          `json:"correlation_id"`
	IdempotencyKey    string          `json:"idempotency_key"`
	RequestHash       string          `json:"request_hash"`
	RequestPayload    json.RawMessage `json:"request_payload,omitempty"`
	Status            Status          `json:"status"`
	AttemptCount      int             `json:"attempt_count"`
	MaxAttempts       int             `json:"max_attempts"`
	AvailableAt       time.Time       `json:"available_at"`
	AttemptStartedAt  time.Time       `json:"attempt_started_at,omitempty"`
	LockedBy          string          `json:"locked_by,omitempty"`
	ProviderReference string          `json:"provider_reference,omitempty"`
	ResponsePayload   json.RawMessage `json:"response_payload,omitempty"`
	LastErrorCode     string          `json:"last_error_code,omitempty"`
	LastErrorMessage  string          `json:"last_error_message,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
}

type Attempt struct {
	ID                string          `json:"id"`
	DeliveryID        string          `json:"delivery_id"`
	AttemptNumber     int             `json:"attempt_number"`
	WorkerID          string          `json:"worker_id"`
	Outcome           Status          `json:"outcome"`
	ErrorCode         string          `json:"error_code,omitempty"`
	ErrorMessage      string          `json:"error_message,omitempty"`
	ProviderReference string          `json:"provider_reference,omitempty"`
	ResponsePayload   json.RawMessage `json:"response_payload,omitempty"`
	StartedAt         time.Time       `json:"started_at"`
	CompletedAt       time.Time       `json:"completed_at"`
}

type Reconciliation struct {
	Total        int `json:"total"`
	Pending      int `json:"pending"`
	InFlight     int `json:"in_flight"`
	Retrying     int `json:"retrying"`
	Succeeded    int `json:"succeeded"`
	DeadLetter   int `json:"dead_letter"`
	Unreconciled int `json:"unreconciled"`
}

type Workspace struct {
	Scope          tenancy.Scope  `json:"scope"`
	Routes         []Route        `json:"routes"`
	Deliveries     []Delivery     `json:"deliveries"`
	Attempts       []Attempt      `json:"attempts"`
	Reconciliation Reconciliation `json:"reconciliation"`
}

type Result struct {
	ProviderReference string
	Response          json.RawMessage
}

type Failure struct {
	Code       string
	Message    string
	Retryable  bool
	RetryAfter time.Duration
}

func (f Failure) Error() string {
	if f.Message != "" {
		return f.Message
	}
	return f.Code
}

var (
	ErrInvalidConfiguration = errors.New("integration processor configuration is incomplete")
	ErrConnectorUnavailable = errors.New("integration connector is not registered")
	ErrInvalidCommand       = errors.New("integration command is invalid")
	ErrInvalidTransition    = errors.New("integration route transition is invalid")
	ErrSeparationOfDuties   = errors.New("integration route maker cannot approve the route")
	ErrReplayUnavailable    = errors.New("only dead-letter integration deliveries can be replayed")
)
