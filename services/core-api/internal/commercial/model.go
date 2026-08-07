// Package commercial owns governed supplier/product masters and competitive sourcing.
package commercial

import (
	"errors"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"time"
)

type EntityType string
type Status string
type RFQStatus string
type QuoteStatus string

const (
	SupplierEntity EntityType  = "SUPPLIER"
	ProductEntity  EntityType  = "PRODUCT"
	Draft          Status      = "DRAFT"
	Submitted      Status      = "SUBMITTED"
	Active         Status      = "ACTIVE"
	Rejected       Status      = "REJECTED"
	RFQDraft       RFQStatus   = "DRAFT"
	RFQSubmitted   RFQStatus   = "SUBMITTED"
	RFQApproved    RFQStatus   = "APPROVED"
	RFQClosed      RFQStatus   = "CLOSED"
	RFQCancelled   RFQStatus   = "CANCELLED"
	QuoteDraft     QuoteStatus = "DRAFT"
	QuoteSubmitted QuoteStatus = "SUBMITTED"
	QuoteSelected  QuoteStatus = "SELECTED"
	QuoteRejected  QuoteStatus = "REJECTED"
)

type SupplierData struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	TaxID            string `json:"tax_id"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	PaymentTermsDays int64  `json:"payment_terms_days"`
	Active           bool   `json:"active"`
}
type ProductData struct {
	SKU                string `json:"sku"`
	Name               string `json:"name"`
	BaseUnitCode       string `json:"base_unit_code"`
	Currency           string `json:"currency"`
	ListPriceMinor     int64  `json:"list_price_minor"`
	StandardCostMinor  int64  `json:"standard_cost_minor"`
	TaxCode            string `json:"tax_code"`
	RevenueAccountID   string `json:"revenue_account_id"`
	COGSAccountID      string `json:"cogs_account_id"`
	InventoryAccountID string `json:"inventory_account_id"`
	Active             bool   `json:"active"`
}
type Revision struct {
	ID         string        `json:"id"`
	Scope      tenancy.Scope `json:"scope"`
	EntityType EntityType    `json:"entity_type"`
	EntityID   string        `json:"entity_id"`
	Status     Status        `json:"status"`
	Supplier   *SupplierData `json:"supplier,omitempty"`
	Product    *ProductData  `json:"product,omitempty"`
	Reason     string        `json:"reason"`
	CreatedBy  string        `json:"created_by"`
	CreatedAt  time.Time     `json:"created_at"`
	ApprovedBy string        `json:"approved_by,omitempty"`
}
type RFQLine struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}
type RFQ struct {
	ID            string        `json:"id"`
	Scope         tenancy.Scope `json:"scope"`
	Number        string        `json:"number"`
	Status        RFQStatus     `json:"status"`
	Currency      string        `json:"currency"`
	ResponseDueAt time.Time     `json:"response_due_at"`
	Reason        string        `json:"reason"`
	CreatedBy     string        `json:"created_by"`
	CreatedAt     time.Time     `json:"created_at"`
	ApprovedBy    string        `json:"approved_by,omitempty"`
	Lines         []RFQLine     `json:"lines"`
}
type QuoteLine struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	AmountMinor    int64  `json:"amount_minor"`
}
type SupplierQuote struct {
	ID               string        `json:"id"`
	Scope            tenancy.Scope `json:"scope"`
	RFQID            string        `json:"rfq_id"`
	SupplierID       string        `json:"supplier_id"`
	Reference        string        `json:"reference"`
	Status           QuoteStatus   `json:"status"`
	Currency         string        `json:"currency"`
	DeliveryDays     int64         `json:"delivery_days"`
	PaymentTermsDays int64         `json:"payment_terms_days"`
	ValidUntil       time.Time     `json:"valid_until"`
	TotalMinor       int64         `json:"total_minor"`
	Reason           string        `json:"reason"`
	CreatedBy        string        `json:"created_by"`
	CreatedAt        time.Time     `json:"created_at"`
	Lines            []QuoteLine   `json:"lines"`
}
type ComparisonItem struct {
	QuoteID          string    `json:"quote_id"`
	SupplierID       string    `json:"supplier_id"`
	Reference        string    `json:"reference"`
	TotalMinor       int64     `json:"total_minor"`
	DeliveryDays     int64     `json:"delivery_days"`
	PaymentTermsDays int64     `json:"payment_terms_days"`
	ValidUntil       time.Time `json:"valid_until"`
	Comparable       bool      `json:"comparable"`
	Selected         bool      `json:"selected"`
}
type Comparison struct {
	RFQ    RFQ              `json:"rfq"`
	Quotes []ComparisonItem `json:"quotes"`
}
type Award struct {
	ID              string        `json:"id"`
	Scope           tenancy.Scope `json:"scope"`
	RFQID           string        `json:"rfq_id"`
	QuoteID         string        `json:"quote_id"`
	SupplierID      string        `json:"supplier_id"`
	PurchaseOrderID string        `json:"purchase_order_id"`
	Reason          string        `json:"reason"`
	SelectedBy      string        `json:"selected_by"`
	SelectedAt      time.Time     `json:"selected_at"`
}
type Workspace struct {
	Revisions []Revision      `json:"revisions"`
	RFQs      []RFQ           `json:"rfqs"`
	Quotes    []SupplierQuote `json:"quotes"`
	Awards    []Award         `json:"awards"`
}

var (
	ErrInvalidCommand     = errors.New("commercial command is invalid")
	ErrInvalidTransition  = errors.New("commercial transition is invalid")
	ErrSeparationOfDuties = errors.New("maker cannot approve own commercial record")
	ErrSourceMismatch     = errors.New("supplier quote does not match approved RFQ")
	ErrAwardExists        = errors.New("RFQ already has an award")
)
