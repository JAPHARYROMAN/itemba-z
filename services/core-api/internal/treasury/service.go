package treasury

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
	ListFacilities(context.Context, tenancy.Scope, string) (Page, error)
	CreateFacility(context.Context, Facility, string, string) (Facility, error)
	TransitionFacility(context.Context, tenancy.Scope, string, string, Status, string, string, string, time.Time) (Facility, error)
	PostTransaction(context.Context, tenancy.Scope, string, string, Transaction, string, string) (Facility, error)
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || ids == nil || c == nil {
		return nil, errors.New("treasury repository, id generator, and clock are required")
	}
	return &Service{repository: r, ids: ids, clock: c}, nil
}

type CreateFacilityCommand struct {
	Scope                                                                                 tenancy.Scope
	Reference, Lender                                                                     string
	Type                                                                                  FacilityType
	Currency                                                                              string
	LimitMinor, AnnualInterestBasisPoints                                                 int64
	StartDate, MaturityDate                                                               time.Time
	BankAccountID, PrincipalAccountID, InterestExpenseAccountID, AccruedInterestAccountID string
	Reason, ActorID, IdempotencyKey                                                       string
}
type TransitionCommand struct {
	Scope                           tenancy.Scope
	FacilityID                      string
	Status                          Status
	Reason, ActorID, IdempotencyKey string
}
type PostTransactionCommand struct {
	Scope                           tenancy.Scope
	FacilityID                      string
	Type                            TransactionType
	AmountMinor                     int64
	OccurredAt                      time.Time
	Reason, ActorID, IdempotencyKey string
}

func (s *Service) Facilities(ctx context.Context, scope tenancy.Scope, actor string) (Page, error) {
	return s.repository.ListFacilities(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}

func (s *Service) CreateFacility(ctx context.Context, c CreateFacilityCommand) (Facility, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reference = strings.ToUpper(strings.TrimSpace(c.Reference))
	c.Lender = strings.TrimSpace(c.Lender)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	accounts := []*string{&c.BankAccountID, &c.PrincipalAccountID, &c.InterestExpenseAccountID, &c.AccruedInterestAccountID}
	for _, v := range accounts {
		*v = identity.NormalizeClaim(*v)
		if !validAccount(*v) {
			return Facility{}, ErrInvalidCommand
		}
	}
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Reference) < 2 || len(c.Reference) > 60 || len(c.Lender) < 3 || len(c.Lender) > 160 || (c.Type != TermLoan && c.Type != Overdraft) || len(c.Currency) != 3 || c.LimitMinor <= 0 || !wire.IsSafeInteger(c.LimitMinor) || c.AnnualInterestBasisPoints < 0 || c.AnnualInterestBasisPoints > 100000 || c.StartDate.IsZero() || !c.MaturityDate.After(c.StartDate) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Facility{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Facility{}, err
	}
	now := s.clock.Now().UTC()
	f := Facility{ID: id, Scope: c.Scope, Reference: c.Reference, Lender: c.Lender, Type: c.Type, Status: Draft, Currency: c.Currency, LimitMinor: c.LimitMinor, AnnualInterestBasisPoints: c.AnnualInterestBasisPoints, StartDate: date(c.StartDate), MaturityDate: date(c.MaturityDate), BankAccountID: c.BankAccountID, PrincipalAccountID: c.PrincipalAccountID, InterestExpenseAccountID: c.InterestExpenseAccountID, AccruedInterestAccountID: c.AccruedInterestAccountID, AvailableMinor: c.LimitMinor, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: now, Transactions: []Transaction{}}
	return s.repository.CreateFacility(ctx, f, c.IdempotencyKey, hash(c))
}

func (s *Service) TransitionFacility(ctx context.Context, c TransitionCommand) (Facility, error) {
	c.Scope = c.Scope.Normalize()
	c.FacilityID = identity.NormalizeClaim(c.FacilityID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.FacilityID) || c.ActorID == "" || (c.Status != Submitted && c.Status != Active && c.Status != Rejected && c.Status != Closed) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Facility{}, ErrInvalidCommand
	}
	return s.repository.TransitionFacility(ctx, c.Scope, c.ActorID, c.FacilityID, c.Status, c.Reason, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}

func (s *Service) PostTransaction(ctx context.Context, c PostTransactionCommand) (Facility, error) {
	c.Scope = c.Scope.Normalize()
	c.FacilityID = identity.NormalizeClaim(c.FacilityID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	c.OccurredAt = c.OccurredAt.UTC()
	validType := c.Type == Drawdown || c.Type == PrincipalRepayment || c.Type == InterestAccrual || c.Type == InterestPayment
	if c.Scope.Validate() != nil || !identity.IsUUID(c.FacilityID) || c.ActorID == "" || !validType || c.AmountMinor <= 0 || !wire.IsSafeInteger(c.AmountMinor) || c.OccurredAt.IsZero() || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Facility{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Facility{}, err
	}
	journal, err := s.ids.New()
	if err != nil {
		return Facility{}, err
	}
	t := Transaction{ID: id, FacilityID: c.FacilityID, Type: c.Type, AmountMinor: c.AmountMinor, OccurredAt: c.OccurredAt, Reason: c.Reason, JournalID: journal, PostedBy: c.ActorID, PostedAt: s.clock.Now().UTC()}
	return s.repository.PostTransaction(ctx, c.Scope, c.ActorID, c.FacilityID, t, c.IdempotencyKey, hash(c))
}

func date(v time.Time) time.Time {
	y, m, d := v.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func validReason(v string) bool  { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdem(v string) bool    { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func validAccount(v string) bool { n := len(v); return n >= 2 && n <= 64 }
func hash(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
