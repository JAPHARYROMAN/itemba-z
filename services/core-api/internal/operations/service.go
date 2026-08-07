package operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	createOperation     = "operations.document.create.v1"
	transitionOperation = "operations.document.transition.v1"
)

type Repository interface {
	CreateOperationDocument(context.Context, Document, audit.Event, outbox.Event, string, string) (Document, error)
	TransitionOperationDocument(context.Context, tenancy.Scope, string, string, Status, string, string, string, EffectIDs, audit.Event, outbox.Event, string, string, time.Time) (Document, error)
	OperationDocument(context.Context, tenancy.Scope, string, string) (Document, error)
	ListOperationDocuments(context.Context, tenancy.Scope, string, DocumentType, string, int) (Page, error)
	ListSuppliers(context.Context, tenancy.Scope, string, string, int) (SupplierPage, error)
}

type CreateCommand struct {
	Scope                  tenancy.Scope
	Type                   DocumentType
	PartyType              PartyType
	PartyID                string
	SourceDocumentID       string
	DestinationWarehouseID string
	Currency               string
	Reason                 string
	Lines                  []CommandLine
	ActorID                string
	IdempotencyKey         string
	CorrelationID          string
}

type CommandLine struct {
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
}

type TransitionCommand struct {
	Scope          tenancy.Scope
	DocumentID     string
	ToStatus       Status
	Reason         string
	PaymentMethod  string
	ActorID        string
	IdempotencyKey string
	CorrelationID  string
}

type EffectIDs struct {
	TransitionID string
	JournalID    string
	LedgerID     string
	PayableID    string
	PaymentID    string
	AllocationID string
	MovementIDs  []string
	ReserveIDs   []string
}

type Service struct {
	repository Repository
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, ids identity.Generator, serviceClock clock.Clock) (*Service, error) {
	if repository == nil || ids == nil || serviceClock == nil {
		return nil, errors.New("operations repository, id generator, and clock are required")
	}
	return &Service{repository: repository, ids: ids, clock: serviceClock}, nil
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (Document, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID, command.PartyID = identity.NormalizeClaim(command.ActorID), identity.NormalizeClaim(command.PartyID)
	command.SourceDocumentID = identity.NormalizeClaim(command.SourceDocumentID)
	command.DestinationWarehouseID = identity.NormalizeClaim(command.DestinationWarehouseID)
	command.Currency, command.Reason = strings.ToUpper(strings.TrimSpace(command.Currency)), strings.TrimSpace(command.Reason)
	command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.IdempotencyKey), identity.NormalizeClaim(command.CorrelationID)
	if err := validateCreate(command); err != nil {
		return Document{}, err
	}
	documentID, err := s.ids.New()
	if err != nil {
		return Document{}, err
	}
	now := s.clock.Now().UTC()
	document := Document{
		ID: documentID, Scope: command.Scope, Number: documentNumber(command.Type, documentID), Type: command.Type,
		Status: Draft, PartyType: command.PartyType, PartyID: command.PartyID, SourceDocumentID: command.SourceDocumentID,
		DestinationWarehouseID: command.DestinationWarehouseID, Currency: command.Currency, Reason: command.Reason,
		CreatedBy: command.ActorID, CreatedAt: now, CorrelationID: command.CorrelationID,
	}
	if document.CorrelationID == "" {
		document.CorrelationID = document.ID
	}
	seen := make(map[string]struct{}, len(command.Lines))
	for _, input := range command.Lines {
		productID := identity.NormalizeClaim(input.ProductID)
		if _, duplicate := seen[productID]; duplicate {
			return Document{}, sales.ErrDuplicateProductLine
		}
		seen[productID] = struct{}{}
		lineID, idErr := s.ids.New()
		if idErr != nil {
			return Document{}, idErr
		}
		amount, amountErr := checkedLineAmount(input.Quantity, input.UnitPriceMinor)
		if amountErr != nil {
			return Document{}, amountErr
		}
		document.Lines = append(document.Lines, DocumentLine{ID: lineID, ProductID: productID, Quantity: input.Quantity, UnitPriceMinor: input.UnitPriceMinor, AmountMinor: amount})
		document.SubtotalMinor, amountErr = checkedAdd(document.SubtotalMinor, amount)
		if amountErr != nil {
			return Document{}, amountErr
		}
	}
	document.TotalMinor = document.SubtotalMinor
	auditID, eventID, err := s.twoIDs()
	if err != nil {
		return Document{}, err
	}
	canonical, _ := json.Marshal(command)
	hash := sha256.Sum256(canonical)
	payload, _ := json.Marshal(document)
	return s.repository.CreateOperationDocument(ctx, document,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "operations.document_created", EntityType: "operation_document", EntityID: document.ID, CorrelationID: document.CorrelationID, CausationID: document.ID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "operation_document", AggregateID: document.ID, EventType: "operations.document_created", Version: 1, CorrelationID: document.CorrelationID, CausationID: document.ID, Payload: payload, OccurredAt: now},
		command.IdempotencyKey, hex.EncodeToString(hash[:]))
}

