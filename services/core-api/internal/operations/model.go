// Package operations owns commercial documents, procurement, inventory
// controls, supplier subledger postings, and their immutable provenance.
package operations

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type DocumentType string

const (
	Quotation       DocumentType = "QUOTATION"
	SalesOrder      DocumentType = "SALES_ORDER"
	PurchaseRequest DocumentType = "PURCHASE_REQUEST"
	PurchaseOrder   DocumentType = "PURCHASE_ORDER"
	GoodsReceipt    DocumentType = "GOODS_RECEIPT"
	SupplierInvoice DocumentType = "SUPPLIER_INVOICE"
	SupplierPayment DocumentType = "SUPPLIER_PAYMENT"
	PurchaseReturn  DocumentType = "PURCHASE_RETURN"
	StockTransfer   DocumentType = "STOCK_TRANSFER"
	StockCount      DocumentType = "STOCK_COUNT"
	StockAdjustment DocumentType = "STOCK_ADJUSTMENT"
)

type Status string

const (
	Draft      Status = "DRAFT"
	Submitted  Status = "SUBMITTED"
	Approved   Status = "APPROVED"
	Rejected   Status = "REJECTED"
	Posted     Status = "POSTED"
	Dispatched Status = "DISPATCHED"
	Received   Status = "RECEIVED"
	Closed     Status = "CLOSED"
	Reversed   Status = "REVERSED"
)

type PartyType string

const (
	CustomerParty PartyType = "CUSTOMER"
	SupplierParty PartyType = "SUPPLIER"
	NoParty       PartyType = "NONE"
)

type Supplier struct {
	ID               string `json:"id"`
	TenantID         string `json:"tenant_id"`
	CompanyID        string `json:"company_id"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	Active           bool   `json:"active"`
	PaymentTermsDays int64  `json:"payment_terms_days"`
}

type Document struct {
	ID                     string         `json:"id"`
	Scope                  tenancy.Scope  `json:"scope"`
	Number                 string         `json:"number"`
	Type                   DocumentType   `json:"type"`
	Status                 Status         `json:"status"`
	PartyType              PartyType      `json:"party_type"`
	PartyID                string         `json:"party_id,omitempty"`
	SourceDocumentID       string         `json:"source_document_id,omitempty"`
	DestinationWarehouseID string         `json:"destination_warehouse_id,omitempty"`
	Currency               string         `json:"currency"`
	SubtotalMinor          int64          `json:"subtotal_minor"`
	TotalMinor             int64          `json:"total_minor"`
	Reason                 string         `json:"reason"`
	CreatedBy              string         `json:"created_by"`
	CreatedAt              time.Time      `json:"created_at"`
	SubmittedAt            *time.Time     `json:"submitted_at,omitempty"`
	ApprovedAt             *time.Time     `json:"approved_at,omitempty"`
	PostedAt               *time.Time     `json:"posted_at,omitempty"`
	CorrelationID          string         `json:"correlation_id"`
	Lines                  []DocumentLine `json:"lines"`
}

type DocumentLine struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	AmountMinor    int64  `json:"amount_minor"`
}

type Transition struct {
	ID            string    `json:"id"`
	DocumentID    string    `json:"document_id"`
	FromStatus    Status    `json:"from_status"`
	ToStatus      Status    `json:"to_status"`
	Reason        string    `json:"reason"`
	ActorID       string    `json:"actor_id"`
	OccurredAt    time.Time `json:"occurred_at"`
	CorrelationID string    `json:"correlation_id"`
}

type Page struct {
	Items      []Document `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}
type SupplierPage struct {
	Items      []Supplier `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

type PostingConfig struct {
	GRNIAccountID                string
	PayableAccountID             string
	InventoryAdjustmentAccountID string
	StockInTransitAccountID      string
	CashAccounts                 map[string]string
}

var (
	ErrInvalidCommand       = errors.New("operations command is incomplete or invalid")
	ErrInvalidTransition    = errors.New("document status transition is not permitted")
	ErrSourceMismatch       = errors.New("source document, party, product, quantity, or value does not match")
	ErrSeparationOfDuties   = errors.New("the document creator cannot approve this document")
	ErrInsufficientStock    = errors.New("inventory operation would create negative available stock")
	ErrOverReceipt          = errors.New("receipt quantity exceeds the approved purchase order balance")
	ErrOverInvoice          = errors.New("invoice quantity or value exceeds received unmatched goods")
	ErrPayableExceeded      = errors.New("supplier payment exceeds the open payable")
	ErrTransferIncomplete   = errors.New("transfer receipt must match the dispatched transfer")
	ErrPostingConfiguration = errors.New("procurement and inventory posting configuration is incomplete")
)
