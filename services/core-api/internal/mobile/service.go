package mobile

import (
	"context"
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
	Scope                      tenancy.Scope
	ActorID                    string
	DeviceID                   string `json:"device_id"`
	DeviceName                 string `json:"device_name"`
	AppVersion                 string `json:"app_version"`
	InstalledMasterDataVersion *int64 `json:"installed_master_data_version,omitempty"`
	InstalledPriceVersion      *int64 `json:"installed_price_version,omitempty"`
	CorrelationID              string
}

type SyncCommand struct {
	CustomerID          string              `json:"customer_id"`
	Kind                sales.Kind          `json:"kind"`
	PaymentMethod       string              `json:"payment_method,omitempty"`
	Lines               []sales.CommandLine `json:"lines"`
	DeviceID            string              `json:"device_id"`
	ClientTransactionID string              `json:"client_transaction_id"`
	ClientTimestamp     time.Time           `json:"client_timestamp"`
	AppVersion          string              `json:"app_version"`
	MasterDataVersion   int64               `json:"master_data_version"`
	PriceVersion        int64               `json:"price_version"`
	SyncAttempt         int                 `json:"sync_attempt"`
	Offline             bool                `json:"offline"`
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
	if command.Scope.Validate() != nil || command.ActorID == "" || !identity.IsUUID(command.DeviceID) || command.DeviceName == "" || utf8.RuneCountInString(command.DeviceName) > 200 || command.AppVersion == "" || utf8.RuneCountInString(command.AppVersion) > 64 || hasMasterAck != hasPriceAck {
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
		acknowledgement = &devices.InstallAcknowledgement{MasterDataVersion: *command.InstalledMasterDataVersion, PriceVersion: *command.InstalledPriceVersion}
	}
	now := s.clock.Now().UTC()
	device := devices.Device{ID: command.DeviceID, Status: devices.StatusActive, ActorID: command.ActorID, Scope: command.Scope, Name: command.DeviceName, AppVersion: command.AppVersion, EnrolledAt: now, LastSeenAt: now}
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
	command.Lines = append([]sales.CommandLine(nil), command.Lines...)
	for index := range command.Lines {
		command.Lines[index].ProductID = identity.NormalizeClaim(command.Lines[index].ProductID)
	}
	if scope.Validate() != nil || actorID == "" || !identity.IsUUID(command.DeviceID) || !identity.IsUUID(command.ClientTransactionID) || command.SyncAttempt < 1 || command.ClientTimestamp.IsZero() || command.MasterDataVersion < 1 || command.PriceVersion < 1 || command.AppVersion == "" || utf8.RuneCountInString(command.AppVersion) > 64 {
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
	created, err := s.sales.Complete(ctx, sales.CompleteCommand{Scope: scope, CustomerID: command.CustomerID, Kind: command.Kind, PaymentMethod: command.PaymentMethod, Lines: command.Lines, ActorID: actorID, CorrelationID: correlationID, IdempotencyKey: command.DeviceID + "::" + command.ClientTransactionID, DeviceID: command.DeviceID, ClientTransactionID: command.ClientTransactionID, ClientTimestamp: command.ClientTimestamp, AppVersion: command.AppVersion, MasterDataVersion: command.MasterDataVersion, PriceVersion: command.PriceVersion, SyncAttempt: command.SyncAttempt, Offline: command.Offline})
	if err != nil {
		return SyncResult{}, err
	}
	return SyncResult{ClientTransactionID: command.ClientTransactionID, State: "synced", ReceiptReference: created.ReceiptReference, FiscalStatus: created.FiscalStatus, IdempotentReplay: created.IdempotentReplay, Sale: created}, nil
}