func (s *Service) Transition(ctx context.Context, command TransitionCommand) (Document, error) {
	command.Scope = command.Scope.Normalize()
	command.DocumentID, command.ActorID = identity.NormalizeClaim(command.DocumentID), identity.NormalizeClaim(command.ActorID)
	command.Reason, command.PaymentMethod = strings.TrimSpace(command.Reason), strings.ToUpper(strings.TrimSpace(command.PaymentMethod))
	command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.IdempotencyKey), identity.NormalizeClaim(command.CorrelationID)
	if command.Scope.Validate() != nil || !identity.IsUUID(command.DocumentID) || command.ActorID == "" ||
		!validStatus(command.ToStatus) || utf8.RuneCountInString(command.Reason) < 8 || utf8.RuneCountInString(command.Reason) > 500 ||
		len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 128 || (command.PaymentMethod != "" && !sales.IsCanonicalPaymentMethod(command.PaymentMethod)) {
		return Document{}, ErrInvalidCommand
	}
	effects, err := s.effectIDs(128)
	if err != nil {
		return Document{}, err
	}
	auditID, eventID, err := s.twoIDs()
	if err != nil {
		return Document{}, err
	}
	now := s.clock.Now().UTC()
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = effects.TransitionID
	}
	canonical, _ := json.Marshal(struct {
		DocumentID, ToStatus, Reason, PaymentMethod string
	}{command.DocumentID, string(command.ToStatus), command.Reason, command.PaymentMethod})
	hash := sha256.Sum256(canonical)
	payload, _ := json.Marshal(command)
	return s.repository.TransitionOperationDocument(ctx, command.Scope, command.ActorID, command.DocumentID, command.ToStatus,
		command.Reason, command.PaymentMethod, correlation, effects,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "operations.document_transitioned", EntityType: "operation_document", EntityID: command.DocumentID, CorrelationID: correlation, CausationID: effects.TransitionID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "operation_document", AggregateID: command.DocumentID, EventType: "operations.document_transitioned", Version: 1, CorrelationID: correlation, CausationID: effects.TransitionID, Payload: payload, OccurredAt: now},
		command.IdempotencyKey, hex.EncodeToString(hash[:]), now)
}

func (s *Service) Get(ctx context.Context, scope tenancy.Scope, actorID, documentID string) (Document, error) {
	return s.repository.OperationDocument(ctx, scope.Normalize(), identity.NormalizeClaim(actorID), identity.NormalizeClaim(documentID))
}

func (s *Service) List(ctx context.Context, scope tenancy.Scope, actorID string, kind DocumentType, cursor string, limit int) (Page, error) {
	if limit < 1 || limit > 200 {
		return Page{}, ErrInvalidCommand
	}
	return s.repository.ListOperationDocuments(ctx, scope.Normalize(), identity.NormalizeClaim(actorID), kind, identity.NormalizeClaim(cursor), limit)
}

func (s *Service) Suppliers(ctx context.Context, scope tenancy.Scope, actorID, cursor string, limit int) (SupplierPage, error) {
	if limit < 1 || limit > 200 {
		return SupplierPage{}, ErrInvalidCommand
	}
	return s.repository.ListSuppliers(ctx, scope.Normalize(), identity.NormalizeClaim(actorID), identity.NormalizeClaim(cursor), limit)
}

