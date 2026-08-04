// Package sales coordinates the atomic sale vertical slice across customer,
// catalog, inventory, finance, audit, and event-owning modules.
package sales

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
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

type FiscalStatus string

const (
	FiscalNotConfigured FiscalStatus = "NOT_CONFIGURED"
	FiscalPending       FiscalStatus = "PENDING"
	Fiscalized          FiscalStatus = "FISCALIZED"
	FiscalFailed        FiscalStatus = "FAILED"
)

const (
	PaymentCash         = "CASH"
	PaymentMobileMoney  = "MOBILE_MONEY"
	PaymentBankCard     = "BANK_CARD"
	PaymentBankTransfer = "BANK_TRANSFER"
)

func IsCanonicalPaymentMethod(value string) bool {
	switch value {
	case PaymentCash, PaymentMobileMoney, PaymentBankCard, PaymentBankTransfer:
		return true
	default:
		return false
	}
}

type Sale struct {
	ID                   string        `json:"id"`
	Scope                tenancy.Scope `json:"scope"`
	RecordType           RecordType    `json:"record_type"`
	Kind                 Kind          `json:"kind"`
	Status               Status        `json:"status"`
	CustomerID           string        `json:"customer_id"`
	Currency             string        `json:"currency"`
	SubtotalMinor        int64         `json:"subtotal_minor"`
	TaxMinor             int64         `json:"tax_minor"`
	TotalMinor           int64         `json:"total_minor"`
	COGSMinor            int64         `json:"cogs_minor"`
	PaymentMethod        string        `json:"payment_method,omitempty"`
	DeviceID             string        `json:"device_id,omitempty"`
	ClientTransactionID  string        `json:"client_transaction_id,omitempty"`
	ClientTimestamp      *time.Time    `json:"client_timestamp,omitempty"`
	AppVersion           string        `json:"app_version,omitempty"`
	MasterDataVersion    int64         `json:"master_data_version,omitempty"`
	PriceVersion         int64         `json:"price_version,omitempty"`
	CatalogSnapshotToken string        `json:"catalog_snapshot_token,omitempty"`
	Offline              bool          `json:"offline,omitempty"`
	ReceiptReference     string        `json:"receipt_reference"`
	FiscalStatus         FiscalStatus  `json:"fiscal_status"`
	ReversalOf           string        `json:"reversal_of,omitempty"`
	ReversalReason       string        `json:"reversal_reason,omitempty"`
	CreatedBy            string        `json:"created_by"`
	CorrelationID        string        `json:"correlation_id"`
	CreatedAt            time.Time     `json:"created_at"`
	ReversedAt           *time.Time    `json:"reversed_at,omitempty"`
	Lines                []Line        `json:"lines"`
	IdempotentReplay     bool          `json:"-"`
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
	Scope                tenancy.Scope `json:"scope"`
	CustomerID           string        `json:"customer_id"`
	Kind                 Kind          `json:"kind"`
	PaymentMethod        string        `json:"payment_method,omitempty"`
	Lines                []CommandLine `json:"lines"`
	ActorID              string        `json:"actor_id"`
	CorrelationID        string        `json:"-"`
	IdempotencyKey       string        `json:"-"`
	DeviceID             string        `json:"device_id,omitempty"`
	ClientTransactionID  string        `json:"client_transaction_id,omitempty"`
	ClientTimestamp      time.Time     `json:"client_timestamp,omitempty"`
	AppVersion           string        `json:"app_version,omitempty"`
	MasterDataVersion    int64         `json:"master_data_version,omitempty"`
	PriceVersion         int64         `json:"price_version,omitempty"`
	CatalogSnapshotToken string        `json:"catalog_snapshot_token,omitempty"`
	SyncAttempt          int           `json:"-"`
	Offline              bool          `json:"offline,omitempty"`
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
	if utf8.RuneCountInString(c.PaymentMethod) > 64 {
		return ErrInvalidCommand
	}
	if c.Kind == KindCredit && strings.TrimSpace(c.PaymentMethod) != "" {
		return ErrInvalidCommand
	}
	hasDevice := strings.TrimSpace(c.DeviceID) != ""
	hasClientTransaction := strings.TrimSpace(c.ClientTransactionID) != ""
	if hasDevice != hasClientTransaction {
		return ErrInvalidCommand
	}
	if hasDevice {
		if c.ClientTimestamp.IsZero() || strings.TrimSpace(c.AppVersion) == "" ||
			c.MasterDataVersion < 1 || c.PriceVersion < 1 || c.SyncAttempt < 1 ||
			!identity.IsUUID(c.CatalogSnapshotToken) || c.CatalogSnapshotToken == devices.UnacknowledgedCatalogSnapshotToken {
			return ErrInvalidCommand
		}
		if c.MasterDataVersion > MaxWireSafeInteger || c.PriceVersion > MaxWireSafeInteger || int64(c.SyncAttempt) > MaxWireSafeInteger {
			return ErrUnsafeWireInteger
		}
		if c.Offline && c.Kind == KindCredit {
			return ErrOfflineCredit
		}
	}
	products := make(map[string]struct{}, len(c.Lines))
	for _, line := range c.Lines {
		if strings.TrimSpace(line.ProductID) == "" || line.Quantity <= 0 {
			return ErrInvalidLine
		}
		if line.Quantity > MaxWireSafeInteger {
			return ErrUnsafeWireInteger
		}
		productID := identity.NormalizeClaim(line.ProductID)
		if _, duplicate := products[productID]; duplicate {
			return ErrDuplicateProductLine
		}
		products[productID] = struct{}{}
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
	if strings.TrimSpace(c.SaleID) == "" || strings.TrimSpace(c.Reason) == "" || utf8.RuneCountInString(strings.TrimSpace(c.Reason)) > 500 ||
		strings.TrimSpace(c.ActorID) == "" || strings.TrimSpace(c.IdempotencyKey) == "" {
		return ErrInvalidCommand
	}
	return nil
}

var (
	ErrInvalidCommand         = errors.New("sale command is incomplete")
	ErrInvalidLine            = errors.New("sale line requires a product and positive quantity")
	ErrDuplicateProductLine   = errors.New("a product may appear only once in a sale")
	ErrInvalidSaleKind        = errors.New("sale kind must be CASH or CREDIT")
	ErrPaymentMethodRequired  = errors.New("cash sale requires a payment method")
	ErrUnsupportedPayment     = errors.New("cash payment method is unsupported or not configured")
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
	ErrOfflineCredit          = errors.New("offline sales must be cash sales")
	ErrOfflinePaymentMethod   = errors.New("offline sales require the physical CASH payment method")
	ErrOfflineTaxUnsupported  = errors.New("offline sales currently support only zero-rated products")
	ErrOfflineReconciliation  = errors.New("offline catalog publication evidence is unavailable; retain the exact command for governed reconciliation")
	ErrUnsafeWireInteger      = wire.ErrUnsafeInteger
)
