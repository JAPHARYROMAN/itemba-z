package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type DevicePage struct {
	Items      []devices.Device `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

type ChangeDeviceStatusCommand struct {
	Scope          tenancy.Scope
	ActorID        string
	DeviceID       string
	Status         devices.Status `json:"status"`
	Reason         string         `json:"reason"`
	IdempotencyKey string
	CorrelationID  string
}

type ChangeDeviceAllocationCommand struct {
	Scope             tenancy.Scope
	ActorID           string
	DeviceID          string
	ProductID         string `json:"product_id"`
	AllocatedQuantity int64  `json:"allocated_quantity"`
	Reason            string `json:"reason"`
	IdempotencyKey    string
	CorrelationID     string
}

type DeviceStatusChange struct {
	ID             string         `json:"id"`
	Scope          tenancy.Scope  `json:"scope"`
	DeviceID       string         `json:"device_id"`
	Status         devices.Status `json:"new_status"`
	PreviousStatus devices.Status `json:"previous_status"`
	Reason         string         `json:"reason"`
	ChangedBy      string         `json:"changed_by"`
	ChangedAt      time.Time      `json:"changed_at"`
	CorrelationID  string         `json:"correlation_id"`
	IdempotencyKey string         `json:"-"`
	RequestHash    string         `json:"-"`
}

type DeviceAllocationChange struct {
	ID                string        `json:"id"`
	Scope             tenancy.Scope `json:"scope"`
	DeviceID          string        `json:"device_id"`
	ProductID         string        `json:"product_id"`
	AllocatedQuantity int64         `json:"new_quantity"`
	PreviousQuantity  int64         `json:"previous_quantity"`
	ConsumedQuantity  int64         `json:"consumed_quantity"`
	Reason            string        `json:"reason"`
	ChangedBy         string        `json:"changed_by"`
	ChangedAt         time.Time     `json:"changed_at"`
	CorrelationID     string        `json:"correlation_id"`
	IdempotencyKey    string        `json:"-"`
	RequestHash       string        `json:"-"`
}

func (s *Service) ManagedDevices(ctx context.Context, scope tenancy.Scope, actorID, cursor string, pageSize int) (DevicePage, error) {
	scope = scope.Normalize()
	actorID = identity.NormalizeClaim(actorID)
	if scope.Validate() != nil || actorID == "" || pageSize < 1 || pageSize > 200 {
		return DevicePage{}, sales.ErrInvalidCommand
	}
	afterID, err := decodeDeviceCursor(cursor)
	if err != nil {
		return DevicePage{}, sales.ErrInvalidCommand
	}
	items, err := s.repository.ListManagedDevices(ctx, scope, actorID, afterID, pageSize+1)
	if err != nil {
		return DevicePage{}, err
	}
	if len(items) <= pageSize {
		return DevicePage{Items: items}, nil
	}
	next := encodeDeviceCursor(items[pageSize-1].ID)
	return DevicePage{Items: items[:pageSize], NextCursor: &next}, nil
}

func (s *Service) ChangeDeviceStatus(ctx context.Context, command ChangeDeviceStatusCommand) (devices.Device, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.DeviceID = identity.NormalizeClaim(command.DeviceID)
	command.Status = devices.Status(strings.ToUpper(strings.TrimSpace(string(command.Status))))
	command.Reason = strings.TrimSpace(command.Reason)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.CorrelationID = identity.NormalizeClaim(command.CorrelationID)
	if command.Scope.Validate() != nil || command.ActorID == "" || !identity.IsUUID(command.DeviceID) ||
		(command.Status != devices.StatusActive && command.Status != devices.StatusSuspended) ||
		utf8.RuneCountInString(command.Reason) < 8 || utf8.RuneCountInString(command.Reason) > 500 ||
		len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 128 {
		return devices.Device{}, sales.ErrInvalidCommand
	}
	return s.changeDeviceStatus(ctx, command)
}

func (s *Service) changeDeviceStatus(ctx context.Context, command ChangeDeviceStatusCommand) (devices.Device, error) {
	canonical, err := json.Marshal(struct {
		DeviceID string         `json:"device_id"`
		Status   devices.Status `json:"status"`
		Reason   string         `json:"reason"`
	}{command.DeviceID, command.Status, command.Reason})
	if err != nil {
		return devices.Device{}, err
	}
	hash := sha256.Sum256(canonical)
	changeID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	auditID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	eventID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	now := s.clock.Now().UTC()
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = changeID
	}
	change := DeviceStatusChange{ID: changeID, Scope: command.Scope, DeviceID: command.DeviceID, Status: command.Status,
		Reason: command.Reason, ChangedBy: command.ActorID, ChangedAt: now, CorrelationID: correlation,
		IdempotencyKey: command.IdempotencyKey, RequestHash: hex.EncodeToString(hash[:])}
	payload, _ := json.Marshal(change)
	return s.repository.ChangeManagedDeviceStatus(ctx, change,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "mobile.device.status_changed", EntityType: "mobile_device", EntityID: command.DeviceID, CorrelationID: correlation, CausationID: changeID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "mobile_device", AggregateID: command.DeviceID, EventType: "mobile.device.status_changed", Version: 1, CorrelationID: correlation, CausationID: changeID, Payload: payload, OccurredAt: now})
}

func (s *Service) ChangeDeviceAllocation(ctx context.Context, command ChangeDeviceAllocationCommand) (devices.Device, error) {
	command.Scope = command.Scope.Normalize()
	command.ActorID = identity.NormalizeClaim(command.ActorID)
	command.DeviceID = identity.NormalizeClaim(command.DeviceID)
	command.ProductID = identity.NormalizeClaim(command.ProductID)
	command.Reason = strings.TrimSpace(command.Reason)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.CorrelationID = identity.NormalizeClaim(command.CorrelationID)
	if command.Scope.Validate() != nil || command.ActorID == "" || !identity.IsUUID(command.DeviceID) || !identity.IsUUID(command.ProductID) ||
		command.AllocatedQuantity < 0 || !wire.IsSafeInteger(command.AllocatedQuantity) ||
		utf8.RuneCountInString(command.Reason) < 8 || utf8.RuneCountInString(command.Reason) > 500 ||
		len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 128 {
		return devices.Device{}, sales.ErrInvalidCommand
	}
	canonical, err := json.Marshal(struct {
		DeviceID          string `json:"device_id"`
		ProductID         string `json:"product_id"`
		AllocatedQuantity int64  `json:"allocated_quantity"`
		Reason            string `json:"reason"`
	}{command.DeviceID, command.ProductID, command.AllocatedQuantity, command.Reason})
	if err != nil {
		return devices.Device{}, err
	}
	hash := sha256.Sum256(canonical)
	changeID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	auditID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	eventID, err := s.ids.New()
	if err != nil {
		return devices.Device{}, err
	}
	now := s.clock.Now().UTC()
	correlation := command.CorrelationID
	if correlation == "" {
		correlation = changeID
	}
	change := DeviceAllocationChange{ID: changeID, Scope: command.Scope, DeviceID: command.DeviceID,
		ProductID: command.ProductID, AllocatedQuantity: command.AllocatedQuantity, Reason: command.Reason,
		ChangedBy: command.ActorID, ChangedAt: now, CorrelationID: correlation,
		IdempotencyKey: command.IdempotencyKey, RequestHash: hex.EncodeToString(hash[:])}
	payload, _ := json.Marshal(change)
	return s.repository.ChangeManagedDeviceAllocation(ctx, change,
		audit.Event{ID: auditID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, ActorID: command.ActorID, Action: "mobile.device.allocation_changed", EntityType: "mobile_device", EntityID: command.DeviceID, CorrelationID: correlation, CausationID: changeID, Data: payload, OccurredAt: now},
		outbox.Event{ID: eventID, TenantID: command.Scope.TenantID, CompanyID: command.Scope.CompanyID, AggregateType: "mobile_device", AggregateID: command.DeviceID, EventType: "mobile.device.allocation_changed", Version: 1, CorrelationID: correlation, CausationID: changeID, Payload: payload, OccurredAt: now})
}

func encodeDeviceCursor(id string) string { return base64.RawURLEncoding.EncodeToString([]byte(id)) }

func decodeDeviceCursor(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || !identity.IsUUID(string(decoded)) {
		return "", sales.ErrInvalidCommand
	}
	return strings.ToLower(string(decoded)), nil
}
