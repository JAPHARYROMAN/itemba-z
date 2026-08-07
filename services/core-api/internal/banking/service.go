package banking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	importOperation    = "banking.statement.import.v1"
	matchOperation     = "banking.statement.match.v1"
	reconcileOperation = "banking.statement.reconcile.v1"
)

type Repository interface {
	ListBankAccounts(context.Context, tenancy.Scope, string) ([]Account, error)
	ImportBankStatement(context.Context, Statement, audit.Event, outbox.Event, string, string) (Statement, error)
	ListBankStatements(context.Context, tenancy.Scope, string, string, int) (Page, error)
	BankStatement(context.Context, tenancy.Scope, string, string) (Statement, error)
	MatchBankStatementLine(context.Context, tenancy.Scope, string, string, string, string, Match, audit.Event, outbox.Event, string, string) (Statement, error)
	ReconcileBankStatement(context.Context, tenancy.Scope, string, string, string, time.Time, audit.Event, outbox.Event, string, string) (Statement, error)
}

type ImportLine struct {
	TransactionAt     time.Time `json:"transaction_at"`
	ExternalReference string    `json:"external_reference"`
	Description       string    `json:"description"`
	AmountMinor       int64     `json:"amount_minor"`
}

type ImportCommand struct {
	Scope             tenancy.Scope
	AccountID         string
	ExternalReference string
	Currency          string
	PeriodStart       time.Time
	PeriodEnd         time.Time
	OpeningMinor      int64
	ClosingMinor      int64
	Lines             []ImportLine
	ActorID           string
	IdempotencyKey    string
	CorrelationID     string
}

type MatchCommand struct {
	Scope          tenancy.Scope
	StatementID    string
	LineID         string
	JournalLineID  string
	Reason         string
	ActorID        string
	IdempotencyKey string
	CorrelationID  string
}

type ReconcileCommand struct {
	Scope          tenancy.Scope
	StatementID    string
	Reason         string
	ActorID        string
	IdempotencyKey string
	CorrelationID  string
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, serviceClock clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || serviceClock == nil {
		return nil, errors.New("banking repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: serviceClock}, nil
}

func (s *Service) Accounts(ctx context.Context, scope tenancy.Scope, actorID string) ([]Account, error) {
	return s.repository.ListBankAccounts(ctx, scope.Normalize(), identity.NormalizeClaim(actorID))
}

func (s *Service) Statements(ctx context.Context, scope tenancy.Scope, actorID, cursor string, limit int) (Page, error) {
	if limit < 1 || limit > 200 {
		return Page{}, ErrInvalidCommand
	}
	return s.repository.ListBankStatements(ctx, scope.Normalize(), identity.NormalizeClaim(actorID), identity.NormalizeClaim(cursor), limit)
}

func (s *Service) Statement(ctx context.Context, scope tenancy.Scope, actorID, statementID string) (Statement, error) {
	return s.repository.BankStatement(ctx, scope.Normalize(), identity.NormalizeClaim(actorID), identity.NormalizeClaim(statementID))
}

func (s *Service) Import(ctx context.Context, command ImportCommand) (Statement, error) {
	command.Scope = command.Scope.Normalize()
	command.AccountID, command.ActorID = identity.NormalizeClaim(command.AccountID), identity.NormalizeClaim(command.ActorID)
	command.ExternalReference, command.Currency = strings.TrimSpace(command.ExternalReference), strings.ToUpper(strings.TrimSpace(command.Currency))
	command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.IdempotencyKey), identity.NormalizeClaim(command.CorrelationID)
	if err := validateImport(command); err != nil {
		return Statement{}, err
	}
	statementID, err := s.ids.New()
	if err != nil {
		return Statement{}, err
	}
	now := s.clock.Now().UTC()
	statement := Statement{ID: statementID, Scope: command.Scope, AccountID: command.AccountID, ExternalReference: command.ExternalReference,
		Currency: command.Currency, PeriodStart: command.PeriodStart.UTC(), PeriodEnd: command.PeriodEnd.UTC(), OpeningMinor: command.OpeningMinor,
		ClosingMinor: command.ClosingMinor, Status: Imported, ImportedBy: command.ActorID, ImportedAt: now, CorrelationID: command.CorrelationID}
	if statement.CorrelationID == "" {
		statement.CorrelationID = statement.ID
	}
	var movement int64
	for _, input := range command.Lines {
		lineID, idErr := s.ids.New()
		if idErr != nil {
			return Statement{}, idErr
		}
		statement.Lines = append(statement.Lines, StatementLine{ID: lineID, TransactionAt: input.TransactionAt.UTC(), ExternalReference: strings.TrimSpace(input.ExternalReference), Description: strings.TrimSpace(input.Description), AmountMinor: input.AmountMinor, Candidates: []Candidate{}})
		movement += input.AmountMinor
		if !wire.IsSafeInteger(movement) {
			return Statement{}, ErrInvalidCommand
		}
	}
	if statement.OpeningMinor+movement != statement.ClosingMinor {
		return Statement{}, ErrStatementImbalance
	}
	auditID, eventID, err := s.twoIDs()
	if err != nil {
		return Statement{}, err
	}
	canonical, _ := json.Marshal(command)
	hash := sha256.Sum256(canonical)
	payload, _ := json.Marshal(statement)
	return s.repository.ImportBankStatement(ctx, statement,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "banking.statement_imported", EntityType: "bank_statement", EntityID: statement.ID, CorrelationID: statement.CorrelationID, CausationID: statement.ID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "bank_statement", AggregateID: statement.ID, EventType: "banking.statement_imported", Version: 1, CorrelationID: statement.CorrelationID, CausationID: statement.ID, Payload: payload, OccurredAt: now}, command.IdempotencyKey, hex.EncodeToString(hash[:]))
}

