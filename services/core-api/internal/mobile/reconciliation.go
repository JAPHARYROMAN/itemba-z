package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type ReconciliationStatus string
type ResolutionAction string

const (
	ReconciliationOpen     ReconciliationStatus = "OPEN"
	ReconciliationResolved ReconciliationStatus = "RESOLVED"

	ResolutionCashRefunded       ResolutionAction = "CASH_REFUNDED"
	ResolutionPostedExternally   ResolutionAction = "POSTED_EXTERNALLY"
	ResolutionDuplicateConfirmed ResolutionAction = "DUPLICATE_CONFIRMED"
)

var ErrReconciliationResolved = errors.New("offline reconciliation case has already been resolved")

type ReconciliationCase struct {
	ID                   string                    `json:"id"`
	Scope                tenancy.Scope             `json:"scope"`
	Status               ReconciliationStatus      `json:"status"`
	DeviceID             string                    `json:"device_id"`
	ClientTransactionID  string                    `json:"client_transaction_id"`
	ClientTimestamp      time.Time                 `json:"client_timestamp"`
	AppVersion           string                    `json:"app_version"`
	MasterDataVersion    int64                     `json:"master_data_version"`
	PriceVersion         int64                     `json:"price_version"`
	CatalogSnapshotToken string                    `json:"catalog_snapshot_token"`
	FailureCode          string                    `json:"failure_code"`
	Command              json.RawMessage           `json:"command"`
	CommandHash          string                    `json:"-"`
	CreatedBy            string                    `json:"created_by"`
	CorrelationID        string                    `json:"correlation_id"`
	CreatedAt            time.Time                 `json:"created_at"`
	Resolution           *ReconciliationResolution `json:"resolution,omitempty"`
}

type ReconciliationResolution struct {
	ID                string           `json:"id"`
	CaseID            string           `json:"case_id"`
	Action            ResolutionAction `json:"action"`
	Reason            string           `json:"reason"`
	ExternalReference string           `json:"external_reference,omitempty"`
	ResolvedBy        string           `json:"resolved_by"`
	CorrelationID     string           `json:"correlation_id"`
	ResolvedAt        time.Time        `json:"resolved_at"`
	IdempotencyKey    string           `json:"-"`
	RequestHash       string           `json:"-"`
}

type ReconciliationPage struct {
	Items      []ReconciliationCase `json:"items"`
	NextCursor *string              `json:"next_cursor"`
}

type ResolveReconciliationCommand struct {
	Scope             tenancy.Scope
	ActorID           string
	CaseID            string
	Action            ResolutionAction `json:"action"`
	Reason            string           `json:"reason"`
	ExternalReference string           `json:"external_reference,omitempty"`
	IdempotencyKey    string
	CorrelationID     string
}

func (s *Service) ReconciliationCases(ctx context.Context, scope tenancy.Scope, actorID, status, cursor string, pageSize int) (ReconciliationPage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" || pageSize < 1 || pageSize > 200 {
		return ReconciliationPage{}, sales.ErrInvalidCommand
	}
	parsedStatus := ReconciliationStatus(strings.ToUpper(strings.TrimSpace(status)))
	if parsedStatus != "" && parsedStatus != ReconciliationOpen && parsedStatus != ReconciliationResolved {
		return ReconciliationPage{}, sales.ErrInvalidCommand
	}
	afterID, err := decodeReconciliationCursor(cursor)
	if err != nil {
		return ReconciliationPage{}, sales.ErrInvalidCommand
	}
	items, err := s.repository.ListReconciliationCases(ctx, scope, actorID, parsedStatus, afterID, pageSize+1)
	if err != nil {
		return ReconciliationPage{}, err
	}
	if len(items) <= pageSize {
		return ReconciliationPage{Items: items}, nil
	}
	next := encodeReconciliationCursor(items[pageSize-1].ID)
	return ReconciliationPage{Items: items[:pageSize], NextCursor: &next}, nil
}

func (s *Service) ReconciliationCase(ctx context.Context, scope tenancy.Scope, actorID, caseID string) (ReconciliationCase, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	caseID = identity.NormalizeClaim(caseID)
	if scope.Validate() != nil || actorID == "" || !identity.IsUUID(caseID) {
		return ReconciliationCase{}, sales.ErrInvalidCommand
	}
	return s.repository.ReconciliationCase(ctx, scope, actorID, caseID)
}

func (s *Service) ResolveReconciliation(ctx context.Context, command ResolveReconciliationCommand) (ReconciliationCase, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.CaseID = identity.NormalizeClaim(command.CaseID)
	command.Action = ResolutionAction(strings.ToUpper(strings.TrimSpace(string(command.Action))))
	command.Reason = strings.TrimSpace(command.Reason)
	command.ExternalReference = strings.TrimSpace(command.ExternalReference)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.CorrelationID = identity.NormalizeClaim(command.CorrelationID)
	if command.Scope.Validate() != nil || command.ActorID == "" || !identity.IsUUID(command.CaseID) ||
		len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 128 ||
		utf8.RuneCountInString(command.Reason) < 8 || utf8.RuneCountInString(command.Reason) > 500 ||
		utf8.RuneCountInString(command.ExternalReference) > 200 || !validResolutionAction(command.Action) {
		return ReconciliationCase{}, sales.ErrInvalidCommand
	}
	if command.Action == ResolutionPostedExternally && command.ExternalReference == "" {
		return ReconciliationCase{}, sales.ErrInvalidCommand
	}
	canonical, err := json.Marshal(struct {
		CaseID            string           `json:"case_id"`
		Action            ResolutionAction `json:"action"`
		Reason            string           `json:"reason"`
		ExternalReference string           `json:"external_reference,omitempty"`
	}{command.CaseID, command.Action, command.Reason, command.ExternalReference})
	if err != nil {
		return ReconciliationCase{}, err
	}
	hash := sha256.Sum256(canonical)
	now := s.clock.Now().UTC()
	resolutionID, err := s.ids.New()
	if err != nil {
		return ReconciliationCase{}, err
	}
	auditID, err := s.ids.New()
	if err != nil {
		return ReconciliationCase{}, err
	}
	eventID, err := s.ids.New()
	if err != nil {
		return ReconciliationCase{}, err
	}
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = resolutionID
	}
	resolution := ReconciliationResolution{
		ID: resolutionID, CaseID: command.CaseID, Action: command.Action, Reason: command.Reason,
		ExternalReference: command.ExternalReference, ResolvedBy: command.ActorID,
		CorrelationID: correlation, ResolvedAt: now, IdempotencyKey: command.IdempotencyKey,
		RequestHash: hex.EncodeToString(hash[:]),
	}
	payload, err := json.Marshal(resolution)
	if err != nil {
		return ReconciliationCase{}, err
	}
	return s.repository.ResolveReconciliationCase(ctx, command.Scope, command.ActorID, resolution,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "mobile.reconciliation.resolved", EntityType: "mobile_reconciliation_case", EntityID: command.CaseID, CorrelationID: correlation, CausationID: resolutionID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "mobile_reconciliation_case", AggregateID: command.CaseID, EventType: "mobile.reconciliation.resolved", Version: 1, CorrelationID: correlation, CausationID: resolutionID, Payload: payload, OccurredAt: now})
}

func validResolutionAction(value ResolutionAction) bool {
	return value == ResolutionCashRefunded || value == ResolutionPostedExternally || value == ResolutionDuplicateConfirmed
}

func encodeReconciliationCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeReconciliationCursor(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || !identity.IsUUID(string(decoded)) {
		return "", errors.New("invalid reconciliation cursor")
	}
	return strings.ToLower(string(decoded)), nil
}
