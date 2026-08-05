// Package receivables owns effective credit authorization and customer AR
// explanations. Posting remains atomic through the sales transaction boundary.
package receivables

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
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const creditPolicyOperation = "customers.credit-policy.schedule.v1"

type Repository interface {
	CustomerAccountDetail(ctx context.Context, scope tenancy.Scope, actorID, customerID string, at time.Time) (customers.AccountDetail, error)
	ScheduleCustomerCreditPolicy(ctx context.Context, policy customers.CreditPolicy, policyAudit audit.Event, policyEvent outbox.Event, commandAt time.Time) (customers.CreditPolicy, error)
}

type ScheduleCreditPolicyCommand struct {
	Scope            tenancy.Scope
	ActorID          string
	CustomerID       string
	CreditEnabled    bool                       `json:"credit_enabled"`
	CreditLimitMinor int64                      `json:"credit_limit_minor"`
	PaymentTermsDays int64                      `json:"payment_terms_days"`
	MaxOverdueDays   int64                      `json:"max_overdue_days"`
	RiskStatus       customers.CreditRiskStatus `json:"risk_status"`
	Reason           string                     `json:"reason"`
	EffectiveFrom    time.Time                  `json:"effective_from"`
	IdempotencyKey   string
	CorrelationID    string
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, serviceClock clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || serviceClock == nil {
		return nil, errors.New("receivables repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: serviceClock}, nil
}

func (s *Service) Account(ctx context.Context, scope tenancy.Scope, actorID, customerID string) (customers.AccountDetail, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	customerID = identity.NormalizeClaim(customerID)
	if scope.Validate() != nil || actorID == "" || !identity.IsUUID(customerID) {
		return customers.AccountDetail{}, sales.ErrInvalidCommand
	}
	result, err := s.repository.CustomerAccountDetail(ctx, scope, actorID, customerID, s.clock.Now().UTC())
	if err != nil {
		return customers.AccountDetail{}, err
	}
	if err := validateAccountDetail(result); err != nil {
		return customers.AccountDetail{}, err
	}
	if result.ScheduledPolicies == nil {
		result.ScheduledPolicies = make([]customers.CreditPolicy, 0)
	}
	if result.OpenItems == nil {
		result.OpenItems = make([]customers.ReceivableItem, 0)
	}
	return result, nil
}

func (s *Service) ScheduleCreditPolicy(ctx context.Context, command ScheduleCreditPolicyCommand) (customers.CreditPolicy, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.CustomerID = identity.NormalizeClaim(command.CustomerID)
	command.RiskStatus = customers.CreditRiskStatus(strings.ToUpper(strings.TrimSpace(string(command.RiskStatus))))
	command.Reason = strings.TrimSpace(command.Reason)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.CorrelationID = identity.NormalizeClaim(command.CorrelationID)
	command.EffectiveFrom = command.EffectiveFrom.UTC()
	if command.Scope.Validate() != nil || command.ActorID == "" || !identity.IsUUID(command.CustomerID) ||
		command.EffectiveFrom.IsZero() || command.CreditLimitMinor < 0 || !wire.IsSafeInteger(command.CreditLimitMinor) ||
		command.PaymentTermsDays < 0 || command.PaymentTermsDays > 365 ||
		command.MaxOverdueDays < 0 || command.MaxOverdueDays > 3650 ||
		(command.RiskStatus != customers.CreditRiskStandard && command.RiskStatus != customers.CreditRiskWatch && command.RiskStatus != customers.CreditRiskHold) ||
		utf8.RuneCountInString(command.Reason) < 8 || utf8.RuneCountInString(command.Reason) > 500 ||
		len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 128 ||
		(!command.CreditEnabled && command.CreditLimitMinor != 0) {
		return customers.CreditPolicy{}, sales.ErrInvalidCommand
	}
	canonical, err := json.Marshal(struct {
		CustomerID       string                     `json:"customer_id"`
		CreditEnabled    bool                       `json:"credit_enabled"`
		CreditLimitMinor int64                      `json:"credit_limit_minor"`
		PaymentTermsDays int64                      `json:"payment_terms_days"`
		MaxOverdueDays   int64                      `json:"max_overdue_days"`
		RiskStatus       customers.CreditRiskStatus `json:"risk_status"`
		Reason           string                     `json:"reason"`
		EffectiveFrom    time.Time                  `json:"effective_from"`
	}{command.CustomerID, command.CreditEnabled, command.CreditLimitMinor, command.PaymentTermsDays,
		command.MaxOverdueDays, command.RiskStatus, command.Reason, command.EffectiveFrom})
	if err != nil {
		return customers.CreditPolicy{}, err
	}
	hash := sha256.Sum256(canonical)
	policyID, err := s.ids.New()
	if err != nil {
		return customers.CreditPolicy{}, err
	}
	auditID, err := s.ids.New()
	if err != nil {
		return customers.CreditPolicy{}, err
	}
	eventID, err := s.ids.New()
	if err != nil {
		return customers.CreditPolicy{}, err
	}
	now := s.clock.Now().UTC()
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = policyID
	}
	policy := customers.CreditPolicy{
		ID: policyID, Scope: command.Scope, CustomerID: command.CustomerID,
		CreditEnabled: command.CreditEnabled, CreditLimitMinor: command.CreditLimitMinor,
		PaymentTermsDays: command.PaymentTermsDays, MaxOverdueDays: command.MaxOverdueDays,
		RiskStatus: command.RiskStatus, Reason: command.Reason, EffectiveFrom: command.EffectiveFrom,
		ApprovedBy: command.ActorID, CreatedAt: now, CorrelationID: correlation,
		IdempotencyKey: command.IdempotencyKey, RequestHash: hex.EncodeToString(hash[:]),
	}
	payload, _ := json.Marshal(policy)
	result, err := s.repository.ScheduleCustomerCreditPolicy(ctx, policy,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID,
			ActorID: command.ActorID, Action: "customer.credit_policy_scheduled", EntityType: "customer",
			EntityID: command.CustomerID, CorrelationID: correlation, CausationID: policyID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID,
			AggregateType: "customer", AggregateID: command.CustomerID, EventType: "customer.credit_policy_scheduled",
			Version: 1, CorrelationID: correlation, CausationID: policyID, Payload: payload, OccurredAt: now}, now)
	if err != nil {
		return customers.CreditPolicy{}, err
	}
	if !wire.IsSafeInteger(result.CreditLimitMinor) || !wire.IsSafeInteger(result.PaymentTermsDays) || !wire.IsSafeInteger(result.MaxOverdueDays) {
		return customers.CreditPolicy{}, wire.ErrUnsafeInteger
	}
	return result, nil
}

func validateAccountDetail(value customers.AccountDetail) error {
	numbers := []int64{
		value.ActivePolicy.CreditLimitMinor, value.ActivePolicy.PaymentTermsDays, value.ActivePolicy.MaxOverdueDays,
		value.Aging.LedgerBalanceMinor, value.Aging.OpenInvoiceMinor, value.Aging.UnappliedCreditMinor,
		value.Aging.CalculatedExposure, value.Aging.OverdueMinor, value.Aging.OldestOverdueDays,
		value.Aging.Buckets.CurrentMinor, value.Aging.Buckets.Days1To30, value.Aging.Buckets.Days31To60,
		value.Aging.Buckets.Days61To90, value.Aging.Buckets.DaysOver90,
	}
	for _, item := range value.OpenItems {
		numbers = append(numbers, item.AmountMinor, item.OutstandingMinor)
	}
	for _, policy := range value.ScheduledPolicies {
		numbers = append(numbers, policy.CreditLimitMinor, policy.PaymentTermsDays, policy.MaxOverdueDays)
	}
	for _, number := range numbers {
		if !wire.IsSafeInteger(number) {
			return wire.ErrUnsafeInteger
		}
	}
	return nil
}

func OperationName() string { return creditPolicyOperation }
