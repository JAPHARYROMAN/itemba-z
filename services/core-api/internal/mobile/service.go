package mobile

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
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type EnrollCommand struct {
	Scope                         tenancy.Scope
	ActorID                       string
	DeviceID                      string  `json:"device_id"`
	DeviceName                    string  `json:"device_name"`
	AppVersion                    string  `json:"app_version"`
	InstalledMasterDataVersion    *int64  `json:"installed_master_data_version,omitempty"`
	InstalledPriceVersion         *int64  `json:"installed_price_version,omitempty"`
	InstalledCatalogSnapshotToken *string `json:"installed_catalog_snapshot_token,omitempty"`
	CorrelationID                 string
}

type SyncCommand struct {
	CustomerID           string              `json:"customer_id"`
	Kind                 sales.Kind          `json:"kind"`
	PaymentMethod        string              `json:"payment_method,omitempty"`
	Lines                []sales.CommandLine `json:"lines"`
	DeviceID             string              `json:"device_id"`
	ClientTransactionID  string              `json:"client_transaction_id"`
	ClientTimestamp      time.Time           `json:"client_timestamp"`
	AppVersion           string              `json:"app_version"`
	MasterDataVersion    int64               `json:"master_data_version"`
	PriceVersion         int64               `json:"price_version"`
	CatalogSnapshotToken string              `json:"catalog_snapshot_token"`
	SyncAttempt          int                 `json:"sync_attempt"`
	Offline              bool                `json:"offline"`
}

type SyncResult struct {
	ClientTransactionID string             `json:"client_transaction_id"`
	State               string             `json:"state"`
	ReceiptReference    string             `json:"receipt_reference"`
	FiscalStatus        sales.FiscalStatus `json:"fiscal_status"`
	IdempotentReplay    bool               `json:"idempotent_replay"`
	Sale                sales.Sale         `json:"sale"`
}

type Repository interface {
	EnrollDevice(ctx context.Context, device devices.Device, acknowledgement *devices.InstallAcknowledgement, enrollmentAudit audit.Event, enrollmentEvent outbox.Event) (devices.Device, error)
	RecordReconciliationCase(ctx context.Context, value ReconciliationCase, caseAudit audit.Event, caseEvent outbox.Event) (ReconciliationCase, error)
	ListReconciliationCases(ctx context.Context, scope tenancy.Scope, actorID string, status ReconciliationStatus, afterID string, limit int) ([]ReconciliationCase, error)
	ReconciliationCase(ctx context.Context, scope tenancy.Scope, actorID, caseID string) (ReconciliationCase, error)
	ResolveReconciliationCase(ctx context.Context, scope tenancy.Scope, actorID string, resolution ReconciliationResolution, resolutionAudit audit.Event, resolutionEvent outbox.Event) (ReconciliationCase, error)
}

type Service struct {
	repository Repository
	sales      *sales.Service
	ids        identity.Generator
	clock      clock.Clock
}

func NewService(repository Repository, salesService *sales.Service, ids identity.Generator, currentTime clock.Clock) (*Service, error) {
	if repository == nil || salesService == nil || ids == nil || currentTime == nil {
		return nil, errors.New("mobile service dependencies are required")
	}
	return &Service{repository: repository, sales: salesService, ids: ids, clock: currentTime}, nil
}

