package readmodel

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type WorkingContext struct {
	ActorID           string   `json:"actor_id"`
	TenantID          string   `json:"tenant_id"`
	CompanyID         string   `json:"company_id"`
	CompanyName       string   `json:"company_name"`
	BranchID          string   `json:"branch_id"`
	BranchName        string   `json:"branch_name"`
	WarehouseID       string   `json:"warehouse_id"`
	WarehouseName     string   `json:"warehouse_name"`
	Currency          string   `json:"currency"`
	Locale            string   `json:"locale"`
	TimeZone          string   `json:"timezone"`
	Permissions       []string `json:"permissions"`
	MasterDataVersion int64    `json:"master_data_version"`
	PriceVersion      int64    `json:"price_version"`
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
	Items      []CustomerSummary `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}
type ProductPage struct {
	Items      []ProductSummary `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}
type SalePage struct {
	Items      []sales.Sale `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

type ListOptions struct {
	Query          string
	CreditEligible *bool
	AfterID        string
	Limit          int
}

type Repository interface {
	WorkingContext(ctx context.Context, scope tenancy.Scope, actorID string) (WorkingContext, error)
	ListCustomers(ctx context.Context, scope tenancy.Scope, actorID string, options ListOptions) ([]CustomerSummary, error)
	ListProducts(ctx context.Context, scope tenancy.Scope, actorID string, options ListOptions) ([]ProductSummary, error)
	ListSales(ctx context.Context, scope tenancy.Scope, actorID string, options ListOptions) ([]sales.Sale, error)
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
	return value, nil
}

func (s *Service) Customers(ctx context.Context, scope tenancy.Scope, actorID, query, cursor string, creditEligible *bool, pageSize int) (CustomerPage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" {
		return CustomerPage{}, sales.ErrInvalidCommand
	}
	options, err := listOptions(query, cursor, creditEligible, pageSize)
	if err != nil {
		return CustomerPage{}, err
	}
	items, err := s.repository.ListCustomers(ctx, scope, actorID, options)
	if err != nil {
		return CustomerPage{}, err
	}
	for _, item := range items {
		if !wire.IsSafeInteger(item.CreditLimitMinor) || !wire.IsSafeInteger(item.CurrentExposureMinor) || !wire.IsSafeInteger(item.AvailableCreditMinor) {
			return CustomerPage{}, wire.ErrUnsafeInteger
		}
	}
	page, next := trim(items, options.Limit)
	return CustomerPage{Items: page, NextCursor: next}, nil
}

func (s *Service) Products(ctx context.Context, scope tenancy.Scope, actorID, query, cursor string, pageSize int) (ProductPage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" {
		return ProductPage{}, sales.ErrInvalidCommand
	}
	options, err := listOptions(query, cursor, nil, pageSize)
	if err != nil {
		return ProductPage{}, err
	}
	items, err := s.repository.ListProducts(ctx, scope, actorID, options)
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
	return ProductPage{Items: page, NextCursor: next}, nil
}

func (s *Service) Sales(ctx context.Context, scope tenancy.Scope, actorID, cursor string, pageSize int) (SalePage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" {
		return SalePage{}, sales.ErrInvalidCommand
	}
	options, err := listOptions("", cursor, nil, pageSize)
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

func listOptions(query, cursor string, eligible *bool, pageSize int) (ListOptions, error) {
	query = strings.TrimSpace(query)
	if len(query) > 120 || pageSize < 1 || pageSize > 200 {
		return ListOptions{}, sales.ErrInvalidCommand
	}
	after, err := decodeCursor(cursor)
	if err != nil {
		return ListOptions{}, sales.ErrInvalidCommand
	}
	return ListOptions{Query: query, CreditEligible: eligible, AfterID: after, Limit: pageSize}, nil
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