func (s *Service) Match(ctx context.Context, command MatchCommand) (Statement, error) {
	command.Scope = command.Scope.Normalize()
	command.StatementID, command.LineID, command.ActorID = identity.NormalizeClaim(command.StatementID), identity.NormalizeClaim(command.LineID), identity.NormalizeClaim(command.ActorID)
	command.JournalLineID, command.Reason = strings.TrimSpace(command.JournalLineID), strings.TrimSpace(command.Reason)
	command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.IdempotencyKey), identity.NormalizeClaim(command.CorrelationID)
	if command.Scope.Validate() != nil || !identity.IsUUID(command.StatementID) || !identity.IsUUID(command.LineID) || command.ActorID == "" || command.JournalLineID == "" || !validReason(command.Reason) || !validIdempotency(command.IdempotencyKey) {
		return Statement{}, ErrInvalidCommand
	}
	matchID, auditID, eventID, err := s.threeIDs()
	if err != nil {
		return Statement{}, err
	}
	now := s.clock.Now().UTC()
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = matchID
	}
	match := Match{ID: matchID, JournalLineID: command.JournalLineID, ActorID: command.ActorID, Reason: command.Reason, OccurredAt: now}
	canonical, _ := json.Marshal(command)
	hash := sha256.Sum256(canonical)
	payload, _ := json.Marshal(match)
	return s.repository.MatchBankStatementLine(ctx, command.Scope, command.ActorID, command.StatementID, command.LineID, command.JournalLineID, match,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "banking.statement_line_matched", EntityType: "bank_statement", EntityID: command.StatementID, CorrelationID: correlation, CausationID: matchID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "bank_statement", AggregateID: command.StatementID, EventType: "banking.statement_line_matched", Version: 1, CorrelationID: correlation, CausationID: matchID, Payload: payload, OccurredAt: now}, command.IdempotencyKey, hex.EncodeToString(hash[:]))
}

func (s *Service) Reconcile(ctx context.Context, command ReconcileCommand) (Statement, error) {
	command.Scope = command.Scope.Normalize()
	command.StatementID, command.ActorID = identity.NormalizeClaim(command.StatementID), identity.NormalizeClaim(command.ActorID)
	command.Reason, command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.Reason), strings.TrimSpace(command.IdempotencyKey), identity.NormalizeClaim(command.CorrelationID)
	if command.Scope.Validate() != nil || !identity.IsUUID(command.StatementID) || command.ActorID == "" || !validReason(command.Reason) || !validIdempotency(command.IdempotencyKey) {
		return Statement{}, ErrInvalidCommand
	}
	auditID, eventID, err := s.twoIDs()
	if err != nil {
		return Statement{}, err
	}
	now := s.clock.Now().UTC()
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = auditID
	}
	canonical, _ := json.Marshal(command)
	hash := sha256.Sum256(canonical)
	payload, _ := json.Marshal(command)
	return s.repository.ReconcileBankStatement(ctx, command.Scope, command.ActorID, command.StatementID, command.Reason, now,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "banking.statement_reconciled", EntityType: "bank_statement", EntityID: command.StatementID, CorrelationID: correlation, CausationID: auditID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "bank_statement", AggregateID: command.StatementID, EventType: "banking.statement_reconciled", Version: 1, CorrelationID: correlation, CausationID: auditID, Payload: payload, OccurredAt: now}, command.IdempotencyKey, hex.EncodeToString(hash[:]))
}

func validateImport(command ImportCommand) error {
	if command.Scope.Validate() != nil || !identity.IsUUID(command.AccountID) || command.ActorID == "" || len(command.Currency) != 3 || !validIdempotency(command.IdempotencyKey) || utf8.RuneCountInString(command.ExternalReference) < 3 || utf8.RuneCountInString(command.ExternalReference) > 120 || len(command.Lines) == 0 || len(command.Lines) > 500 || command.PeriodStart.IsZero() || !command.PeriodEnd.After(command.PeriodStart) || !wire.IsSafeInteger(command.OpeningMinor) || !wire.IsSafeInteger(command.ClosingMinor) {
		return ErrInvalidCommand
	}
	for _, line := range command.Lines {
		if line.TransactionAt.Before(command.PeriodStart) || line.TransactionAt.After(command.PeriodEnd) || line.AmountMinor == 0 || !wire.IsSafeInteger(line.AmountMinor) || utf8.RuneCountInString(strings.TrimSpace(line.Description)) < 3 || utf8.RuneCountInString(strings.TrimSpace(line.Description)) > 300 || utf8.RuneCountInString(strings.TrimSpace(line.ExternalReference)) > 120 {
			return ErrInvalidCommand
		}
	}
	return nil
}

func validReason(value string) bool {
	count := utf8.RuneCountInString(value)
	return count >= 8 && count <= 500
}
func validIdempotency(value string) bool { return len(value) >= 16 && len(value) <= 128 }
func (s *Service) twoIDs() (string, string, error) {
	a, err := s.ids.New()
	if err != nil {
		return "", "", err
	}
	b, err := s.ids.New()
	return a, b, err
}
func (s *Service) threeIDs() (string, string, string, error) {
	a, b, err := s.twoIDs()
	if err != nil {
		return "", "", "", err
	}
	c, err := s.ids.New()
	return a, b, c, err
}