func (s *Service) Enroll(ctx context.Context, command EnrollCommand) (devices.Device, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.DeviceID = identity.NormalizeClaim(command.DeviceID)
	command.DeviceName = strings.TrimSpace(command.DeviceName)
	command.AppVersion = strings.TrimSpace(command.AppVersion)
	hasMasterAck, hasPriceAck := command.InstalledMasterDataVersion != nil, command.InstalledPriceVersion != nil
	hasSnapshotAck := command.InstalledCatalogSnapshotToken != nil
	if command.Scope.Validate() != nil || command.ActorID == "" || !identity.IsUUID(command.DeviceID) || command.DeviceName == "" || utf8.RuneCountInString(command.DeviceName) > 200 || command.AppVersion == "" || utf8.RuneCountInString(command.AppVersion) > 64 || hasMasterAck != hasPriceAck {
		return devices.Device{}, sales.ErrInvalidCommand
	}
	if hasMasterAck != hasSnapshotAck {
		return devices.Device{}, sales.ErrInvalidCommand
	}
	var acknowledgement *devices.InstallAcknowledgement
	if hasMasterAck {
		if *command.InstalledMasterDataVersion < 1 || *command.InstalledPriceVersion < 1 {
			return devices.Device{}, sales.ErrInvalidCommand
		}
		if *command.InstalledMasterDataVersion > sales.MaxWireSafeInteger || *command.InstalledPriceVersion > sales.MaxWireSafeInteger {
			return devices.Device{}, sales.ErrUnsafeWireInteger
		}
		token := identity.NormalizeClaim(*command.InstalledCatalogSnapshotToken)
		if !identity.IsUUID(token) || token == devices.UnacknowledgedCatalogSnapshotToken {
			return devices.Device{}, sales.ErrInvalidCommand
		}
		acknowledgement = &devices.InstallAcknowledgement{MasterDataVersion: *command.InstalledMasterDataVersion, PriceVersion: *command.InstalledPriceVersion, CatalogSnapshotToken: token}
	}
	now := s.clock.Now().UTC()
	device := devices.Device{ID: command.DeviceID, Status: devices.StatusActive, ActorID: command.ActorID, Scope: command.Scope, Name: command.DeviceName, AppVersion: command.AppVersion, CatalogSnapshotToken: devices.UnacknowledgedCatalogSnapshotToken, EnrolledAt: now, LastSeenAt: now}
	payload, _ := json.Marshal(device)
	auditID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	eventID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = auditID
	}
	result, err := s.repository.EnrollDevice(ctx, device, acknowledgement, audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "mobile.device.enrolled", EntityType: "mobile_device", EntityID: device.ID, CorrelationID: correlation, CausationID: device.ID, Data: payload, OccurredAt: now}, outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "mobile_device", AggregateID: device.ID, EventType: "mobile.device.enrolled", Version: 1, CorrelationID: correlation, CausationID: device.ID, Payload: payload, OccurredAt: now})
	if err != nil {
		return devices.Device{}, err
	}
	if err := devices.ValidateWireSafe(result); err != nil {
		return devices.Device{}, err
	}
	return result, nil
}

func (s *Service) SyncSale(ctx context.Context, scope tenancy.Scope, actorID, correlationID string, command SyncCommand) (SyncResult, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	correlationID = identity.NormalizeClaim(correlationID)
	command.DeviceID = identity.NormalizeClaim(command.DeviceID)
	command.ClientTransactionID = identity.NormalizeClaim(command.ClientTransactionID)
	command.CustomerID = identity.NormalizeClaim(command.CustomerID)
	command.AppVersion = strings.TrimSpace(command.AppVersion)
	command.CatalogSnapshotToken = identity.NormalizeClaim(command.CatalogSnapshotToken)
	command.Lines = append([]sales.CommandLine(nil), command.Lines...)
	for index := range command.Lines {
		command.Lines[index].ProductID = identity.NormalizeClaim(command.Lines[index].ProductID)
	}
	if scope.Validate() != nil || actorID == "" || !identity.IsUUID(command.DeviceID) || !identity.IsUUID(command.ClientTransactionID) || command.SyncAttempt < 1 || command.ClientTimestamp.IsZero() || command.MasterDataVersion < 1 || command.PriceVersion < 1 || !identity.IsUUID(command.CatalogSnapshotToken) || command.CatalogSnapshotToken == devices.UnacknowledgedCatalogSnapshotToken || command.AppVersion == "" || utf8.RuneCountInString(command.AppVersion) > 64 {
		return SyncResult{}, sales.ErrInvalidCommand
	}
	if !identity.IsUUID(command.CustomerID) {
		return SyncResult{}, sales.ErrInvalidCommand
	}
	for _, line := range command.Lines {
		if !identity.IsUUID(line.ProductID) {
			return SyncResult{}, sales.ErrInvalidCommand
		}
	}
	// Credit remains a Control Center-only workflow until mobile can obtain and
	// prove an authoritative AR-aging and approval decision. Being online is not
	// sufficient: forwarding the command would let the generic posting service
	// apply only its basic customer-limit checks.
	if command.Kind == sales.KindCredit {
		return SyncResult{}, devices.ErrMobileCreditUnsupported
	}
	created, err := s.sales.Complete(ctx, sales.CompleteCommand{Scope: scope, CustomerID: command.CustomerID, Kind: command.Kind, PaymentMethod: command.PaymentMethod, Lines: command.Lines, ActorID: actorID, CorrelationID: correlationID, IdempotencyKey: command.DeviceID + "::" + command.ClientTransactionID, DeviceID: command.DeviceID, ClientTransactionID: command.ClientTransactionID, ClientTimestamp: command.ClientTimestamp, AppVersion: command.AppVersion, MasterDataVersion: command.MasterDataVersion, PriceVersion: command.PriceVersion, CatalogSnapshotToken: command.CatalogSnapshotToken, SyncAttempt: command.SyncAttempt, Offline: command.Offline})
	if err != nil {
		failureCode := reconciliationFailureCode(err)
		if command.Offline && failureCode != "" {
			if recordErr := s.recordReconciliationCase(ctx, scope, actorID, correlationID, command, failureCode); recordErr != nil {
				return SyncResult{}, recordErr
			}
		}
		return SyncResult{}, err
	}
	return SyncResult{ClientTransactionID: command.ClientTransactionID, State: "synced", ReceiptReference: created.ReceiptReference, FiscalStatus: created.FiscalStatus, IdempotentReplay: created.IdempotentReplay, Sale: created}, nil
}

