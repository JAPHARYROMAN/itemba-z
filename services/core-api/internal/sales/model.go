// Package sales coordinates the atomic sale vertical slice across customer,
// catalog, inventory, finance, audit, and event-owning modules.
package sales

import (
	"errors"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Kind string

const (
	KindCash   Kind = "CASH"
	KindCredit Kind = "CREDIT"
)

type RecordType string

const (
	RecordSale     RecordType = "SALE"
	RecordReversal RecordType = "REVERSAL"
)

type Status string

const (
	StatusPosted   Status = "POSTED"
	StatusReversed Status = "REVERSED"
)

type Sale struct {
	ID             string        `json:"id"`
	Scope          tenancy.Scope `json:"scope"`
	RecordType     RecordType    `json:"record_type"`
	Kind           Kind          `json:"kind"`
	Status         Status        `json:"status"`
	CustomerID     string        `json:"customer_id"`
	Currency       string        `json:"currency"`
	SubtotalMinor  int64         `json:"subtotal_minor"`
	TaxMinor       int64         `json:"tax_minor"`
	TotalMinor     int64         `json:"total_minor"`
	COGSMinor      int64         `json:"cogs_minor"`
	PaymentMethod  string        `json:"payment_method,omitempty"`
	ReversalOf     string        `json:"reversal_of,omitempty"`
	ReversalReason string        `json:"reversal_reason,omitempty"`
	CreatedBy      string        `json:"created_by"`
	CorrelationID  string        `json:"correlation_id"`
	CreatedAt      time.Time     `json:"created_at"`
	ReversedAt     *time.Time    `json:"reversed_at,omitempty"`
	Lines          []Line        `json:"lines"`
}

type Line struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	SubtotalMinor  int64  `json:"subtotal_minor"`
	TaxMinor       int64  `json:"tax_minor"`
	TotalMinor     int64  `json:"total_minor"`
	UnitCostMinor  int64  `json:"unit_cost_minor"`
	COGSMinor      int64  `json:"cogs_minor"`
}

type Payment struct {
	ID          string
	TenantID    string
	CompanyID   string
	SaleID      string
	AccountID   string
	Method      string
	AmountMinor int64
	Currency    string
	OccurredAt  time.Time
}

type CompleteCommand struct {
	Scope          tenancy.Scope `json:"scope"`
	CustomerID     string        `json:"customer_id"`
	Kind           Kind          `json:"kind"`
	PaymentMethod  string        `json:"payment_method,omitempty"`
	Lines          []CommandLine `json:"lines"`
	ActorID        string        `json:"actor_id"`
	CorrelationID  string        `json:"-"`
	IdempotencyKey string        `json:"-"`
}

type CommandLine struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}

func (c CompleteCommand) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.CustomerID) == "" || strings.TrimSpace(c.ActorID) == "" ||
		strings.TrimSpace(c.IdempotencyKey) == "" || len(c.Lines) == 0 {
		return ErrInvalidCommand
	}
	if c.Kind != KindCash && c.Kind != KindCredit {
		return ErrInvalidSaleKind
	}
	if c.Kind == KindCash && strings.TrimSpace(c.PaymentMethod) == "" {
		return ErrPaymentMethodRequired
	}
	if c.Kind == KindCredit && strings.TrimSpace(c.PaymentMethod) != "" {
		return ErrInvalidCommand
	}
	for _, line := range c.Lines {
		if strings.TrimSpace(line.ProductID) == "" || line.Quantity <= 0 {
			return ErrInvalidLine
		}
	}
	return nil
}

type ReverseCommand struct {
	Scope          tenancy.Scope `json:"scope"`
	SaleID         string        `json:"sale_id"`
	Reason         string        `json:"reason"`
	ActorID        string        `json:"actor_id"`
	CorrelationID  string        `json:"-"`
	IdempotencyKey string        `json:"-"`
}

func (c ReverseCommand) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.SaleID) == "" || strings.TrimSpace(c.Reason) == "" ||
		strings.TrimSpace(c.ActorID) == "" || strings.TrimSpace(c.IdempotencyKey) == "" {
		return ErrInvalidCommand
	}
	return nil
}

var (
	ErrInvalidCommand         = errors.New("sale command is incomplete")
	ErrInvalidLine            = errors.New("sale line requires a product and positive quantity")
	ErrInvalidSaleKind        = errors.New("sale kind must be CASH or CREDIT")
	ErrPaymentMethodRequired  = errors.New("cash sale requires a payment method")
	ErrNotFound               = errors.New("resource not found in the requested scope")
	ErrCustomerInactive       = errors.New("customer account is inactive")
	ErrGeneralCustomerCredit  = errors.New("General Customer cannot buy on credit")
	ErrCustomerCreditDisabled = errors.New("customer is not enabled for credit")
	ErrCreditLimitExceeded    = errors.New("customer credit limit would be exceeded")
	ErrProductInactive        = errors.New("product is inactive")
	ErrInsufficientStock      = errors.New("insufficient available stock")
	ErrFiscalPeriodClosed     = errors.New("fiscal period is closed")
	ErrPostingConfig          = errors.New("sales accounting configuration is incomplete")
	ErrIdempotencyConflict    = errors.New("idempotency key was already used for a different request")
	ErrAlreadyReversed        = errors.New("sale has already been reversed")
	ErrMoneyOverflow          = errors.New("calculated money amount exceeds supported range")
	ErrForbidden              = errors.New("actor is not authorized for this operation and scope")
)
