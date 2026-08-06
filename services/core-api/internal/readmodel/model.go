package readmodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type WorkingContext struct {
	ActorID              string   `json:"actor_id"`
	TenantID             string   `json:"tenant_id"`
	CompanyID            string   `json:"company_id"`
	CompanyName          string   `json:"company_name"`
	BranchID             string   `json:"branch_id"`
	BranchName           string   `json:"branch_name"`
	WarehouseID          string   `json:"warehouse_id"`
	WarehouseName        string   `json:"warehouse_name"`
	Currency             string   `json:"currency"`
	Locale               string   `json:"locale"`
	TimeZone             string   `json:"timezone"`
	Permissions          []string `json:"permissions"`
	MasterDataVersion    int64    `json:"master_data_version"`
	PriceVersion         int64    `json:"price_version"`
	CatalogSnapshotToken string   `json:"catalog_snapshot_token"`
}

type CustomerSummary struct {
	ID                   string `json:"id"`
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	IsGeneralCustomer    bool   `json:"is_general_customer"`
	CreditEnabled        bool   `json:"credit_enabled"`
	CreditLimitMinor     int64  `json:"credit_limit_minor"`
	CurrentExposureMinor int64  `json:"current_exposure_minor"`
	AvailableCreditMinor int64  `json:"available_credit_minor"`
}

type ProductSummary struct {
	ID                string `json:"id"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	Unit              string `json:"unit"`
	Currency          string `json:"currency"`
	UnitPriceMinor    int64  `json:"unit_price_minor"`
	AvailableQuantity int64  `json:"available_quantity"`
	PriceVersion      int64  `json:"price_version"`
	MasterDataVersion int64  `json:"master_data_version"`
	TaxBasisPoints    int64  `json:"tax_basis_points"`
}

type CustomerPage struct {
	Items                []CustomerSummary `json:"items"`
	NextCursor           *string           `json:"next_cursor"`
	CatalogSnapshotToken string            `json:"catalog_snapshot_token"`
	MasterDataVersion    int64             `json:"master_data_version"`
	PriceVersion         int64             `json:"price_version"`
}
type ProductPage struct {
	Items                []ProductSummary `json:"items"`
	NextCursor           *string          `json:"next_cursor"`
	CatalogSnapshotToken string           `json:"catalog_snapshot_token"`
	MasterDataVersion    int64            `json:"master_data_version"`
	PriceVersion         int64            `json:"price_version"`
}
type SalePage struct {
	Items      []sales.Sale `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

type ListOptions struct {
	Query                string
	CreditEligible       *bool
	AfterID              string
	Limit                int
	CatalogSnapshotToken string
}

type AuditRecord struct {
	ID            string          `json:"id"`
	ActorID       string          `json:"actor_id"`
	Action        string          `json:"action"`
	EntityType    string          `json:"entity_type"`
	EntityID      string          `json:"entity_id"`
	CorrelationID string          `json:"correlation_id"`
	CausationID   string          `json:"causation_id"`
	Data          json.RawMessage `json:"data"`
	OccurredAt    time.Time       `json:"occurred_at"`
}

type AuditTrail struct {
	Items []AuditRecord `json:"items"`
}

type CatalogSnapshot struct {
	Token             string
	MasterDataVersion int64
	PriceVersion      int64
}

type Repository interface {
	WorkingContext(ctx context.Context, scope tenancy.Scope, actorID string) (WorkingContext, error)
	ListCustomers(ctx context.Context, scope tenancy.Scope, actorID string, options ListOptions) (CatalogSnapshot, []CustomerSummary, error)
	ListProducts(ctx context.Context, scope tenancy.Scope, actorID string, options ListOptions) (CatalogSnapshot, []ProductSummary, error)
	ListSales(ctx context.Context, scope tenancy.Scope, actorID string, options ListOptions) ([]sales.Sale, error)
	ListAuditEvents(ctx context.Context, scope tenancy.Scope, actorID, entityType, entityID string) ([]AuditRecord, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("read repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) Context(ctx context.Context, scope tenancy.Scope, actorID string) (WorkingContext, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || strings.TrimSpace(actorID) == "" {
		return WorkingContext{}, sales.ErrInvalidCommand
	}
	value, err := s.repository.WorkingContext(ctx, scope, actorID)
	if err != nil {
		return WorkingContext{}, err
	}
	if !wire.IsSafeInteger(value.MasterDataVersion) || !wire.IsSafeInteger(value.PriceVersion) {
		return WorkingContext{}, wire.ErrUnsafeInteger
	}
	if !identity.IsUUID(value.CatalogSnapshotToken) || value.CatalogSnapshotToken == devices.UnacknowledgedCatalogSnapshotToken {
		return WorkingContext{}, sales.ErrPostingConfig
	}
	return value, nil
}

func (s *Service) Customers(ctx context.Context, scope tenancy.Scope, actorID, query, cursor, snapshotToken string, creditEligible *bool, pageSize int) (CustomerPage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" {
		return CustomerPage{}, sales.ErrInvalidCommand
	}
	options, err := listOptions(query, cursor, snapshotToken, creditEligible, pageSize)
	if err != nil {
		return CustomerPage{}, err
	}
	snapshot, items, err := s.repository.ListCustomers(ctx, scope, actorID, options)
	if err != nil {
		return CustomerPage{}, err
	}
	for _, item := range items {
		if !wire.IsSafeInteger(item.CreditLimitMinor) || !wire.IsSafeInteger(item.CurrentExposureMinor) || !wire.IsSafeInteger(item.AvailableCreditMinor) {
			return CustomerPage{}, wire.ErrUnsafeInteger
		}
	}
	page, next := trim(items, options.Limit)
	if err := validateCatalogSnapshot(snapshot); err != nil {
		return CustomerPage{}, err
	}
	return CustomerPage{Items: page, NextCursor: next, CatalogSnapshotToken: snapshot.Token, MasterDataVersion: snapshot.MasterDataVersion, PriceVersion: snapshot.PriceVersion}, nil
}

func (s *Service) Products(ctx context.Context, scope tenancy.Scope, actorID, query, cursor, snapshotToken string, pageSize int) (ProductPage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" {
		return ProductPage{}, sales.ErrInvalidCommand
	}
	options, err := listOptions(query, cursor, snapshotToken, nil, pageSize)
	if err != nil {
		return ProductPage{}, err
	}
	snapshot, items, err := s.repository.ListProducts(ctx, scope, actorID, options)
	if err != nil {
		return ProductPage{}, err
	}
	for _, item := range items {
		if !wire.IsSafeInteger(item.UnitPriceMinor) || !wire.IsSafeInteger(item.AvailableQuantity) ||
			!wire.IsSafeInteger(item.PriceVersion) || !wire.IsSafeInteger(item.MasterDataVersion) {
			return ProductPage{}, wire.ErrUnsafeInteger
		}
	}
	page, next := trim(items, options.Limit)
	if err := validateCatalogSnapshot(snapshot); err != nil {
		return ProductPage{}, err
	}
	return ProductPage{Items: page, NextCursor: next, CatalogSnapshotToken: snapshot.Token, MasterDataVersion: snapshot.MasterDataVersion, PriceVersion: snapshot.PriceVersion}, nil
}