func (s *Service) recordReconciliationCase(ctx context.Context, scope tenancy.Scope, actorID, correlationID string, command SyncCommand, failureCode string) error {
	stable := command
	stable.SyncAttempt = 0
	canonical, err := json.Marshal(stable)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(canonical)
	evidence, err := json.Marshal(command)
	if err != nil {
		return err
	}
	caseID, err := s.ids.New()
	if err != nil {
		return err
	}
	auditID, err := s.ids.New()
	if err != nil {
		return err
	}
	eventID, err := s.ids.New()
	if err != nil {
		return err
	}
	if correlationID == "" {
		correlationID = caseID
	}
	now := s.clock.Now().UTC()
	value := ReconciliationCase{
		ID: caseID, Scope: scope, Status: ReconciliationOpen, DeviceID: command.DeviceID,
		ClientTransactionID: command.ClientTransactionID, ClientTimestamp: command.ClientTimestamp.UTC(),
		AppVersion: command.AppVersion, MasterDataVersion: command.MasterDataVersion,
		PriceVersion: command.PriceVersion, CatalogSnapshotToken: command.CatalogSnapshotToken,
		FailureCode: failureCode, Command: evidence,
		CommandHash: hex.EncodeToString(hash[:]), CreatedBy: actorID,
		CorrelationID: correlationID, CreatedAt: now,
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.repository.RecordReconciliationCase(ctx, value,
		audit.Event{ID: auditID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, ActorID: actorID, Action: "mobile.reconciliation.opened", EntityType: "mobile_reconciliation_case", EntityID: caseID, CorrelationID: correlationID, CausationID: command.ClientTransactionID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, AggregateType: "mobile_reconciliation_case", AggregateID: caseID, EventType: "mobile.reconciliation.opened", Version: 1, CorrelationID: correlationID, CausationID: command.ClientTransactionID, Payload: payload, OccurredAt: now})
	return err
}

func reconciliationFailureCode(err error) string {
	switch {
	case errors.Is(err, sales.ErrOfflineReconciliation):
		return "offline_reconciliation_required"
	case errors.Is(err, sales.ErrOfflinePeriodReconciliation):
		return "offline_fiscal_period_reconciliation_required"
	case errors.Is(err, sales.ErrOfflineClockReconciliation):
		return "offline_clock_reconciliation_required"
	default:
		return ""
	}
}
