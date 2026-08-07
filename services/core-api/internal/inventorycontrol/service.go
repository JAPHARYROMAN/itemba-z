package inventorycontrol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Repository interface {
	InventoryControlWorkspace(context.Context, tenancy.Scope, string, time.Time) (Workspace, error)
	CreateInventoryPolicy(context.Context, Policy, string, string) (Policy, error)
	TransitionInventoryPolicy(context.Context, tenancy.Scope, string, string, PolicyStatus, string, string, string, time.Time) (Policy, error)
	RegisterReceiptLots(context.Context, LotRegistration, string, string) (LotRegistration, error)
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || ids == nil || c == nil {
		return nil, errors.New("inventory control repository, IDs, and clock are required")
	}
	return &Service{r, ids, c}, nil
}

type PolicyCommand struct {
	Scope                                                                  tenancy.Scope
	ProductID                                                              string
	CostMethod                                                             CostMethod
	LotControlled                                                          bool
	ReorderPoint, ReorderQuantity, MaximumStock, SafetyStock, LeadTimeDays int64
	PreferredSupplierID, Reason, ActorID, IdempotencyKey                   string
}
type PolicyTransitionCommand struct {
	Scope                           tenancy.Scope
	PolicyID                        string
	Status                          PolicyStatus
	Reason, ActorID, IdempotencyKey string
}
type LotCommandLine struct {
	ProductID, LotNumber      string
	Quantity                  int64
	ManufacturedAt, ExpiresAt *time.Time
}
type LotCommand struct {
	Scope                                           tenancy.Scope
	GoodsReceiptID, Reason, ActorID, IdempotencyKey string
	Lines                                           []LotCommandLine
}

func (s *Service) Workspace(ctx context.Context, scope tenancy.Scope, actor string) (Workspace, error) {
	return s.repository.InventoryControlWorkspace(ctx, scope.Normalize(), identity.NormalizeClaim(actor), s.clock.Now().UTC())
}
func (s *Service) CreatePolicy(ctx context.Context, c PolicyCommand) (Policy, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.ProductID = identity.NormalizeClaim(c.ProductID)
	c.PreferredSupplierID = identity.NormalizeClaim(c.PreferredSupplierID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.ProductID) || c.ActorID == "" || (c.CostMethod != StandardCost && c.CostMethod != MovingAverage) || c.ReorderPoint < 0 || c.ReorderQuantity <= 0 || c.MaximumStock < c.ReorderPoint || c.SafetyStock < 0 || c.SafetyStock > c.ReorderPoint || c.LeadTimeDays < 0 || c.LeadTimeDays > 3650 || !wire.IsSafeInteger(c.MaximumStock) || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) || (c.PreferredSupplierID != "" && !identity.IsUUID(c.PreferredSupplierID)) {
		return Policy{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return Policy{}, e
	}
	v := Policy{ID: id, Scope: c.Scope, ProductID: c.ProductID, Status: PolicyDraft, CostMethod: c.CostMethod, LotControlled: c.LotControlled, ReorderPoint: c.ReorderPoint, ReorderQuantity: c.ReorderQuantity, MaximumStock: c.MaximumStock, SafetyStock: c.SafetyStock, LeadTimeDays: c.LeadTimeDays, PreferredSupplierID: c.PreferredSupplierID, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateInventoryPolicy(ctx, v, c.IdempotencyKey, commandHash(c))
}
func (s *Service) TransitionPolicy(ctx context.Context, c PolicyTransitionCommand) (Policy, error) {
	c.Scope = c.Scope.Normalize()
	c.PolicyID = identity.NormalizeClaim(c.PolicyID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.PolicyID) || c.ActorID == "" || (c.Status != PolicySubmitted && c.Status != PolicyActive && c.Status != PolicyRejected) || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) {
		return Policy{}, ErrInvalidCommand
	}
	return s.repository.TransitionInventoryPolicy(ctx, c.Scope, c.ActorID, c.PolicyID, c.Status, c.Reason, c.IdempotencyKey, commandHash(c), s.clock.Now().UTC())
}
func (s *Service) RegisterLots(ctx context.Context, c LotCommand) (LotRegistration, error) {
	c.Scope = c.Scope.Normalize()
	c.GoodsReceiptID = identity.NormalizeClaim(c.GoodsReceiptID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.GoodsReceiptID) || c.ActorID == "" || len(c.Lines) == 0 || len(c.Lines) > 200 || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) {
		return LotRegistration{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return LotRegistration{}, e
	}
	v := LotRegistration{ID: id, Scope: c.Scope, GoodsReceiptID: c.GoodsReceiptID, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC(), Lines: []LotRegistrationLine{}}
	seen := map[string]bool{}
	for _, line := range c.Lines {
		line.ProductID = identity.NormalizeClaim(line.ProductID)
		line.LotNumber = strings.ToUpper(strings.TrimSpace(line.LotNumber))
		key := line.ProductID + ":" + line.LotNumber
		if !identity.IsUUID(line.ProductID) || len(line.LotNumber) < 1 || len(line.LotNumber) > 100 || line.Quantity <= 0 || !wire.IsSafeInteger(line.Quantity) || seen[key] || (line.ManufacturedAt != nil && line.ExpiresAt != nil && !line.ExpiresAt.After(*line.ManufacturedAt)) {
			return LotRegistration{}, ErrInvalidCommand
		}
		seen[key] = true
		lineID, e := s.ids.New()
		if e != nil {
			return LotRegistration{}, e
		}
		v.Lines = append(v.Lines, LotRegistrationLine{ID: lineID, ProductID: line.ProductID, LotNumber: line.LotNumber, Quantity: line.Quantity, ManufacturedAt: utcPtr(line.ManufacturedAt), ExpiresAt: utcPtr(line.ExpiresAt)})
	}
	return s.repository.RegisterReceiptLots(ctx, v, c.IdempotencyKey, commandHash(c))
}
func validReason(v string) bool      { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdempotency(v string) bool { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func commandHash(v any) string {
	b, _ := json.Marshal(v)
	x := sha256.Sum256(b)
	return hex.EncodeToString(x[:])
}
func utcPtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	x := v.UTC()
	return &x
}
