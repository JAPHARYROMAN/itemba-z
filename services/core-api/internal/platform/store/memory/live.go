package memory

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func (s *Store) EnrollDevice(_ context.Context, value devices.Device, acknowledgement *devices.InstallAcknowledgement, enrollmentAudit audit.Event, enrollmentEvent outbox.Event) (devices.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(value.Scope, value.ActorID, "mobile.devices.enroll")] {
		return devices.Device{}, sales.ErrForbidden
	}
	key := deviceKey(value.Scope.TenantID, value.ID)
	if value.AuthorizationEpoch == "" {
		value.AuthorizationEpoch = "enrolled:" + value.ID
	}
	workingContext, contextFound := s.state.contexts[scopeKey(value.Scope)]
	if !contextFound {
		return devices.Device{}, sales.ErrNotFound
	}
	value.AvailableMasterDataVersion = workingContext.MasterDataVersion
	value.AvailablePriceVersion = workingContext.PriceVersion
	value.AvailableCatalogSnapshotToken = workingContext.CatalogSnapshotToken
	value.TimeZone = workingContext.TimeZone
	leaseUntil := offlineLeaseUntil(s.state, value.Scope, value.LastSeenAt)
	if acknowledgement != nil && (acknowledgement.MasterDataVersion != workingContext.MasterDataVersion || acknowledgement.PriceVersion != workingContext.PriceVersion || acknowledgement.CatalogSnapshotToken != workingContext.CatalogSnapshotToken) {
		return devices.Device{}, devices.ErrStaleMasterData
	}
	if err := devices.ValidateWireSafe(value); err != nil {
		return devices.Device{}, err
	}
	if acknowledgement != nil {
		captureCatalogPublication(s.state, value.Scope, acknowledgement.CatalogSnapshotToken)
	}
	if existing, ok := s.state.devices[key]; ok {
		if existing.Scope != value.Scope || existing.ActorID != value.ActorID {
			return devices.Device{}, devices.ErrScopeMismatch
		}
		if acknowledgement != nil {
			sameInstallation := existing.AppVersion == value.AppVersion &&
				existing.MasterDataVersion == acknowledgement.MasterDataVersion &&
				existing.PriceVersion == acknowledgement.PriceVersion
			sameInstallation = sameInstallation && existing.CatalogSnapshotToken == acknowledgement.CatalogSnapshotToken
			liveAuthorization := !existing.OfflineEnabled ||
				(!existing.OfflineSalesValidFrom.After(value.LastSeenAt) && existing.OfflineSalesValidUntil.After(value.LastSeenAt))
			if sameInstallation && liveAuthorization {
				existing.AvailableMasterDataVersion = workingContext.MasterDataVersion
				existing.AvailablePriceVersion = workingContext.PriceVersion
				existing.AvailableCatalogSnapshotToken = workingContext.CatalogSnapshotToken
				existing.TimeZone = workingContext.TimeZone
				result := withAllocations(s.state, existing)
				if err := devices.ValidateWireSafe(result); err != nil {
					return devices.Device{}, err
				}
				return result, nil
			}

			existing.Name = value.Name
			if value.LastSeenAt.After(existing.LastSeenAt) {
				existing.LastSeenAt = value.LastSeenAt
			}
			existing.AppVersion = value.AppVersion
			existing.MasterDataVersion = acknowledgement.MasterDataVersion
			existing.PriceVersion = acknowledgement.PriceVersion
			existing.CatalogSnapshotToken = acknowledgement.CatalogSnapshotToken
			existing.OfflineSalesValidFrom = value.LastSeenAt
			existing.OfflineSalesValidUntil = value.LastSeenAt
			if existing.OfflineEnabled && existing.Status == devices.StatusActive {
				existing.OfflineSalesValidUntil = leaseUntil
				appendOfflineLease(s.state, devices.OfflineLease{
					Scope: existing.Scope, DeviceID: existing.ID, AppVersion: existing.AppVersion,
					MasterDataVersion: existing.MasterDataVersion, PriceVersion: existing.PriceVersion,
					CatalogSnapshotToken: existing.CatalogSnapshotToken,
					AuthorizationEpoch:   existing.AuthorizationEpoch,
					ValidFrom:            existing.OfflineSalesValidFrom, ValidUntil: existing.OfflineSalesValidUntil,
				})
			}
		} else {
			existing.Name = value.Name
			if value.LastSeenAt.After(existing.LastSeenAt) {
				existing.LastSeenAt = value.LastSeenAt
			}
		}
		existing.AvailableMasterDataVersion = workingContext.MasterDataVersion
		existing.AvailablePriceVersion = workingContext.PriceVersion
		existing.AvailableCatalogSnapshotToken = workingContext.CatalogSnapshotToken
		existing.TimeZone = workingContext.TimeZone
		result := withAllocations(s.state, existing)
		if err := devices.ValidateWireSafe(result); err != nil {
			return devices.Device{}, err
		}
		if acknowledgement != nil {
			payload, err := json.Marshal(result)
			if err != nil {
				return devices.Device{}, err
			}
			enrollmentAudit.Action = "mobile.device.installation_acknowledged"
			enrollmentAudit.Data = payload
			enrollmentEvent.EventType = "mobile.device.installation_acknowledged"
			enrollmentEvent.Payload = payload
			s.state.audits = append(s.state.audits, enrollmentAudit)
			s.state.outbox = append(s.state.outbox, enrollmentEvent)
		}
		s.state.devices[key] = existing
		return result, nil
	}
	if acknowledgement != nil {
		value.MasterDataVersion = acknowledgement.MasterDataVersion
		value.PriceVersion = acknowledgement.PriceVersion
		value.CatalogSnapshotToken = acknowledgement.CatalogSnapshotToken
		value.OfflineSalesValidFrom = value.LastSeenAt
		value.OfflineSalesValidUntil = value.LastSeenAt
		if value.OfflineEnabled {
			value.OfflineSalesValidUntil = leaseUntil
			appendOfflineLease(s.state, devices.OfflineLease{
				Scope: value.Scope, DeviceID: value.ID, AppVersion: value.AppVersion,
				MasterDataVersion: value.MasterDataVersion, PriceVersion: value.PriceVersion,
				CatalogSnapshotToken: value.CatalogSnapshotToken,
				AuthorizationEpoch:   value.AuthorizationEpoch,
				ValidFrom:            value.OfflineSalesValidFrom, ValidUntil: value.OfflineSalesValidUntil,
			})
		}
	} else {
		value.OfflineSalesValidFrom = value.LastSeenAt
		value.OfflineSalesValidUntil = value.LastSeenAt
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return devices.Device{}, err
	}
	enrollmentAudit.Data = payload
	enrollmentEvent.Payload = payload
	result := withAllocations(s.state, value)
	if err := devices.ValidateWireSafe(result); err != nil {
		return devices.Device{}, err
	}
	s.state.devices[key] = value
	s.state.audits = append(s.state.audits, enrollmentAudit)
	s.state.outbox = append(s.state.outbox, enrollmentEvent)
	return result, nil
}

