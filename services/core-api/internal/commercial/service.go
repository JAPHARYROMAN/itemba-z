package commercial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"strings"
	"time"
	"unicode/utf8"
)

type Repository interface {
	CommercialWorkspace(context.Context, tenancy.Scope, string) (Workspace, error)
	CreateMasterRevision(context.Context, Revision, string, string) (Revision, error)
	TransitionMasterRevision(context.Context, tenancy.Scope, string, string, Status, string, string, string, time.Time) (Revision, error)
	CreateRFQ(context.Context, RFQ, string, string) (RFQ, error)
	TransitionRFQ(context.Context, tenancy.Scope, string, string, RFQStatus, string, string, string, time.Time) (RFQ, error)
	CreateSupplierQuote(context.Context, SupplierQuote, string, string) (SupplierQuote, error)
	SubmitSupplierQuote(context.Context, tenancy.Scope, string, string, string, string, string, time.Time) (SupplierQuote, error)
	RFQComparison(context.Context, tenancy.Scope, string, string) (Comparison, error)
	AwardRFQ(context.Context, Award, string, string) (Award, error)
}
type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, i identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || i == nil || c == nil {
		return nil, errors.New("commercial repository, id generator, and clock are required")
	}
	return &Service{r, i, c}, nil
}

type RFQCommandLine struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}
type QuoteCommandLine struct {
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
}
type RevisionCommand struct {
	Scope                           tenancy.Scope
	EntityType                      EntityType
	EntityID                        string
	Supplier                        *SupplierData
	Product                         *ProductData
	Reason, ActorID, IdempotencyKey string
}
type RevisionTransitionCommand struct {
	Scope                           tenancy.Scope
	RevisionID                      string
	Status                          Status
	Reason, ActorID, IdempotencyKey string
}
type RFQCommand struct {
	Scope                           tenancy.Scope
	Currency                        string
	ResponseDueAt                   time.Time
	Reason, ActorID, IdempotencyKey string
	Lines                           []RFQCommandLine
}
type RFQTransitionCommand struct {
	Scope                           tenancy.Scope
	RFQID                           string
	Status                          RFQStatus
	Reason, ActorID, IdempotencyKey string
}
type QuoteCommand struct {
	Scope                                  tenancy.Scope
	RFQID, SupplierID, Reference, Currency string
	DeliveryDays, PaymentTermsDays         int64
	ValidUntil                             time.Time
	Reason, ActorID, IdempotencyKey        string
	Lines                                  []QuoteCommandLine
}
type QuoteSubmitCommand struct {
	Scope                                    tenancy.Scope
	QuoteID, Reason, ActorID, IdempotencyKey string
}
type AwardCommand struct {
	Scope                                           tenancy.Scope
	RFQID, QuoteID, Reason, ActorID, IdempotencyKey string
}

