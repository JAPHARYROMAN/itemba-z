// Package integrations owns the provider-neutral external delivery boundary.
// Core ERP transactions remain authoritative even when an external provider is
// unavailable; delivery evidence is durable, retryable, and independently
// reconcilable.
package integrations

import (
	"encoding/json"
	"errors"
	"time"
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

const (
	Pending        Status = "PENDING"
	InFlight       Status = "IN_FLIGHT"
	RetryScheduled Status = "RETRY_SCHEDULED"
	Succeeded      Status = "SUCCEEDED"
	DeadLetter     Status = "DEAD_LETTER"
	Cancelled      Status = "CANCELLED"
)

type Route struct {
	ID                      string
	TenantID                string
	CompanyID               string
	Capability              Capability
	ProviderCode            string
	ContractVersion         string
	EndpointURL             string
	SecretReference         string
	Timeout                 time.Duration
	MaxAttempts             int
	BaseBackoff             time.Duration
	MaxBackoff              time.Duration
	CircuitFailureThreshold int
	CircuitOpen             time.Duration
}

type Delivery struct {
	ID                string
	TenantID          string
	CompanyID         string
	Route             Route
	Capability        Capability
	Operation         string
	SourceEventID     string
	AggregateType     string
	AggregateID       string
	CorrelationID     string
	IdempotencyKey    string
	RequestHash       string
	RequestPayload    json.RawMessage
	Status            Status
	AttemptCount      int
	MaxAttempts       int
	AvailableAt       time.Time
	AttemptStartedAt  time.Time
	LockedBy          string
	ProviderReference string
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
)
