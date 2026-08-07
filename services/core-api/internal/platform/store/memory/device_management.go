package memory

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	deviceStatusOperation     = "mobile.device.status.change.v1"
	deviceAllocationOperation = "mobile.device.allocation.change.v1"
)

func (s *Store) ListManagedDevices(_ context.Context, scope tenancy.Scope, actorID, afterID string, limit int) ([]devices.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "mobile.devices.read")] {
		return nil, sales.ErrForbidden
	}
	items := make([]devices.Device, 0)
	for _, value := range s.state.devices {
		if value.Scope == scope && value.ID > afterID {
			items = append(items, managedMemoryDevice(s.state, value))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *Store) ChangeManagedDeviceStatus(_ context.Context, change mobile.DeviceStatusChange, changeAudit audit.Event, changeEvent outbox.Event) (devices.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(change.Scope, change.ChangedBy, "mobile.devices.manage")] {
		return devices.Device{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(change.Scope, deviceStatusOperation, change.IdempotencyKey)
	if found, ok := s.state.idempotencies[idemKey]; ok {
		if found.RequestHash != change.RequestHash {
			return devices.Device{}, sales.ErrIdempotencyConflict
		}
		value, ok := s.state.devices[deviceKey(change.Scope.TenantID, change.DeviceID)]
		if !ok || value.Scope != change.Scope {
			return devices.Device{}, devices.ErrNotEnrolled
		}
		return managedMemoryDevice(s.state, value), nil
	}
	key := deviceKey(change.Scope.TenantID, change.DeviceID)
	value, ok := s.state.devices[key]
	if !ok || value.Scope != change.Scope {
		return devices.Device{}, devices.ErrNotEnrolled
	}
	if value.Status == devices.StatusRevoked || (value.Status != devices.StatusActive && value.Status != devices.StatusSuspended) {
		return devices.Device{}, devices.ErrInvalidStatusTransition
	}
	if value.Status == change.Status {
		return devices.Device{}, devices.ErrStatusUnchanged
	}
	change.PreviousStatus = value.Status
	value.Status = change.Status
	if change.Status == devices.StatusSuspended {
		value.AuthorizationEpoch = change.ID
		value.OfflineSalesValidUntil = change.ChangedAt
		if value.OfflineSalesValidUntil.Before(value.OfflineSalesValidFrom) {
			value.OfflineSalesValidUntil = value.OfflineSalesValidFrom
		}
	}
	payload, err := json.Marshal(change)
	if err != nil {
		return devices.Device{}, err
	}
	changeAudit.Data, changeEvent.Payload = payload, payload
	s.state.devices[key] = value
	s.state.audits = append(s.state.audits, changeAudit)
	s.state.outbox = append(s.state.outbox, changeEvent)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: change.RequestHash, ResultID: change.DeviceID}
	return managedMemoryDevice(s.state, value), nil
}

func (s *Store) ChangeManagedDeviceAllocation(_ context.Context, change mobile.DeviceAllocationChange, changeAudit audit.Event, changeEvent outbox.Event) (devices.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(change.Scope, change.ChangedBy, "mobile.devices.manage")] {
		return devices.Device{}, sales.ErrForbidden
	}
	idemKey := idempotencyKey(change.Scope, deviceAllocationOperation, change.IdempotencyKey)
	if found, ok := s.state.idempotencies[idemKey]; ok {
		if found.RequestHash != change.RequestHash {
			return devices.Device{}, sales.ErrIdempotencyConflict
		}
		value, ok := s.state.devices[deviceKey(change.Scope.TenantID, change.DeviceID)]
		if !ok || value.Scope != change.Scope {
			return devices.Device{}, devices.ErrNotEnrolled
		}
		return managedMemoryDevice(s.state, value), nil
	}
	deviceKeyValue := deviceKey(change.Scope.TenantID, change.DeviceID)
	value, ok := s.state.devices[deviceKeyValue]
	if !ok || value.Scope != change.Scope {
		return devices.Device{}, devices.ErrNotEnrolled
	}
	if _, ok := s.state.products[companyEntityKey(change.Scope.TenantID, change.Scope.CompanyID, change.ProductID)]; !ok {
		return devices.Device{}, sales.ErrNotFound
	}
	change.PreviousQuantity = s.state.allocations[allocationKey(change.Scope, change.DeviceID, change.ProductID)]
	change.ConsumedQuantity = consumedOfflineQuantity(s.state, change.Scope, change.DeviceID, change.ProductID)
	if change.AllocatedQuantity < change.ConsumedQuantity {
		return devices.Device{}, devices.ErrAllocationBelowConsumed
	}
	reserved := change.AllocatedQuantity - change.ConsumedQuantity
	for _, other := range s.state.devices {
		if other.Scope != change.Scope || other.ID == change.DeviceID {
			continue
		}
		allocated := s.state.allocations[allocationKey(change.Scope, other.ID, change.ProductID)]
		consumed := consumedOfflineQuantity(s.state, change.Scope, other.ID, change.ProductID)
		if allocated > consumed {
			reserved += allocated - consumed
		}
	}
	if reserved > availableStock(s.state, change.Scope, change.ProductID) {
		return devices.Device{}, devices.ErrAllocationOvercommitted
	}
	payload, err := json.Marshal(change)
	if err != nil {
		return devices.Device{}, err
	}
	changeAudit.Data, changeEvent.Payload = payload, payload
	s.state.allocations[allocationKey(change.Scope, change.DeviceID, change.ProductID)] = change.AllocatedQuantity
	s.state.audits = append(s.state.audits, changeAudit)
	s.state.outbox = append(s.state.outbox, changeEvent)
	s.state.idempotencies[idemKey] = idempotency{RequestHash: change.RequestHash, ResultID: change.DeviceID}
	return managedMemoryDevice(s.state, value), nil
}

func managedMemoryDevice(current *state, value devices.Device) devices.Device {
	if working, ok := current.contexts[scopeKey(value.Scope)]; ok {
		value.AvailableMasterDataVersion = working.MasterDataVersion
		value.AvailablePriceVersion = working.PriceVersion
		value.AvailableCatalogSnapshotToken = working.CatalogSnapshotToken
		value.TimeZone = working.TimeZone
	}
	return withAllocations(current, value)
}

func consumedOfflineQuantity(current *state, scope tenancy.Scope, deviceID, productID string) int64 {
	var quantity int64
	for _, sale := range current.sales {
		if sale.Scope != scope || sale.DeviceID != deviceID || !sale.Offline || sale.RecordType != sales.RecordSale || sale.Status != sales.StatusPosted {
			continue
		}
		for _, line := range sale.Lines {
			if line.ProductID == productID {
				quantity += line.Quantity
			}
		}
	}
	return quantity
}