func appendOfflineLease(current *state, lease devices.OfflineLease) {
	for _, existing := range current.offlineLeases {
		if existing.Scope == lease.Scope && existing.DeviceID == lease.DeviceID &&
			existing.AppVersion == lease.AppVersion && existing.MasterDataVersion == lease.MasterDataVersion &&
			existing.PriceVersion == lease.PriceVersion && existing.CatalogSnapshotToken == lease.CatalogSnapshotToken && existing.ValidFrom.Equal(lease.ValidFrom) &&
			existing.ValidUntil.Equal(lease.ValidUntil) {
			return
		}
	}
	current.offlineLeases = append(current.offlineLeases, lease)
}

func offlineLeaseUntil(current *state, scope tenancy.Scope, issuedAt time.Time) time.Time {
	result := issuedAt.Add(devices.OfflineSalesLeaseDuration)
	for _, rate := range current.taxRates {
		if rate.TenantID != scope.TenantID || rate.CompanyID != scope.CompanyID {
			continue
		}
		for _, transition := range []*time.Time{&rate.EffectiveFrom, rate.EffectiveTo} {
			if transition != nil && transition.After(issuedAt) && transition.Before(result) {
				result = *transition
			}
		}
	}
	return result.UTC()
}

func (s *Store) WorkingContext(_ context.Context, scope tenancy.Scope, actorID string) (readmodel.WorkingContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	permissions := permissionsFor(s.state, scope, actorID)
	if len(permissions) == 0 {
		return readmodel.WorkingContext{}, sales.ErrForbidden
	}
	value, ok := s.state.contexts[scopeKey(scope)]
	if !ok {
		return readmodel.WorkingContext{}, sales.ErrNotFound
	}
	value.ActorID = actorID
	value.Permissions = permissions
	return value, nil
}