func (s *Service) Sales(ctx context.Context, scope tenancy.Scope, actorID, cursor string, pageSize int) (SalePage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" {
		return SalePage{}, sales.ErrInvalidCommand
	}
	options, err := listOptions("", cursor, "", nil, pageSize)
	if err != nil {
		return SalePage{}, err
	}
	items, err := s.repository.ListSales(ctx, scope, actorID, options)
	if err != nil {
		return SalePage{}, err
	}
	for _, item := range items {
		if err := sales.ValidateWireSafeSale(item); err != nil {
			return SalePage{}, err
		}
	}
	if len(items) <= options.Limit {
		return SalePage{Items: items}, nil
	}
	value := encodeCursor(items[options.Limit-1].ID)
	return SalePage{Items: items[:options.Limit], NextCursor: &value}, nil
}

func (s *Service) Audit(ctx context.Context, scope tenancy.Scope, actorID, entityType, entityID string) (AuditTrail, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	entityType = strings.ToLower(strings.TrimSpace(entityType))
	entityID = strings.ToLower(strings.TrimSpace(entityID))
	if scope.Validate() != nil || actorID == "" || entityType == "" || len(entityType) > 80 || !identity.IsUUID(entityID) {
		return AuditTrail{}, sales.ErrInvalidCommand
	}
	items, err := s.repository.ListAuditEvents(ctx, scope, actorID, entityType, entityID)
	if err != nil {
		return AuditTrail{}, err
	}
	return AuditTrail{Items: items}, nil
}

func listOptions(query, cursor, snapshotToken string, eligible *bool, pageSize int) (ListOptions, error) {
	query = strings.TrimSpace(query)
	if len(query) > 120 || pageSize < 1 || pageSize > 200 {
		return ListOptions{}, sales.ErrInvalidCommand
	}
	after, err := decodeCursor(cursor)
	if err != nil {
		return ListOptions{}, sales.ErrInvalidCommand
	}
	snapshotToken = identity.NormalizeClaim(snapshotToken)
	if snapshotToken != "" && !identity.IsUUID(snapshotToken) {
		return ListOptions{}, sales.ErrInvalidCommand
	}
	return ListOptions{Query: query, CreditEligible: eligible, AfterID: after, Limit: pageSize, CatalogSnapshotToken: snapshotToken}, nil
}

func validateCatalogSnapshot(value CatalogSnapshot) error {
	if !identity.IsUUID(value.Token) || value.Token == devices.UnacknowledgedCatalogSnapshotToken ||
		!wire.IsSafeInteger(value.MasterDataVersion) || !wire.IsSafeInteger(value.PriceVersion) ||
		value.MasterDataVersion < 1 || value.PriceVersion < 1 {
		return sales.ErrPostingConfig
	}
	return nil
}

type identified interface{ identifier() string }

func trim[T interface{ identifier() string }](items []T, limit int) ([]T, *string) {
	if len(items) <= limit {
		return items, nil
	}
	value := encodeCursor(items[limit-1].identifier())
	return items[:limit], &value
}
func (c CustomerSummary) identifier() string { return c.ID }
func (p ProductSummary) identifier() string  { return p.ID }

func encodeCursor(id string) string { return base64.RawURLEncoding.EncodeToString([]byte(id)) }
func decodeCursor(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) == 0 || !identity.IsUUID(string(decoded)) {
		return "", fmt.Errorf("invalid cursor")
	}
	return strings.ToLower(string(decoded)), nil
}
