package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type OperationsRepository interface {
	IntegrationWorkspace(context.Context, tenancy.Scope, string) (Workspace, error)
	CreateIntegrationRoute(context.Context, tenancy.Scope, Route, string, string) (Route, error)
	TransitionIntegrationRoute(context.Context, tenancy.Scope, string, string, RouteStatus, string, string, string, time.Time) (Route, error)
	ReplayIntegrationDelivery(context.Context, tenancy.Scope, string, string, string, string, string, string, time.Time) (Delivery, error)
	ResetIntegrationCircuit(context.Context, tenancy.Scope, string, string, string, string, string, string, time.Time) (Route, error)
}

type Service struct {
	repository OperationsRepository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository OperationsRepository, ids identity.Generator, serviceClock clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || serviceClock == nil {
		return nil, errors.New("integration repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: serviceClock}, nil
}

type CreateRouteCommand struct {
	Scope                                                                   tenancy.Scope
	Capability                                                              Capability
	ProviderCode, ContractVersion, EndpointURL, SecretReference             string
	TimeoutMilliseconds, MaxAttempts, BaseBackoffSeconds, MaxBackoffSeconds int
	CircuitFailureThreshold, CircuitOpenSeconds                             int
	ValidFrom                                                               time.Time
	ValidUntil                                                              *time.Time
	Reason, ActorID, IdempotencyKey                                         string
}

type TransitionRouteCommand struct {
	Scope                           tenancy.Scope
	RouteID                         string
	Status                          RouteStatus
	Reason, ActorID, IdempotencyKey string
}
type ReplayCommand struct {
	Scope                                       tenancy.Scope
	DeliveryID, Reason, ActorID, IdempotencyKey string
}
type ResetCircuitCommand struct {
	Scope                                    tenancy.Scope
	RouteID, Reason, ActorID, IdempotencyKey string
}

var providerPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)

func (s *Service) Workspace(ctx context.Context, scope tenancy.Scope, actor string) (Workspace, error) {
	return s.repository.IntegrationWorkspace(ctx, scope.Normalize(), identity.NormalizeClaim(actor))
}

func (s *Service) CreateRoute(ctx context.Context, command CreateRouteCommand) (Route, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.ProviderCode = strings.ToUpper(strings.TrimSpace(command.ProviderCode))
	command.ContractVersion = strings.TrimSpace(command.ContractVersion)
	command.EndpointURL = strings.TrimSpace(command.EndpointURL)
	command.SecretReference = strings.TrimSpace(command.SecretReference)
	command.Reason = strings.TrimSpace(command.Reason)
	endpoint, endpointErr := url.Parse(command.EndpointURL)
	secret, secretErr := url.Parse(command.SecretReference)
	if command.Scope.Validate() != nil || command.ActorID == "" || !validCapability(command.Capability) || !providerPattern.MatchString(command.ProviderCode) || len(command.ContractVersion) < 1 || len(command.ContractVersion) > 100 || endpointErr != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || secretErr != nil || secret.Scheme == "" || secret.Host == "" || secret.User != nil || command.TimeoutMilliseconds < 1000 || command.TimeoutMilliseconds > 120000 || command.MaxAttempts < 1 || command.MaxAttempts > 20 || command.BaseBackoffSeconds < 1 || command.MaxBackoffSeconds < command.BaseBackoffSeconds || command.MaxBackoffSeconds > 86400 || command.CircuitFailureThreshold < 1 || command.CircuitFailureThreshold > 100 || command.CircuitOpenSeconds < 1 || command.CircuitOpenSeconds > 86400 || command.ValidFrom.IsZero() || command.ValidUntil != nil && !command.ValidUntil.After(command.ValidFrom) || !validReason(command.Reason) || !validIdempotency(command.IdempotencyKey) {
		return Route{}, ErrInvalidCommand
	}
	id, err := s.ids.New()
	if err != nil {
		return Route{}, err
	}
	now := s.clock.Now().UTC()
	route := Route{ID: id, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, Capability: command.Capability, ProviderCode: command.ProviderCode, ContractVersion: command.ContractVersion, EndpointURL: command.EndpointURL, SecretReference: command.SecretReference, Timeout: time.Duration(command.TimeoutMilliseconds) * time.Millisecond, TimeoutMilliseconds: command.TimeoutMilliseconds, MaxAttempts: command.MaxAttempts, BaseBackoff: time.Duration(command.BaseBackoffSeconds) * time.Second, BaseBackoffSeconds: command.BaseBackoffSeconds, MaxBackoff: time.Duration(command.MaxBackoffSeconds) * time.Second, MaxBackoffSeconds: command.MaxBackoffSeconds, CircuitFailureThreshold: command.CircuitFailureThreshold, CircuitOpen: time.Duration(command.CircuitOpenSeconds) * time.Second, CircuitOpenSeconds: command.CircuitOpenSeconds, Status: RouteDraft, ValidFrom: command.ValidFrom.UTC(), ValidUntil: command.ValidUntil, Reason: command.Reason, CreatedBy: command.ActorID, CreatedAt: now}
	return s.repository.CreateIntegrationRoute(ctx, command.Scope, route, command.IdempotencyKey, commandHash(command))
}

func (s *Service) TransitionRoute(ctx context.Context, command TransitionRouteCommand) (Route, error) {
	command.Scope = command.Scope.Normalize()
	command.RouteID = identity.NormalizeClaim(command.RouteID)
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.Scope.Validate() != nil || !identity.IsUUID(command.RouteID) || command.ActorID == "" || (command.Status != RouteActive && command.Status != RouteSuspended && command.Status != RouteRetired) || !validReason(command.Reason) || !validIdempotency(command.IdempotencyKey) {
		return Route{}, ErrInvalidCommand
	}
	return s.repository.TransitionIntegrationRoute(ctx, command.Scope, command.ActorID, command.RouteID, command.Status, command.Reason, command.IdempotencyKey, commandHash(command), s.clock.Now().UTC())
}

func (s *Service) Replay(ctx context.Context, command ReplayCommand) (Delivery, error) {
	command.Scope = command.Scope.Normalize()
	command.DeliveryID = identity.NormalizeClaim(command.DeliveryID)
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.Scope.Validate() != nil || !identity.IsUUID(command.DeliveryID) || command.ActorID == "" || !validReason(command.Reason) || !validIdempotency(command.IdempotencyKey) {
		return Delivery{}, ErrInvalidCommand
	}
	replayID, err := s.ids.New()
	if err != nil {
		return Delivery{}, err
	}
	return s.repository.ReplayIntegrationDelivery(ctx, command.Scope, command.ActorID, command.DeliveryID, replayID, command.Reason, command.IdempotencyKey, commandHash(command), s.clock.Now().UTC())
}

func (s *Service) ResetCircuit(ctx context.Context, command ResetCircuitCommand) (Route, error) {
	command.Scope = command.Scope.Normalize()
	command.RouteID = identity.NormalizeClaim(command.RouteID)
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.Scope.Validate() != nil || !identity.IsUUID(command.RouteID) || command.ActorID == "" || !validReason(command.Reason) || !validIdempotency(command.IdempotencyKey) {
		return Route{}, ErrInvalidCommand
	}
	resetID, err := s.ids.New()
	if err != nil {
		return Route{}, err
	}
	return s.repository.ResetIntegrationCircuit(ctx, command.Scope, command.ActorID, command.RouteID, resetID, command.Reason, command.IdempotencyKey, commandHash(command), s.clock.Now().UTC())
}

func validCapability(value Capability) bool {
	switch value {
	case TRAFiscalization, PaymentCallback, BankStatement, PayrollExport, EmailDelivery, WhatsAppDelivery, ReceiptPrint:
		return true
	}
	return false
}
func validReason(value string) bool { n := utf8.RuneCountInString(value); return n >= 8 && n <= 500 }
func validIdempotency(value string) bool {
	n := len(strings.TrimSpace(value))
	return n >= 16 && n <= 128
}
func commandHash(value any) string {
	payload, _ := json.Marshal(value)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