func validateCreate(command CreateCommand) error {
	if command.Scope.Validate() != nil || !validType(command.Type) || command.ActorID == "" || len(command.Currency) != 3 ||
		utf8.RuneCountInString(command.Reason) < 8 || utf8.RuneCountInString(command.Reason) > 500 || len(command.Lines) == 0 || len(command.Lines) > 60 ||
		len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 128 {
		return ErrInvalidCommand
	}
	if (command.PartyType == NoParty) != (command.PartyID == "") || (command.PartyID != "" && !identity.IsUUID(command.PartyID)) ||
		(command.SourceDocumentID != "" && !identity.IsUUID(command.SourceDocumentID)) ||
		(command.Type == StockTransfer) != (command.DestinationWarehouseID != "") {
		return ErrInvalidCommand
	}
	expectedParty:=NoParty
	switch command.Type { case Quotation,SalesOrder: expectedParty=CustomerParty; case PurchaseOrder,GoodsReceipt,SupplierInvoice,SupplierPayment,PurchaseReturn: expectedParty=SupplierParty }
	if command.PartyType!=expectedParty{return ErrInvalidCommand}
	requiresSource:=command.Type==GoodsReceipt||command.Type==SupplierInvoice||command.Type==SupplierPayment||command.Type==PurchaseReturn
	if requiresSource && command.SourceDocumentID==""{return ErrInvalidCommand}
	for _, line := range command.Lines {
		if !identity.IsUUID(identity.NormalizeClaim(line.ProductID)) || (line.Quantity == 0 && command.Type != StockCount) || !wire.IsSafeInteger(line.Quantity) ||
			line.UnitPriceMinor < 0 || !wire.IsSafeInteger(line.UnitPriceMinor) || (line.Quantity < 0 && command.Type != StockAdjustment) {
			return ErrInvalidCommand
		}
		if (command.Type == StockCount || command.Type == StockAdjustment || command.Type == StockTransfer) && line.UnitPriceMinor != 0 {
			return ErrInvalidCommand
		}
	}
	return nil
}

func validType(value DocumentType) bool {
	switch value {
	case Quotation, SalesOrder, PurchaseRequest, PurchaseOrder, GoodsReceipt, SupplierInvoice, SupplierPayment, PurchaseReturn, StockTransfer, StockCount, StockAdjustment:
		return true
	default:
		return false
	}
}

func validStatus(value Status) bool {
	switch value {
	case Submitted, Approved, Rejected, Posted, Dispatched, Received, Closed, Reversed:
		return true
	default:
		return false
	}
}

func (s *Service) effectIDs(count int) (EffectIDs, error) {
	values := make([]string, count)
	for index := range values {
		value, err := s.ids.New()
		if err != nil {
			return EffectIDs{}, err
		}
		values[index] = value
	}
	return EffectIDs{TransitionID: values[0], JournalID: values[1], LedgerID: values[2], PayableID: values[3], PaymentID: values[4], AllocationID: values[5], MovementIDs: values[6:67], ReserveIDs: values[67:]}, nil
}

func (s *Service) twoIDs() (string, string, error) {
	first, err := s.ids.New()
	if err != nil {
		return "", "", err
	}
	second, err := s.ids.New()
	return first, second, err
}

func documentNumber(kind DocumentType, id string) string {
	prefix := strings.ReplaceAll(string(kind), "_", "")
	token := strings.ReplaceAll(id, "-", "")
	if len(token) > 12 {
		token = token[:12]
	}
	return fmt.Sprintf("%s-%s", prefix, strings.ToUpper(token))
}

func checkedLineAmount(quantity, price int64) (int64, error) {
	absolute := quantity
	if absolute < 0 {
		absolute = -absolute
	}
	if absolute != 0 && price > wire.MaxSafeInteger/absolute {
		return 0, sales.ErrMoneyOverflow
	}
	return absolute * price, nil
}

func checkedAdd(left, right int64) (int64, error) {
	if right > wire.MaxSafeInteger-left {
		return 0, sales.ErrMoneyOverflow
	}
	return left + right, nil
}
