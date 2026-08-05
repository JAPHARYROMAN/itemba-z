package financialops

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type AccountType string

const (
	AccountAsset        AccountType = "ASSET"
	AccountLiability    AccountType = "LIABILITY"
	AccountEquity       AccountType = "EQUITY"
	AccountRevenue      AccountType = "REVENUE"
	AccountExpense      AccountType = "EXPENSE"
	AccountUnclassified AccountType = "UNCLASSIFIED"
)

type GovernanceStatus string

const (
	GovernanceSubmitted GovernanceStatus = "SUBMITTED"
	GovernanceActive    GovernanceStatus = "ACTIVE"
	GovernanceRejected  GovernanceStatus = "REJECTED"
	GovernanceInactive  GovernanceStatus = "INACTIVE"
)

type GLAccount struct {
	RecordID           string           `json:"record_id"`
	ID                 string           `json:"id"`
	TenantID           string           `json:"tenant_id"`
	CompanyID          string           `json:"company_id"`
	Code               string           `json:"code"`
	Name               string           `json:"name"`
	Type               AccountType      `json:"type"`
	ParentAccountID    string           `json:"parent_account_id,omitempty"`
	ControlAccount     bool             `json:"control_account"`
	AllowManualPosting bool             `json:"allow_manual_posting"`
	Status             GovernanceStatus `json:"status"`
	CreatedBy          string           `json:"created_by,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	ApprovedBy         string           `json:"approved_by,omitempty"`
	ApprovedAt         *time.Time       `json:"approved_at,omitempty"`
}
type GLAccountPage struct {
	Items []GLAccount `json:"items"`
}
type MappingKey string

const (
	MapSalesReceivable     MappingKey = "SALES_RECEIVABLE"
	MapSalesTaxPayable     MappingKey = "SALES_TAX_PAYABLE"
	MapPaymentCash         MappingKey = "PAYMENT_CASH"
	MapPaymentMobileMoney  MappingKey = "PAYMENT_MOBILE_MONEY"
	MapPaymentBankCard     MappingKey = "PAYMENT_BANK_CARD"
	MapPaymentBankTransfer MappingKey = "PAYMENT_BANK_TRANSFER"
	MapProcurementGRNI     MappingKey = "PROCUREMENT_GRNI"
	MapProcurementPayable  MappingKey = "PROCUREMENT_PAYABLE"
	MapInventoryAdjustment MappingKey = "INVENTORY_ADJUSTMENT"
	MapStockInTransit      MappingKey = "STOCK_IN_TRANSIT"
)

type PostingMapping struct {
	ID            string           `json:"id"`
	TenantID      string           `json:"tenant_id"`
	CompanyID     string           `json:"company_id"`
	Key           MappingKey       `json:"key"`
	AccountID     string           `json:"account_id"`
	EffectiveFrom time.Time        `json:"effective_from"`
	Status        GovernanceStatus `json:"status"`
	Reason        string           `json:"reason"`
	CreatedBy     string           `json:"created_by,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	ApprovedBy    string           `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time       `json:"approved_at,omitempty"`
}
type PostingMappingPage struct {
	Items []PostingMapping `json:"items"`
}
type CreateAccountCommand struct {
	Scope                   tenancy.Scope
	Code                    string
	Name                    string
	Type                    AccountType
	ParentAccountID         string
	ControlAccount          bool
	AllowManualPosting      bool
	ActorID, IdempotencyKey string
}
type DecideAccountCommand struct {
	Scope                           tenancy.Scope
	AccountID                       string
	Status                          GovernanceStatus
	Reason, ActorID, IdempotencyKey string
}
type CreateMappingCommand struct {
	Scope                           tenancy.Scope
	Key                             MappingKey
	AccountID                       string
	EffectiveFrom                   time.Time
	Reason, ActorID, IdempotencyKey string
}
type DecideMappingCommand struct {
	Scope                           tenancy.Scope
	MappingID                       string
	Status                          GovernanceStatus
	Reason, ActorID, IdempotencyKey string
}

var ErrAccountGovernance = errors.New("chart-of-accounts governance rule failed")
var accountCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

