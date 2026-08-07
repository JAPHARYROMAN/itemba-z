package groupfinance

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
	ListIntercompany(context.Context, tenancy.Scope, string) (Page, error)
	CreateIntercompany(context.Context, Transaction, string, string) (Transaction, error)
	TransitionIntercompany(context.Context, tenancy.Scope, string, string, Status, string, string, string, string, string, time.Time) (Transaction, error)
	Consolidation(context.Context, tenancy.Scope, string, time.Time) (Consolidation, error)
}
type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(r Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if r == nil || ids == nil || c == nil {
		return nil, errors.New("group finance repository, id generator, and clock are required")
	}
	return &Service{repository: r, ids: ids, clock: c}, nil
}

type CreateCommand struct {
	Scope                                                                                                                                 tenancy.Scope
	CounterpartyCompanyID, Reference                                                                                                      string
	Type                                                                                                                                  TransactionType
	Currency                                                                                                                              string
	AmountMinor                                                                                                                           int64
	OccurredAt                                                                                                                            time.Time
	SourceDebitAccountID, SourceCreditAccountID, CounterpartyDebitAccountID, CounterpartyCreditAccountID, Reason, ActorID, IdempotencyKey string
}
type TransitionCommand struct {
	Scope                           tenancy.Scope
	TransactionID                   string
	Status                          Status
	Reason, ActorID, IdempotencyKey string
}

func (s *Service) Transactions(ctx context.Context, scope tenancy.Scope, actor string) (Page, error) {
	return s.repository.ListIntercompany(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) Consolidated(ctx context.Context, scope tenancy.Scope, actor string, asOf time.Time) (Consolidation, error) {
	scope = scope.Normalize()
	actor = identity.NormalizeClaim(actor)
	if scope.Validate() != nil || actor == "" || asOf.IsZero() {
		return Consolidation{}, ErrInvalidCommand
	}
	return s.repository.Consolidation(ctx, scope, actor, asOf.UTC())
}
func (s *Service) Create(ctx context.Context, c CreateCommand) (Transaction, error) {
	c.Scope = c.Scope.Normalize()
	c.CounterpartyCompanyID = identity.NormalizeClaim(c.CounterpartyCompanyID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reference = strings.ToUpper(strings.TrimSpace(c.Reference))
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	c.OccurredAt = c.OccurredAt.UTC()
	for _, v := range []*string{&c.SourceDebitAccountID, &c.SourceCreditAccountID, &c.CounterpartyDebitAccountID, &c.CounterpartyCreditAccountID} {
		*v = identity.NormalizeClaim(*v)
		if !validAccount(*v) {
			return Transaction{}, ErrInvalidCommand
		}
	}
	if c.Scope.Validate() != nil || !identity.IsUUID(c.CounterpartyCompanyID) || c.CounterpartyCompanyID == c.Scope.CompanyID || c.ActorID == "" || len(c.Reference) < 2 || len(c.Reference) > 60 || (c.Type != CashTransfer && c.Type != CostAllocation) || len(c.Currency) != 3 || c.AmountMinor <= 0 || !wire.IsSafeInteger(c.AmountMinor) || c.OccurredAt.IsZero() || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Transaction{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Transaction{}, err
	}
	now := s.clock.Now().UTC()
	v := Transaction{ID: id, Scope: c.Scope, CounterpartyCompanyID: c.CounterpartyCompanyID, Reference: c.Reference, Type: c.Type, Status: Draft, Currency: c.Currency, AmountMinor: c.AmountMinor, OccurredAt: c.OccurredAt, SourceDebitAccountID: c.SourceDebitAccountID, SourceCreditAccountID: c.SourceCreditAccountID, CounterpartyDebitAccountID: c.CounterpartyDebitAccountID, CounterpartyCreditAccountID: c.CounterpartyCreditAccountID, Reason: c.Reason, CreatedBy: c.ActorID, CreatedAt: now}
	return s.repository.CreateIntercompany(ctx, v, c.IdempotencyKey, hash(c))
}
func (s *Service) Transition(ctx context.Context, c TransitionCommand) (Transaction, error) {
	c.Scope = c.Scope.Normalize()
	c.TransactionID = identity.NormalizeClaim(c.TransactionID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.TransactionID) || c.ActorID == "" || (c.Status != Submitted && c.Status != SourceApproved && c.Status != Posted && c.Status != Rejected) || !validReason(c.Reason) || !validIdem(c.IdempotencyKey) {
		return Transaction{}, ErrInvalidCommand
	}
	sourceJournal, counterJournal := "", ""
	if c.Status == Posted {
		var err error
		sourceJournal, err = s.ids.New()
		if err != nil {
			return Transaction{}, err
		}
		counterJournal, err = s.ids.New()
		if err != nil {
			return Transaction{}, err
		}
	}
	return s.repository.TransitionIntercompany(ctx, c.Scope, c.ActorID, c.TransactionID, c.Status, c.Reason, sourceJournal, counterJournal, c.IdempotencyKey, hash(c), s.clock.Now().UTC())
}
func validReason(v string) bool  { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdem(v string) bool    { n := len(strings.TrimSpace(v)); return n >= 16 && n <= 128 }
func validAccount(v string) bool { return len(v) >= 2 && len(v) <= 64 }
func hash(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
