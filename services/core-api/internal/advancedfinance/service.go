package advancedfinance

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
	ListBudgets(context.Context, tenancy.Scope, string) (Page[Budget], error)
	CreateBudget(context.Context, Budget, string, string) (Budget, error)
	TransitionBudget(context.Context, tenancy.Scope, string, string, Status, string, string, string, time.Time) (Budget, error)
	BudgetActual(context.Context, tenancy.Scope, string, string, time.Time) (BudgetActual, error)
	ListAssets(context.Context, tenancy.Scope, string) (Page[Asset], error)
	CreateAsset(context.Context, Asset, string, string) (Asset, error)
	TransitionAsset(context.Context, tenancy.Scope, string, string, Status, string, string, string, string, time.Time) (Asset, error)
	PostDepreciation(context.Context, tenancy.Scope, string, string, string, string, time.Time, string, string, time.Time) (Depreciation, error)
	DisposeAsset(context.Context, tenancy.Scope, string, string, int64, string, string, string, string, string, time.Time) (Asset, error)
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || c == nil {
		return nil, errors.New("advanced finance repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: c}, nil
}

type CreateBudgetCommand struct {
	Scope                                     tenancy.Scope
	Name                                      string
	FiscalYear                                int
	Currency, Reason, ActorID, IdempotencyKey string
	Lines                                     []BudgetLine
}
type TransitionCommand struct {
	Scope                           tenancy.Scope
	ID                              string
	Status                          Status
	Reason, ActorID, IdempotencyKey string
}
type CreateAssetCommand struct {
	Scope                                                                                                                                                                                        tenancy.Scope
	Code, Name, Category, Currency                                                                                                                                                               string
	AcquiredAt                                                                                                                                                                                   time.Time
	CostMinor, ResidualMinor                                                                                                                                                                     int64
	UsefulLifeMonths                                                                                                                                                                             int
	AssetAccountID, AccumulatedDepreciationAccountID, DepreciationExpenseAccountID, CapitalizationOffsetAccountID, DisposalGainAccountID, DisposalLossAccountID, Reason, ActorID, IdempotencyKey string
}
type DepreciateCommand struct {
	Scope                           tenancy.Scope
	AssetID                         string
	Period                          time.Time
	Reason, ActorID, IdempotencyKey string
}
type DisposeCommand struct {
	Scope                                              tenancy.Scope
	AssetID                                            string
	ProceedsMinor                                      int64
	ProceedsAccountID, Reason, ActorID, IdempotencyKey string
}

func (s *Service) Budgets(ctx context.Context, scope tenancy.Scope, actor string) (Page[Budget], error) {
	return s.repository.ListBudgets(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) Assets(ctx context.Context, scope tenancy.Scope, actor string) (Page[Asset], error) {
	return s.repository.ListAssets(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) BudgetActual(ctx context.Context, scope tenancy.Scope, actor, id string) (BudgetActual, error) {
	return s.repository.BudgetActual(ctx, scope.Normalize(), identity.NormalizeClaim(actor), identity.NormalizeClaim(id), s.clock.Now().UTC())
}

func (s *Service) CreateBudget(ctx context.Context, c CreateBudgetCommand) (Budget, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Name = strings.TrimSpace(c.Name)
	c.Reason = strings.TrimSpace(c.Reason)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Name) < 3 || len(c.Name) > 120 || c.FiscalYear < 2000 || c.FiscalYear > 2200 || len(c.Currency) != 3 || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) || len(c.Lines) == 0 || len(c.Lines) > 5000 {
		return Budget{}, ErrInvalidCommand
	}
	seen := map[string]bool{}
	normalized := make([]BudgetLine, 0, len(c.Lines))
	for _, l := range c.Lines {
		l.AccountID = identity.NormalizeClaim(l.AccountID)
		l.Month = month(l.Month)
		key := l.AccountID + ":" + l.Month.Format("2006-01")
		if !validAccountID(l.AccountID) || l.Month.Year() != c.FiscalYear || l.AmountMinor < 0 || !wire.IsSafeInteger(l.AmountMinor) || seen[key] {
			return Budget{}, ErrInvalidCommand
		}
		seen[key] = true
		normalized = append(normalized, l)
	}
	id, err := s.ids.New()
	if err != nil {
		return Budget{}, err
	}
	now := s.clock.Now().UTC()
	return s.repository.CreateBudget(ctx, Budget{ID: id, Scope: c.Scope, Name: c.Name, FiscalYear: c.FiscalYear, Currency: c.Currency, Status: Draft, Reason: c.Reason, Lines: normalized, CreatedBy: c.ActorID, CreatedAt: now}, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionBudget(ctx context.Context, c TransitionCommand) (Budget, error) {
	c = normalizeTransition(c)
	if !validTransitionCommand(c, Submitted, Approved, Rejected) {
		return Budget{}, ErrInvalidCommand
	}
	return s.repository.TransitionBudget(ctx, c.Scope, c.ActorID, c.ID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}

func (s *Service) CreateAsset(ctx context.Context, c CreateAssetCommand) (Asset, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	c.Name = strings.TrimSpace(c.Name)
	c.Category = strings.TrimSpace(c.Category)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	ids := []*string{&c.AssetAccountID, &c.AccumulatedDepreciationAccountID, &c.DepreciationExpenseAccountID, &c.CapitalizationOffsetAccountID, &c.DisposalGainAccountID, &c.DisposalLossAccountID}
	for _, v := range ids {
		*v = identity.NormalizeClaim(*v)
		if !validAccountID(*v) {
			return Asset{}, ErrInvalidCommand
		}
	}
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Code) < 2 || len(c.Code) > 40 || len(c.Name) < 3 || len(c.Name) > 160 || len(c.Category) < 2 || len(c.Currency) != 3 || c.AcquiredAt.IsZero() || c.CostMinor <= 0 || c.ResidualMinor < 0 || c.ResidualMinor >= c.CostMinor || c.UsefulLifeMonths < 1 || c.UsefulLifeMonths > 1200 || !wire.IsSafeInteger(c.CostMinor) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Asset{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Asset{}, err
	}
	now := s.clock.Now().UTC()
	a := Asset{ID: id, Scope: c.Scope, Code: c.Code, Name: c.Name, Category: c.Category, Status: Draft, Currency: c.Currency, AcquiredAt: c.AcquiredAt.UTC(), CostMinor: c.CostMinor, ResidualMinor: c.ResidualMinor, UsefulLifeMonths: c.UsefulLifeMonths, NetBookValueMinor: c.CostMinor, AssetAccountID: c.AssetAccountID, AccumulatedDepreciationAccountID: c.AccumulatedDepreciationAccountID, DepreciationExpenseAccountID: c.DepreciationExpenseAccountID, CapitalizationOffsetAccountID: c.CapitalizationOffsetAccountID, DisposalGainAccountID: c.DisposalGainAccountID, DisposalLossAccountID: c.DisposalLossAccountID, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: now}
	return s.repository.CreateAsset(ctx, a, c.IdempotencyKey, hash(c))
}
func (s *Service) TransitionAsset(ctx context.Context, c TransitionCommand) (Asset, error) {
	c = normalizeTransition(c)
	if !validTransitionCommand(c, Submitted, Active, Rejected) {
		return Asset{}, ErrInvalidCommand
	}
	journal := ""
	if c.Status == Active {
		var err error
		journal, err = s.ids.New()
		if err != nil {
			return Asset{}, err
		}
	}
	return s.repository.TransitionAsset(ctx, c.Scope, c.ActorID, c.ID, c.Status, c.Reason, journal, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) Depreciate(ctx context.Context, c DepreciateCommand) (Depreciation, error) {
	c.Scope = c.Scope.Normalize()
	c.AssetID = identity.NormalizeClaim(c.AssetID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	c.Period = month(c.Period)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.AssetID) || c.ActorID == "" || c.Period.IsZero() || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Depreciation{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Depreciation{}, err
	}
	journal, err := s.ids.New()
	if err != nil {
		return Depreciation{}, err
	}
	return s.repository.PostDepreciation(ctx, c.Scope, c.ActorID, c.AssetID, id, journal, c.Period, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func (s *Service) Dispose(ctx context.Context, c DisposeCommand) (Asset, error) {
	c.Scope = c.Scope.Normalize()
	c.AssetID = identity.NormalizeClaim(c.AssetID)
	c.ProceedsAccountID = identity.NormalizeClaim(c.ProceedsAccountID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.AssetID) || !validAccountID(c.ProceedsAccountID) || c.ProceedsMinor < 0 || !wire.IsSafeInteger(c.ProceedsMinor) || c.ActorID == "" || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Asset{}, ErrInvalidCommand
	}
	journal, err := s.ids.New()
	if err != nil {
		return Asset{}, err
	}
	return s.repository.DisposeAsset(ctx, c.Scope, c.ActorID, c.AssetID, c.ProceedsMinor, c.ProceedsAccountID, c.Reason, journal, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}

func normalizeTransition(c TransitionCommand) TransitionCommand {
	c.Scope = c.Scope.Normalize()
	c.ID = identity.NormalizeClaim(c.ID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	return c
}
func validTransitionCommand(c TransitionCommand, allowed ...Status) bool {
	ok := false
	for _, s := range allowed {
		ok = ok || c.Status == s
	}
	return ok && c.Scope.Validate() == nil && identity.IsUUID(c.ID) && c.ActorID != "" && validReason(c.Reason) && validIdem(c.IdempotencyKey)
}
func month(v time.Time) time.Time {
	if v.IsZero() {
		return v
	}
	y, m, _ := v.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}
func validReason(v string) bool    { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdem(v string) bool      { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func validAccountID(v string) bool { n := len(strings.TrimSpace(v)); return n >= 2 && n <= 64 }
func hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
