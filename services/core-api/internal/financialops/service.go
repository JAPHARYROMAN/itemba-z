package financialops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Repository interface {
	ListGLAccounts(context.Context, tenancy.Scope, string) (GLAccountPage, error)
	CreateGLAccount(context.Context, GLAccount, tenancy.Scope, string, string) (GLAccount, error)
	DecideGLAccount(context.Context, tenancy.Scope, string, string, GovernanceStatus, string, string, string, time.Time) (GLAccount, error)
	ListPostingMappings(context.Context, tenancy.Scope, string) (PostingMappingPage, error)
	CreatePostingMapping(context.Context, PostingMapping, tenancy.Scope, string, string) (PostingMapping, error)
	DecidePostingMapping(context.Context, tenancy.Scope, string, string, GovernanceStatus, string, string, string, time.Time) (PostingMapping, error)
	CreateFinancialDocument(context.Context, Document, string, string) (Document, error)
	ListFinancialDocuments(context.Context, tenancy.Scope, string, string, int) (Page, error)
	FinancialDocument(context.Context, tenancy.Scope, string, string) (Document, error)
	TransitionFinancialDocument(context.Context, tenancy.Scope, string, string, Status, string, string, string, string, time.Time) (Document, error)
	ListFiscalPeriods(context.Context, tenancy.Scope, string) ([]FiscalPeriod, error)
	ListFiscalPeriodActions(context.Context, tenancy.Scope, string) (PeriodActionPage, error)
	RequestFiscalPeriodAction(context.Context, tenancy.Scope, string, string, PeriodAction, string, string, string, string, time.Time) (PeriodActionRequest, error)
	ApproveFiscalPeriodAction(context.Context, tenancy.Scope, string, string, string, string, string, time.Time) (PeriodActionRequest, error)
}