func (s *Service) Workspace(ctx context.Context, scope tenancy.Scope, actor string) (Workspace, error) {
	return s.repository.CommercialWorkspace(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) CreateRevision(ctx context.Context, c RevisionCommand) (Revision, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.EntityID = identity.NormalizeClaim(c.EntityID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || c.ActorID == "" || (c.EntityType != SupplierEntity && c.EntityType != ProductEntity) || (c.Supplier == nil) == (c.Product == nil) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Revision{}, ErrInvalidCommand
	}
	if c.EntityID == "" {
		var e error
		c.EntityID, e = s.ids.New()
		if e != nil {
			return Revision{}, e
		}
	} else if !identity.IsUUID(c.EntityID) {
		return Revision{}, ErrInvalidCommand
	}
	if c.Supplier != nil {
		normalizeSupplier(c.Supplier)
		if !validSupplier(*c.Supplier) {
			return Revision{}, ErrInvalidCommand
		}
	}
	if c.Product != nil {
		normalizeProduct(c.Product)
		if !validProduct(*c.Product) {
			return Revision{}, ErrInvalidCommand
		}
	}
	id, e := s.ids.New()
	if e != nil {
		return Revision{}, e
	}
	v := Revision{ID: id, Scope: c.Scope, EntityType: c.EntityType, EntityID: c.EntityID, Status: Draft, Supplier: c.Supplier, Product: c.Product, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC()}
	return s.repository.CreateMasterRevision(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionRevision(ctx context.Context, c RevisionTransitionCommand) (Revision, error) {
	c.Scope = c.Scope.Normalize()
	c.RevisionID = identity.NormalizeClaim(c.RevisionID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.RevisionID) || c.ActorID == "" || (c.Status != Submitted && c.Status != Active && c.Status != Rejected) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Revision{}, ErrInvalidCommand
	}
	return s.repository.TransitionMasterRevision(ctx, c.Scope, c.ActorID, c.RevisionID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) CreateRFQ(ctx context.Context, c RFQCommand) (RFQ, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Currency) != 3 || !c.ResponseDueAt.After(s.clock.Now()) || len(c.Lines) == 0 || len(c.Lines) > 60 || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return RFQ{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return RFQ{}, e
	}
	v := RFQ{ID: id, Scope: c.Scope, Number: "RFQ-" + strings.ToUpper(strings.ReplaceAll(id, "-", "")[:12]), Status: RFQDraft, Currency: c.Currency, ResponseDueAt: c.ResponseDueAt.UTC(), Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC(), Lines: []RFQLine{}}
	seen := map[string]bool{}
	for _, x := range c.Lines {
		x.ProductID = identity.NormalizeClaim(x.ProductID)
		if !identity.IsUUID(x.ProductID) || x.Quantity <= 0 || !wire.IsSafeInteger(x.Quantity) || seen[x.ProductID] {
			return RFQ{}, ErrInvalidCommand
		}
		seen[x.ProductID] = true
		lineID, e := s.ids.New()
		if e != nil {
			return RFQ{}, e
		}
		v.Lines = append(v.Lines, RFQLine{ID: lineID, ProductID: x.ProductID, Quantity: x.Quantity})
	}
	return s.repository.CreateRFQ(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionRFQ(ctx context.Context, c RFQTransitionCommand) (RFQ, error) {
	c.Scope = c.Scope.Normalize()
	c.RFQID = identity.NormalizeClaim(c.RFQID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.RFQID) || c.ActorID == "" || (c.Status != RFQSubmitted && c.Status != RFQApproved && c.Status != RFQCancelled) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return RFQ{}, ErrInvalidCommand
	}
	return s.repository.TransitionRFQ(ctx, c.Scope, c.ActorID, c.RFQID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) CreateQuote(ctx context.Context, c QuoteCommand) (SupplierQuote, error) {
	c.Scope = c.Scope.Normalize()
	c.RFQID = identity.NormalizeClaim(c.RFQID)
	c.SupplierID = identity.NormalizeClaim(c.SupplierID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reference = strings.ToUpper(strings.TrimSpace(c.Reference))
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.RFQID) || !identity.IsUUID(c.SupplierID) || c.ActorID == "" || len(c.Reference) < 2 || len(c.Currency) != 3 || c.DeliveryDays < 0 || c.DeliveryDays > 3650 || c.PaymentTermsDays < 0 || c.PaymentTermsDays > 3650 || c.ValidUntil.Before(s.clock.Now()) || len(c.Lines) == 0 || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return SupplierQuote{}, ErrInvalidCommand
	}
	id, e := s.ids.New()
	if e != nil {
		return SupplierQuote{}, e
	}
	v := SupplierQuote{ID: id, Scope: c.Scope, RFQID: c.RFQID, SupplierID: c.SupplierID, Reference: c.Reference, Status: QuoteDraft, Currency: c.Currency, DeliveryDays: c.DeliveryDays, PaymentTermsDays: c.PaymentTermsDays, ValidUntil: c.ValidUntil.UTC(), Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: s.clock.Now().UTC(), Lines: []QuoteLine{}}
	seen := map[string]bool{}
	for _, x := range c.Lines {
		x.ProductID = identity.NormalizeClaim(x.ProductID)
		if !identity.IsUUID(x.ProductID) || x.Quantity <= 0 || x.UnitPriceMinor <= 0 || !wire.IsSafeInteger(x.Quantity) || !wire.IsSafeInteger(x.UnitPriceMinor) || seen[x.ProductID] {
			return SupplierQuote{}, ErrInvalidCommand
		}
		seen[x.ProductID] = true
		amount, e := safeMultiply(x.Quantity, x.UnitPriceMinor)
		if e != nil {
			return SupplierQuote{}, e
		}
		lineID, e := s.ids.New()
		if e != nil {
			return SupplierQuote{}, e
		}
		v.Lines = append(v.Lines, QuoteLine{ID: lineID, ProductID: x.ProductID, Quantity: x.Quantity, UnitPriceMinor: x.UnitPriceMinor, AmountMinor: amount})
		v.TotalMinor += amount
	}
	return s.repository.CreateSupplierQuote(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) SubmitQuote(ctx context.Context, c QuoteSubmitCommand) (SupplierQuote, error) {
	c.Scope = c.Scope.Normalize()
	c.QuoteID = identity.NormalizeClaim(c.QuoteID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.QuoteID) || c.ActorID == "" || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return SupplierQuote{}, ErrInvalidCommand
	}
	return s.repository.SubmitSupplierQuote(ctx, c.Scope, c.ActorID, c.QuoteID, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) Comparison(ctx context.Context, scope tenancy.Scope, actor, rfq string) (Comparison, error) {
	return s.repository.RFQComparison(ctx, scope.Normalize(), identity.NormalizeClaim(actor), identity.NormalizeClaim(rfq))
}
func (s *Service) Award(ctx context.Context, c AwardCommand) (Award, error) {
	c.Scope = c.Scope.Normalize()
	c.RFQID = identity.NormalizeClaim(c.RFQID)
	c.QuoteID = identity.NormalizeClaim(c.QuoteID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.RFQID) || !identity.IsUUID(c.QuoteID) || c.ActorID == "" || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Award{}, ErrInvalidCommand
	}
	awardID, e := s.ids.New()
	if e != nil {
		return Award{}, e
	}
	poID, e := s.ids.New()
	if e != nil {
		return Award{}, e
	}
	v := Award{ID: awardID, Scope: c.Scope, RFQID: c.RFQID, QuoteID: c.QuoteID, PurchaseOrderID: poID, Reason: c.Reason, SelectedBy: c.ActorID, SelectedAt: s.clock.Now().UTC()}
	return s.repository.AwardRFQ(ctx, v, c.IdempotencyKey, hash(c))
}
func normalizeSupplier(v *SupplierData) {
	v.Code = strings.ToUpper(strings.TrimSpace(v.Code))
	v.Name = strings.TrimSpace(v.Name)
	v.TaxID = strings.ToUpper(strings.TrimSpace(v.TaxID))
	v.Email = strings.ToLower(strings.TrimSpace(v.Email))
	v.Phone = strings.TrimSpace(v.Phone)
}
func validSupplier(v SupplierData) bool {
	return len(v.Code) >= 2 && len(v.Code) <= 40 && len(v.Name) >= 3 && len(v.Name) <= 160 && v.PaymentTermsDays >= 0 && v.PaymentTermsDays <= 365
}
func normalizeProduct(v *ProductData) {
	v.SKU = strings.ToUpper(strings.TrimSpace(v.SKU))
	v.Name = strings.TrimSpace(v.Name)
	v.BaseUnitCode = strings.ToUpper(strings.TrimSpace(v.BaseUnitCode))
	v.Currency = strings.ToUpper(strings.TrimSpace(v.Currency))
	v.TaxCode = strings.ToUpper(strings.TrimSpace(v.TaxCode))
	v.RevenueAccountID = strings.TrimSpace(v.RevenueAccountID)
	v.COGSAccountID = strings.TrimSpace(v.COGSAccountID)
	v.InventoryAccountID = strings.TrimSpace(v.InventoryAccountID)
}
func validProduct(v ProductData) bool {
	return len(v.SKU) >= 2 && len(v.Name) >= 2 && len(v.BaseUnitCode) >= 1 && len(v.Currency) == 3 && v.ListPriceMinor > 0 && v.StandardCostMinor >= 0 && wire.IsSafeInteger(v.ListPriceMinor) && wire.IsSafeInteger(v.StandardCostMinor) && v.TaxCode != "" && v.RevenueAccountID != "" && v.COGSAccountID != "" && v.InventoryAccountID != ""
}
func validReason(v string) bool { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdem(v string) bool   { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func hash(v any) string {
	b, _ := json.Marshal(v)
	x := sha256.Sum256(b)
	return hex.EncodeToString(x[:])
}
func safeMultiply(a, b int64) (int64, error) {
	if a != 0 && b > wire.MaxSafeInteger/a {
		return 0, ErrInvalidCommand
	}
	v := a * b
	if !wire.IsSafeInteger(v) {
		return 0, ErrInvalidCommand
	}
	return v, nil
}
func purchaseOrderNumber(id string) string {
	return fmt.Sprintf("PO-%s", strings.ToUpper(strings.ReplaceAll(id, "-", "")[:12]))
}