func validAccountType(v AccountType) bool {
	return v == AccountAsset || v == AccountLiability || v == AccountEquity || v == AccountRevenue || v == AccountExpense
}
func validMappingKey(v MappingKey) bool {
	switch v {
	case MapSalesReceivable, MapSalesTaxPayable, MapPaymentCash, MapPaymentMobileMoney, MapPaymentBankCard, MapPaymentBankTransfer, MapProcurementGRNI, MapProcurementPayable, MapInventoryAdjustment, MapStockInTransit:
		return true
	}
	return false
}

func (s *Service) Accounts(ctx context.Context, scope tenancy.Scope, actor string) (GLAccountPage, error) {
	return s.repository.ListGLAccounts(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) CreateAccount(ctx context.Context, c CreateAccountCommand) (GLAccount, error) {
	c.Scope = c.Scope.Normalize()
	c.Code = strings.ToLower(strings.TrimSpace(c.Code))
	c.Name = strings.TrimSpace(c.Name)
	c.ParentAccountID = strings.ToLower(strings.TrimSpace(c.ParentAccountID))
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	if c.Scope.Validate() != nil || !accountCodePattern.MatchString(c.Code) || len(c.Name) < 2 || len(c.Name) > 160 || !validAccountType(c.Type) || c.ActorID == "" || !validIdempotency(c.IdempotencyKey) || (c.ControlAccount && c.AllowManualPosting) {
		return GLAccount{}, ErrInvalidCommand
	}
	recordID, err := s.ids.New()
	if err != nil {
		return GLAccount{}, err
	}
	now := s.clock.Now().UTC()
	return s.repository.CreateGLAccount(ctx, GLAccount{RecordID: recordID, ID: c.Code, TenantID: c.Scope.TenantID, CompanyID: c.Scope.CompanyID, Code: c.Code, Name: c.Name, Type: c.Type, ParentAccountID: c.ParentAccountID, ControlAccount: c.ControlAccount, AllowManualPosting: c.AllowManualPosting, Status: GovernanceSubmitted, CreatedBy: c.ActorID, CreatedAt: now}, c.Scope, c.IdempotencyKey, commandHash(c))
}
func (s *Service) DecideAccount(ctx context.Context, c DecideAccountCommand) (GLAccount, error) {
	c.Scope = c.Scope.Normalize()
	c.AccountID = strings.ToLower(strings.TrimSpace(c.AccountID))
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !accountCodePattern.MatchString(c.AccountID) || c.ActorID == "" || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) || (c.Status != GovernanceActive && c.Status != GovernanceRejected) {
		return GLAccount{}, ErrInvalidCommand
	}
	return s.repository.DecideGLAccount(ctx, c.Scope, c.ActorID, c.AccountID, c.Status, c.Reason, c.IdempotencyKey, commandHash(c), s.clock.Now().UTC())
}
func (s *Service) Mappings(ctx context.Context, scope tenancy.Scope, actor string) (PostingMappingPage, error) {
	return s.repository.ListPostingMappings(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) CreateMapping(ctx context.Context, c CreateMappingCommand) (PostingMapping, error) {
	c.Scope = c.Scope.Normalize()
	c.AccountID = strings.ToLower(strings.TrimSpace(c.AccountID))
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !validMappingKey(c.Key) || !accountCodePattern.MatchString(c.AccountID) || c.EffectiveFrom.IsZero() || c.ActorID == "" || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) {
		return PostingMapping{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return PostingMapping{}, err
	}
	now := s.clock.Now().UTC()
	return s.repository.CreatePostingMapping(ctx, PostingMapping{ID: id, TenantID: c.Scope.TenantID, CompanyID: c.Scope.CompanyID, Key: c.Key, AccountID: c.AccountID, EffectiveFrom: c.EffectiveFrom.UTC(), Status: GovernanceSubmitted, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: now}, c.Scope, c.IdempotencyKey, commandHash(c))
}
func (s *Service) DecideMapping(ctx context.Context, c DecideMappingCommand) (PostingMapping, error) {
	c.Scope = c.Scope.Normalize()
	c.MappingID = identity.NormalizeClaim(c.MappingID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.MappingID) || c.ActorID == "" || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) || (c.Status != GovernanceActive && c.Status != GovernanceRejected) {
		return PostingMapping{}, ErrInvalidCommand
	}
	return s.repository.DecidePostingMapping(ctx, c.Scope, c.ActorID, c.MappingID, c.Status, c.Reason, c.IdempotencyKey, commandHash(c), s.clock.Now().UTC())
}