type CreateCommand struct {
	Scope                                                                                          tenancy.Scope
	Type                                                                                           DocumentType
	Currency                                                                                       string
	AccountingAt                                                                                   time.Time
	Reason, FromAccountID, ToAccountID, ReversesDocumentID, ActorID, IdempotencyKey, CorrelationID string
	Lines                                                                                          []finance.JournalEntry
}
type TransitionCommand struct {
	Scope                                          tenancy.Scope
	DocumentID                                     string
	Status                                         Status
	Reason, ActorID, IdempotencyKey, CorrelationID string
}
type PeriodCommand struct {
	Scope                                          tenancy.Scope
	PeriodID                                       string
	Action                                         PeriodAction
	Reason, ActorID, IdempotencyKey, CorrelationID string
}
type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, c clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || c == nil {
		return nil, errors.New("financial operations repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: c}, nil
}
func (s *Service) Documents(ctx context.Context, scope tenancy.Scope, actor, cursor string, limit int) (Page, error) {
	if limit < 1 || limit > 200 {
		return Page{}, ErrInvalidCommand
	}
	return s.repository.ListFinancialDocuments(ctx, scope.Normalize(), identity.NormalizeClaim(actor), identity.NormalizeClaim(cursor), limit)
}
func (s *Service) Document(ctx context.Context, scope tenancy.Scope, actor, id string) (Document, error) {
	return s.repository.FinancialDocument(ctx, scope.Normalize(), identity.NormalizeClaim(actor), identity.NormalizeClaim(id))
}
func (s *Service) Periods(ctx context.Context, scope tenancy.Scope, actor string) ([]FiscalPeriod, error) {
	return s.repository.ListFiscalPeriods(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}
func (s *Service) PeriodActions(ctx context.Context, scope tenancy.Scope, actor string) (PeriodActionPage, error) {
	return s.repository.ListFiscalPeriodActions(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}

func (s *Service) Create(ctx context.Context, c CreateCommand) (Document, error) {
	c.Scope = c.Scope.Normalize()
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.Reason = strings.TrimSpace(c.Reason)
	c.FromAccountID = identity.NormalizeClaim(c.FromAccountID)
	c.ToAccountID = identity.NormalizeClaim(c.ToAccountID)
	c.ReversesDocumentID = identity.NormalizeClaim(c.ReversesDocumentID)
	if c.Scope.Validate() != nil || c.ActorID == "" || len(c.Currency) != 3 || c.AccountingAt.IsZero() || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) {
		return Document{}, ErrInvalidCommand
	}
	switch c.Type {
	case ManualJournal:
		if c.FromAccountID != "" || c.ToAccountID != "" || c.ReversesDocumentID != "" || validateLines(c.Lines) != nil {
			return Document{}, ErrInvalidCommand
		}
	case CashTransfer:
		if !identity.IsUUID(c.FromAccountID) || !identity.IsUUID(c.ToAccountID) || c.FromAccountID == c.ToAccountID || len(c.Lines) != 1 || c.Lines[0].DebitMinor <= 0 || c.Lines[0].CreditMinor != 0 || !wire.IsSafeInteger(c.Lines[0].DebitMinor) {
			return Document{}, ErrInvalidCommand
		}
	case BankAdjustment:
		if !identity.IsUUID(c.FromAccountID) || len(c.Lines) != 1 || c.Lines[0].AccountID == "" || (c.Lines[0].DebitMinor == 0) == (c.Lines[0].CreditMinor == 0) || !wire.IsSafeInteger(c.Lines[0].DebitMinor) || !wire.IsSafeInteger(c.Lines[0].CreditMinor) {
			return Document{}, ErrInvalidCommand
		}
	case Reversal:
		if !identity.IsUUID(c.ReversesDocumentID) || len(c.Lines) != 0 {
			return Document{}, ErrInvalidCommand
		}
	default:
		return Document{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Document{}, err
	}
	now := s.clock.Now().UTC()
	d := Document{ID: id, Scope: c.Scope, Type: c.Type, Status: Draft, Currency: c.Currency, AccountingAt: c.AccountingAt.UTC(), Reason: c.Reason, FromAccountID: c.FromAccountID, ToAccountID: c.ToAccountID, ReversesDocumentID: c.ReversesDocumentID, CreatedBy: c.ActorID, CreatedAt: now, Lines: c.Lines}
	h := commandHash(c)
	return s.repository.CreateFinancialDocument(ctx, d, c.IdempotencyKey, h)
}
func (s *Service) Transition(ctx context.Context, c TransitionCommand) (Document, error) {
	c.Scope = c.Scope.Normalize()
	c.DocumentID = identity.NormalizeClaim(c.DocumentID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.DocumentID) || c.ActorID == "" || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) || (c.Status != Submitted && c.Status != Posted && c.Status != Rejected) {
		return Document{}, ErrInvalidCommand
	}
	jid := ""
	if c.Status == Posted {
		var err error
		jid, err = s.ids.New()
		if err != nil {
			return Document{}, err
		}
	}
	return s.repository.TransitionFinancialDocument(ctx, c.Scope, c.ActorID, c.DocumentID, c.Status, c.Reason, jid, c.IdempotencyKey, commandHash(c), s.clock.Now().UTC())
}
func (s *Service) RequestPeriodAction(ctx context.Context, c PeriodCommand) (PeriodActionRequest, error) {
	c.Scope = c.Scope.Normalize()
	c.PeriodID = identity.NormalizeClaim(c.PeriodID)
	c.ActorID = identity.NormalizeClaim(c.ActorID)
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Scope.Validate() != nil || !identity.IsUUID(c.PeriodID) || c.ActorID == "" || !validReason(c.Reason) || !validIdempotency(c.IdempotencyKey) || (c.Action != ClosePeriod && c.Action != ReopenPeriod) {
		return PeriodActionRequest{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return PeriodActionRequest{}, err
	}
	return s.repository.RequestFiscalPeriodAction(ctx, c.Scope, c.ActorID, c.PeriodID, c.Action, c.Reason, id, c.IdempotencyKey, commandHash(c), s.clock.Now().UTC())
}
func (s *Service) ApprovePeriodAction(ctx context.Context, scope tenancy.Scope, actor, requestID, reason, idem string) (PeriodActionRequest, error) {
	scope = scope.Normalize()
	actor = identity.NormalizeClaim(actor)
	requestID = identity.NormalizeClaim(requestID)
	reason = strings.TrimSpace(reason)
	if scope.Validate() != nil || actor == "" || !identity.IsUUID(requestID) || !validReason(reason) || !validIdempotency(idem) {
		return PeriodActionRequest{}, ErrInvalidCommand
	}
	return s.repository.ApproveFiscalPeriodAction(ctx, scope, actor, requestID, reason, idem, commandHash([]string{requestID, reason}), s.clock.Now().UTC())
}
func validateLines(lines []finance.JournalEntry) error {
	j := finance.Journal{ID: "x", TenantID: "x", CompanyID: "x", SourceType: "x", SourceID: "x", Currency: "TZS", Entries: lines}
	var debits, credits int64
	for _, l := range lines {
		if !wire.IsSafeInteger(l.DebitMinor) || !wire.IsSafeInteger(l.CreditMinor) {
			return ErrInvalidCommand
		}
		debits += l.DebitMinor
		credits += l.CreditMinor
		if !wire.IsSafeInteger(debits) || !wire.IsSafeInteger(credits) {
			return ErrInvalidCommand
		}
	}
	return j.Validate()
}
func validReason(v string) bool { n := utf8.RuneCountInString(v); return n >= 8 && n <= 500 }
func validIdempotency(v string) bool {
	return len(strings.TrimSpace(v)) >= 16 && len(strings.TrimSpace(v)) <= 128
}
func commandHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
