// Package inventorycontrol owns governed replenishment, costing, and lot/expiry controls.
package inventorycontrol

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type PolicyStatus string
type CostMethod string

const (
	PolicyDraft     PolicyStatus = "DRAFT"
	PolicySubmitted PolicyStatus = "SUBMITTED"
	PolicyActive    PolicyStatus = "ACTIVE"
	PolicyRejected  PolicyStatus = "REJECTED"
	StandardCost    CostMethod   = "STANDARD"
	MovingAverage   CostMethod   = "MOVING_AVERAGE"
)

type Policy struct {
	ID                  string        `json:"id"`
	Scope               tenancy.Scope `json:"scope"`
	ProductID           string        `json:"product_id"`
	Status              PolicyStatus  `json:"status"`
	CostMethod          CostMethod    `json:"cost_method"`
	LotControlled       bool          `json:"lot_controlled"`
	ReorderPoint        int64         `json:"reorder_point"`
	ReorderQuantity     int64         `json:"reorder_quantity"`
	MaximumStock        int64         `json:"maximum_stock"`
	SafetyStock         int64         `json:"safety_stock"`
	LeadTimeDays        int64         `json:"lead_time_days"`
	PreferredSupplierID string        `json:"preferred_supplier_id,omitempty"`
	Reason              string        `json:"reason"`
	CreatedBy           string        `json:"created_by"`
	CreatedAt           time.Time     `json:"created_at"`
	ApprovedBy          string        `json:"approved_by,omitempty"`
}

type LotRegistrationLine struct {
	ID             string     `json:"id"`
	ProductID      string     `json:"product_id"`
	LotNumber      string     `json:"lot_number"`
	Quantity       int64      `json:"quantity"`
	ManufacturedAt *time.Time `json:"manufactured_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type LotRegistration struct {
	ID             string                `json:"id"`
	Scope          tenancy.Scope         `json:"scope"`
	GoodsReceiptID string                `json:"goods_receipt_id"`
	Reason         string                `json:"reason"`
	CreatedBy      string                `json:"created_by"`
	CreatedAt      time.Time             `json:"created_at"`
	Lines          []LotRegistrationLine `json:"lines"`
}

type LotBalance struct {
	LotID          string     `json:"lot_id"`
	ProductID      string     `json:"product_id"`
	LotNumber      string     `json:"lot_number"`
	ManufacturedAt *time.Time `json:"manufactured_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Quantity       int64      `json:"quantity"`
}

type Replenishment struct {
	ProductID           string `json:"product_id"`
	OnHand              int64  `json:"on_hand"`
	Reserved            int64  `json:"reserved"`
	Available           int64  `json:"available"`
	Incoming            int64  `json:"incoming"`
	Projected           int64  `json:"projected"`
	ReorderPoint        int64  `json:"reorder_point"`
	RecommendedQuantity int64  `json:"recommended_quantity"`
	PreferredSupplierID string `json:"preferred_supplier_id,omitempty"`
	ActionRequired      bool   `json:"action_required"`
}

type CostHistory struct {
	ID               string     `json:"id"`
	ProductID        string     `json:"product_id"`
	SourceID         string     `json:"source_id"`
	Method           CostMethod `json:"method"`
	QuantityBefore   int64      `json:"quantity_before"`
	QuantityReceived int64      `json:"quantity_received"`
	CostBeforeMinor  int64      `json:"cost_before_minor"`
	ReceiptCostMinor int64      `json:"receipt_cost_minor"`
	CostAfterMinor   int64      `json:"cost_after_minor"`
	OccurredAt       time.Time  `json:"occurred_at"`
}

type Workspace struct {
	Policies       []Policy          `json:"policies"`
	Registrations  []LotRegistration `json:"registrations"`
	Lots           []LotBalance      `json:"lots"`
	Replenishments []Replenishment   `json:"replenishments"`
	CostHistory    []CostHistory     `json:"cost_history"`
}

var (
	ErrInvalidCommand     = errors.New("inventory control command is invalid")
	ErrInvalidTransition  = errors.New("inventory policy transition is invalid")
	ErrSeparationOfDuties = errors.New("inventory policy maker cannot approve own revision")
	ErrLotReconciliation  = errors.New("lot balances must reconcile to warehouse stock")
	ErrLotAllocation      = errors.New("lot allocation must exactly match the goods receipt")
)