func (s *Store) ListCustomers(_ context.Context, scope tenancy.Scope, actorID string, options readmodel.ListOptions) (readmodel.CatalogSnapshot, []readmodel.CustomerSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "customers.read")] {
		return readmodel.CatalogSnapshot{}, nil, sales.ErrForbidden
	}
	workingContext, ok := s.state.contexts[scopeKey(scope)]
	if !ok {
		return readmodel.CatalogSnapshot{}, nil, sales.ErrNotFound
	}
	snapshot := readmodel.CatalogSnapshot{Token: workingContext.CatalogSnapshotToken, MasterDataVersion: workingContext.MasterDataVersion, PriceVersion: workingContext.PriceVersion}
	if options.CatalogSnapshotToken != "" && options.CatalogSnapshotToken != snapshot.Token {
		return readmodel.CatalogSnapshot{}, nil, devices.ErrStaleMasterData
	}
	captureCatalogPublication(s.state, scope, snapshot.Token)
	query := strings.ToLower(options.Query)
	items := make([]readmodel.CustomerSummary, 0)
	for _, account := range s.state.customers {
		if account.TenantID != scope.TenantID || account.CompanyID != scope.CompanyID || account.ID <= options.AfterID {
			continue
		}
		code := account.Code
		if code == "" {
			code = account.ID
		}
		if query != "" && !strings.Contains(strings.ToLower(code+" "+account.Name), query) {
			continue
		}
		exposure := creditExposure(s.state, scope, account.ID)
		available := account.CreditLimitMinor - exposure
		eligible := account.Active && !account.General && account.CreditEnabled && available > 0
		if options.CreditEligible != nil && eligible != *options.CreditEligible {
			continue
		}
		status := "inactive"
		if account.Active {
			status = "active"
		}
		items = append(items, readmodel.CustomerSummary{
			ID: account.ID, Code: code, Name: account.Name, Status: status,
			IsGeneralCustomer: account.General, CreditEnabled: account.CreditEnabled,
			CreditLimitMinor: account.CreditLimitMinor, CurrentExposureMinor: exposure,
			AvailableCreditMinor: available,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return snapshot, capCustomers(items, options.Limit+1), nil
}

func (s *Store) ListProducts(_ context.Context, scope tenancy.Scope, actorID string, options readmodel.ListOptions) (readmodel.CatalogSnapshot, []readmodel.ProductSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "products.read")] {
		return readmodel.CatalogSnapshot{}, nil, sales.ErrForbidden
	}
	workingContext, ok := s.state.contexts[scopeKey(scope)]
	if !ok {
		return readmodel.CatalogSnapshot{}, nil, sales.ErrNotFound
	}
	snapshot := readmodel.CatalogSnapshot{Token: workingContext.CatalogSnapshotToken, MasterDataVersion: workingContext.MasterDataVersion, PriceVersion: workingContext.PriceVersion}
	if options.CatalogSnapshotToken != "" && options.CatalogSnapshotToken != snapshot.Token {
		return readmodel.CatalogSnapshot{}, nil, devices.ErrStaleMasterData
	}
	captureCatalogPublication(s.state, scope, snapshot.Token)
	query := strings.ToLower(options.Query)
	items := make([]readmodel.ProductSummary, 0)
	for _, product := range s.state.products {
		if product.TenantID != scope.TenantID || product.CompanyID != scope.CompanyID || !product.Active || product.ID <= options.AfterID {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(product.SKU+" "+product.Name), query) {
			continue
		}
		unit := product.BaseUnitCode
		if unit == "" {
			unit = "EA"
		}
		priceVersion, masterVersion := product.PriceVersion, product.MasterDataVersion
		if priceVersion < 1 {
			priceVersion = 1
		}
		if masterVersion < 1 {
			masterVersion = 1
		}
		if workingContext, ok := s.state.contexts[scopeKey(scope)]; ok && workingContext.MasterDataVersion > 0 {
			masterVersion = workingContext.MasterDataVersion
		}
		taxBasisPoints, found := effectiveTaxBasisPoints(s.state, scope, product.TaxCode, time.Now().UTC())
		if !found || taxBasisPoints < 0 || taxBasisPoints > 10_000 {
			return readmodel.CatalogSnapshot{}, nil, sales.ErrPostingConfig
		}
		items = append(items, readmodel.ProductSummary{
			ID: product.ID, Code: product.SKU, Name: product.Name, Unit: unit,
			Currency: product.Currency, UnitPriceMinor: product.ListPriceMinor,
			AvailableQuantity: availableStock(s.state, scope, product.ID),
			PriceVersion:      priceVersion, MasterDataVersion: masterVersion, TaxBasisPoints: taxBasisPoints,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return snapshot, capProducts(items, options.Limit+1), nil
}

func effectiveTaxBasisPoints(current *state, scope tenancy.Scope, code string, at time.Time) (int64, bool) {
	var selected TaxRate
	found := false
	for _, value := range current.taxRates {
		if value.TenantID != scope.TenantID || value.CompanyID != scope.CompanyID || value.Code != code ||
			at.Before(value.EffectiveFrom) || (value.EffectiveTo != nil && !at.Before(*value.EffectiveTo)) {
			continue
		}
		if !found || value.EffectiveFrom.After(selected.EffectiveFrom) {
			selected, found = value, true
		}
	}
	return selected.BasisPoints, found
}

func (s *Store) ListSales(_ context.Context, scope tenancy.Scope, actorID string, options readmodel.ListOptions) ([]sales.Sale, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.permissions[permissionKey(scope, actorID, "sales.read")] {
		return nil, sales.ErrForbidden
	}
	items := make([]sales.Sale, 0)
	for _, value := range s.state.sales {
		if value.Scope == scope && value.ID > options.AfterID {
			items = append(items, cloneSale(value))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) > options.Limit+1 {
		items = items[:options.Limit+1]
	}
	return items, nil
}

func (t *transaction) MobileDevice(_ context.Context, scope tenancy.Scope, actorID, deviceID string) (devices.Device, error) {
	value, ok := t.state.devices[deviceKey(scope.TenantID, deviceID)]
	if !ok {
		return devices.Device{}, devices.ErrNotEnrolled
	}
	if value.Scope != scope || value.ActorID != actorID {
		return devices.Device{}, devices.ErrScopeMismatch
	}
	if workingContext, ok := t.state.contexts[scopeKey(scope)]; ok {
		value.AvailableMasterDataVersion = workingContext.MasterDataVersion
		value.AvailablePriceVersion = workingContext.PriceVersion
		value.AvailableCatalogSnapshotToken = workingContext.CatalogSnapshotToken
		value.TimeZone = workingContext.TimeZone
	}
	value = withAllocations(t.state, value)
	if err := devices.ValidateWireSafe(value); err != nil {
		return devices.Device{}, err
	}
	return value, nil
}

func (t *transaction) OfflineLeaseValid(_ context.Context, lease devices.OfflineLease, clientTimestamp time.Time) (bool, error) {
	device, ok := t.state.devices[deviceKey(lease.Scope.TenantID, lease.DeviceID)]
	if !ok || device.Scope != lease.Scope {
		return false, nil
	}
	for _, existing := range t.state.offlineLeases {
		if existing.Scope == lease.Scope && existing.DeviceID == lease.DeviceID &&
			existing.AppVersion == lease.AppVersion && existing.MasterDataVersion == lease.MasterDataVersion &&
			existing.PriceVersion == lease.PriceVersion && existing.CatalogSnapshotToken == lease.CatalogSnapshotToken &&
			existing.AuthorizationEpoch == device.AuthorizationEpoch && !clientTimestamp.Before(existing.ValidFrom) &&
			clientTimestamp.Before(existing.ValidUntil) {
			return true, nil
		}
	}
	return false, nil
}

func (t *transaction) OfflineAllocation(_ context.Context, scope tenancy.Scope, deviceID, productID string) (int64, error) {
	allocated, ok := t.state.allocations[allocationKey(scope, deviceID, productID)]
	if !ok || allocated <= 0 {
		return 0, devices.ErrAllocationExceeded
	}
	for _, sale := range t.state.sales {
		if sale.Scope != scope || sale.DeviceID != deviceID || !sale.Offline || sale.RecordType != sales.RecordSale || sale.Status != sales.StatusPosted {
			continue
		}
		for _, line := range sale.Lines {
			if line.ProductID == productID {
				allocated -= line.Quantity
			}
		}
	}
	return allocated, nil
}

func (t *transaction) OfflineSalesTotal(_ context.Context, scope tenancy.Scope, deviceID string, startsAt, endsAt time.Time) (int64, error) {
	var total int64
	for _, sale := range t.state.sales {
		if sale.Scope == scope && sale.DeviceID == deviceID && sale.Offline && sale.RecordType == sales.RecordSale &&
			sale.Status == sales.StatusPosted && sale.ClientTimestamp != nil &&
			!sale.ClientTimestamp.Before(startsAt) && sale.ClientTimestamp.Before(endsAt) {
			total += sale.TotalMinor
		}
	}
	return total, nil
}

func permissionsFor(current *state, scope tenancy.Scope, actorID string) []string {
	prefix := companyEntityKey(scope.TenantID, scope.CompanyID, actorID) + "\x00" + scope.BranchID + "\x00" + scope.WarehouseID + "\x00"
	result := make([]string, 0)
	for key, granted := range current.permissions {
		if granted && strings.HasPrefix(key, prefix) {
			result = append(result, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(result)
	return result
}

func creditExposure(current *state, scope tenancy.Scope, customerID string) int64 {
	var amount int64
	for _, entry := range current.ledger {
		if entry.TenantID == scope.TenantID && entry.CompanyID == scope.CompanyID && entry.CustomerID == customerID {
			amount += entry.AmountMinor
		}
	}
	return amount
}

func availableStock(current *state, scope tenancy.Scope, productID string) int64 {
	var quantity int64
	for _, movement := range current.movements {
		if movement.TenantID == scope.TenantID && movement.CompanyID == scope.CompanyID && movement.BranchID == scope.BranchID &&
			movement.WarehouseID == scope.WarehouseID && movement.ProductID == productID {
			quantity += movement.Quantity
		}
	}
	return quantity
}

func capCustomers(items []readmodel.CustomerSummary, limit int) []readmodel.CustomerSummary {
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func capProducts(items []readmodel.ProductSummary, limit int) []readmodel.ProductSummary {
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func withAllocations(current *state, device devices.Device) devices.Device {
	prefix := scopeKey(device.Scope) + "\x00" + device.ID + "\x00"
	device.StockAllocations = make([]devices.StockAllocation, 0)
	for key, allocated := range current.allocations {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		productID := strings.TrimPrefix(key, prefix)
		remaining := allocated
		for _, sale := range current.sales {
			if sale.Scope != device.Scope || sale.DeviceID != device.ID || !sale.Offline || sale.RecordType != sales.RecordSale || sale.Status != sales.StatusPosted {
				continue
			}
			for _, line := range sale.Lines {
				if line.ProductID == productID {
					remaining -= line.Quantity
				}
			}
		}
		device.StockAllocations = append(device.StockAllocations, devices.StockAllocation{
			ProductID: productID, AllocatedQuantity: allocated, RemainingQuantity: remaining,
		})
	}
	sort.Slice(device.StockAllocations, func(i, j int) bool {
		return device.StockAllocations[i].ProductID < device.StockAllocations[j].ProductID
	})
	device.OfflineRemainingDailyMinor = device.OfflineDailyLimitMinor
	if location, err := time.LoadLocation(device.TimeZone); err == nil {
		reference := device.LastSeenAt
		if reference.IsZero() {
			reference = time.Now().UTC()
		}
		local := reference.In(location)
		startLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		start, end := startLocal.UTC(), startLocal.AddDate(0, 0, 1).UTC()
		for _, sale := range current.sales {
			if sale.Scope == device.Scope && sale.DeviceID == device.ID && sale.Offline && sale.RecordType == sales.RecordSale &&
				sale.Status == sales.StatusPosted && sale.ClientTimestamp != nil &&
				!sale.ClientTimestamp.Before(start) && sale.ClientTimestamp.Before(end) {
				device.OfflineRemainingDailyMinor -= sale.TotalMinor
			}
		}
	}
	if device.OfflineRemainingDailyMinor < 0 {
		device.OfflineRemainingDailyMinor = 0
	}
	return device
}
