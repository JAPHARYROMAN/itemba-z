package mobile_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

const (
	tenantID    = "20000000-0000-4000-8000-000000000001"
	companyID   = "20000000-0000-4000-8000-000000000002"
	branchID    = "20000000-0000-4000-8000-000000000003"
	warehouseID = "20000000-0000-4000-8000-000000000004"
	actorID     = "20000000-0000-4000-8000-000000000005"
	customerID  = "20000000-0000-4000-8000-000000000006"
	productID   = "20000000-0000-4000-8000-000000000007"
	deviceID    = "20000000-0000-4000-8000-000000000008"
)

var testScope = tenancy.Scope{TenantID: tenantID, CompanyID: companyID, BranchID: branchID, WarehouseID: warehouseID}

var (
	catalogSnapshotTokenV1 = "00000000-0000-4000-8000-000000000001"
	catalogSnapshotTokenV2 = "00000000-0000-4000-8000-000000000002"
)

func fixture(t *testing.T, at time.Time) (*memory.Store, *mobile.Service) {
	t.Helper()
	store := memory.New()
	for _, permission := range []string{"sales.complete", "sales.read", "mobile.devices.enroll", "mobile.sales.sync", "mobile.reconciliation.read", "mobile.reconciliation.resolve", "customers.read", "products.read"} {
		store.SeedPermission(testScope, actorID, permission)
	}
	store.SeedContext(testScope, readmodel.WorkingContext{
		CompanyName: "Company", BranchName: "Branch", WarehouseName: "Warehouse",
		Currency: "TZS", Locale: "en-TZ", TimeZone: "Africa/Dar_es_Salaam",
		MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1,
	})
	store.SeedCustomer(customers.Account{ID: customerID, Code: "GENERAL", TenantID: tenantID, CompanyID: companyID, Name: "General Customer", Active: true, General: true})
	store.SeedProduct(catalog.Product{
		ID: productID, TenantID: tenantID, CompanyID: companyID, SKU: "SKU", Name: "Product",
		BaseUnitCode: "EA", Active: true, Currency: "TZS", ListPriceMinor: 10_000,
		StandardCostMinor: 6_000, TaxCode: "VAT", RevenueAccountID: "revenue",
		COGSAccountID: "cogs", InventoryAccountID: "inventory", PriceVersion: 1, MasterDataVersion: 1,
	})
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 1800, EffectiveFrom: at.AddDate(-1, 0, 0)})
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: tenantID, CompanyID: companyID, StartsAt: at.AddDate(-1, 0, 0), EndsAt: at.AddDate(1, 0, 0), Open: true})
	store.SeedOfflinePostingPolicy(sales.OfflinePostingPolicy{
		TenantID: tenantID, CompanyID: companyID,
		AccountingTimeBasis:      sales.AccountingTimeServerReceipt,
		MaximumFutureSkewSeconds: int64((4 * time.Hour) / time.Second),
		RequireSameFiscalPeriod:  true, EffectiveFrom: at.AddDate(-1, 0, 0),
	})
	store.SeedPostingConfig(tenantID, companyID, finance.SalesPostingConfig{ReceivableAccountID: "receivable", TaxPayableAccountID: "tax", CashAccounts: map[string]string{"CASH": "cash"}})
	store.SeedStock(inventory.Movement{ID: "opening", TenantID: tenantID, CompanyID: companyID, BranchID: branchID, WarehouseID: warehouseID, ProductID: productID, SourceType: "OPENING", SourceID: "opening", Quantity: 100, OccurredAt: at.Add(-time.Hour)})
	salesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	mobileService, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	return store, mobileService
}

func TestEnrollmentBindingAndMobileIdempotency(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	enrolled, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if enrolled.Scope != testScope || enrolled.ActorID != actorID || enrolled.TimeZone != "Africa/Dar_es_Salaam" || enrolled.StockAllocations == nil {
		t.Fatalf("enrollment=%+v", enrolled)
	}
	if enrolled.MasterDataVersion != 0 || enrolled.PriceVersion != 0 || enrolled.AvailableMasterDataVersion != 1 || enrolled.AvailablePriceVersion != 1 {
		t.Fatalf("new device was incorrectly treated as having an installed cache: %+v", enrolled)
	}
	one := int64(1)
	if _, err := service.Enroll(context.Background(), mobile.EnrollCommand{Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS renamed", AppVersion: "1.0.1", InstalledMasterDataVersion: &one, InstalledPriceVersion: &one, InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1}); err != nil {
		t.Fatalf("idempotent re-enrollment: %v", err)
	}
	if snapshot := store.Snapshot(); len(snapshot.Audits) != 2 || len(snapshot.Outbox) != 2 ||
		snapshot.Audits[1].Action != "mobile.device.installation_acknowledged" ||
		snapshot.Outbox[1].EventType != "mobile.device.installation_acknowledged" {
		t.Fatalf("installation acknowledgement evidence: %+v", snapshot)
	}
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: "CASH",
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000009", ClientTimestamp: at,
		AppVersion: "1.0.1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1,
	}
	first, err := service.SyncSale(context.Background(), testScope, actorID, "", command)
	if err != nil {
		t.Fatal(err)
	}
	command.SyncAttempt = 2
	replay, err := service.SyncSale(context.Background(), testScope, actorID, "", command)
	if err != nil {
		t.Fatal(err)
	}
	if first.IdempotentReplay || !replay.IdempotentReplay || first.Sale.ID != replay.Sale.ID || first.ReceiptReference != replay.ReceiptReference || first.FiscalStatus != sales.FiscalNotConfigured {
		t.Fatalf("first=%+v replay=%+v", first, replay)
	}
	command.Lines[0].Quantity = 2
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrIdempotencyConflict) {
		t.Fatalf("changed payload error=%v", err)
	}
}

func TestUnboundAndMismatchedDevicesAreRejected(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: "CASH",
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000010", ClientTimestamp: at,
		AppVersion: "1.0.0", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, devices.ErrNotEnrolled) {
		t.Fatalf("unbound error=%v", err)
	}
	store.SeedDevice(devices.Device{ID: deviceID, Status: devices.StatusActive, ActorID: actorID, Scope: testScope, AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, TimeZone: "Africa/Dar_es_Salaam", EnrolledAt: at, LastSeenAt: at})
	otherActor := "20000000-0000-4000-8000-000000000011"
	for _, permission := range []string{"sales.complete", "mobile.sales.sync"} {
		store.SeedPermission(testScope, otherActor, permission)
	}
	if _, err := service.SyncSale(context.Background(), testScope, otherActor, "", command); !errors.Is(err, devices.ErrScopeMismatch) {
		t.Fatalf("mismatch error=%v", err)
	}
}

func TestReEnrollmentRefreshesVersionsAndOnlineSyncRejectsStaleState(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	one := int64(1)
	if _, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.0.0",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	}); err != nil {
		t.Fatal(err)
	}
	store.SeedContext(testScope, readmodel.WorkingContext{
		CompanyName: "Company", BranchName: "Branch", WarehouseName: "Warehouse",
		Currency: "TZS", Locale: "en-TZ", TimeZone: "Africa/Dar_es_Salaam",
		MasterDataVersion: 2, PriceVersion: 3, CatalogSnapshotToken: catalogSnapshotTokenV2,
	})
	refreshed, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.1.0",
	})
	if err != nil || refreshed.MasterDataVersion != 1 || refreshed.PriceVersion != 1 || refreshed.AvailableMasterDataVersion != 2 || refreshed.AvailablePriceVersion != 3 || refreshed.AppVersion != "1.0.0" {
		t.Fatalf("unacknowledged refresh changed installed versions: %+v err=%v", refreshed, err)
	}
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: "CASH",
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000014", ClientTimestamp: at,
		AppVersion: "1.0.0", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, devices.ErrStaleMasterData) {
		t.Fatalf("online stale state error=%v", err)
	}
	if _, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.1.0",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	}); !errors.Is(err, devices.ErrStaleMasterData) {
		t.Fatalf("stale install acknowledgement error=%v", err)
	}
	unchanged, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.1.0",
	})
	if err != nil || unchanged.MasterDataVersion != 1 || unchanged.PriceVersion != 1 || unchanged.AppVersion != "1.0.0" {
		t.Fatalf("stale acknowledgement changed installed versions: %+v err=%v", unchanged, err)
	}
	if snapshot := store.Snapshot(); len(snapshot.Audits) != 1 || len(snapshot.Outbox) != 1 {
		t.Fatalf("stale acknowledgement left effects: %+v", snapshot)
	}
	two, three := int64(2), int64(3)
	acknowledged, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.1.0",
		InstalledMasterDataVersion: &two, InstalledPriceVersion: &three,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV2,
	})
	if err != nil || acknowledged.MasterDataVersion != 2 || acknowledged.PriceVersion != 3 || acknowledged.AppVersion != "1.1.0" {
		t.Fatalf("cache acknowledgement failed: %+v err=%v", acknowledged, err)
	}
	// Retrying the same acknowledgement models a lost response and is idempotent.
	acknowledged, err = service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1.1.0",
		InstalledMasterDataVersion: &two, InstalledPriceVersion: &three,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV2,
	})
	if err != nil || acknowledged.MasterDataVersion != 2 || acknowledged.PriceVersion != 3 {
		t.Fatalf("lost-response acknowledgement retry failed: %+v err=%v", acknowledged, err)
	}
	if snapshot := store.Snapshot(); len(snapshot.Audits) != 2 || len(snapshot.Outbox) != 2 {
		t.Fatalf("lost-response acknowledgement retry duplicated evidence: %+v", snapshot)
	}
	command.AppVersion, command.MasterDataVersion, command.PriceVersion = "1.1.0", 2, 3
	command.CatalogSnapshotToken = catalogSnapshotTokenV2
	created, err := service.SyncSale(context.Background(), testScope, actorID, "", command)
	if err != nil || created.Sale.ClientTimestamp == nil || created.Sale.AppVersion != "1.1.0" || created.Sale.MasterDataVersion != 2 || created.Sale.PriceVersion != 3 {
		t.Fatalf("current state sync=%+v err=%v", created, err)
	}
}

func TestInstallationAcknowledgementIsAuditedAndIdempotentAcrossLeaseRenewal(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	device := offlineDevice(at)
	device.Name = "POS Original"
	device.OfflineSalesValidFrom, device.OfflineSalesValidUntil = time.Time{}, time.Time{}
	store.SeedDevice(device)
	one := int64(1)

	acknowledged, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS Acknowledged", AppVersion: "1",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	})
	if err != nil || !acknowledged.OfflineSalesValidUntil.Equal(at.Add(devices.OfflineSalesLeaseDuration)) {
		t.Fatalf("initial acknowledgement=%+v err=%v", acknowledged, err)
	}
	assertAcknowledgementEvidence(t, store.Snapshot(), 1, "1", 1, 1, acknowledged.OfflineSalesValidUntil)

	retried, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS Retry Rename", AppVersion: "1",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	})
	if err != nil || retried.Name != "POS Acknowledged" || !retried.OfflineSalesValidUntil.Equal(acknowledged.OfflineSalesValidUntil) {
		t.Fatalf("live acknowledgement retry was not a pure read: %+v err=%v", retried, err)
	}
	assertAcknowledgementEvidence(t, store.Snapshot(), 1, "1", 1, 1, acknowledged.OfflineSalesValidUntil)

	checkedIn, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS Check-in", AppVersion: "unacknowledged-upgrade",
	})
	if err != nil || checkedIn.AppVersion != "1" || checkedIn.MasterDataVersion != 1 || checkedIn.PriceVersion != 1 ||
		!checkedIn.OfflineSalesValidUntil.Equal(acknowledged.OfflineSalesValidUntil) {
		t.Fatalf("no-ack check-in changed installed state: %+v err=%v", checkedIn, err)
	}
	assertAcknowledgementEvidence(t, store.Snapshot(), 1, "1", 1, 1, acknowledged.OfflineSalesValidUntil)

	renewedAt := acknowledged.OfflineSalesValidUntil.Add(time.Second)
	salesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: renewedAt})
	if err != nil {
		t.Fatal(err)
	}
	renewalService, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: renewedAt})
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := renewalService.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS Renewed", AppVersion: "1",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	})
	if err != nil || !renewed.OfflineSalesValidUntil.Equal(renewedAt.Add(devices.OfflineSalesLeaseDuration)) {
		t.Fatalf("expired acknowledgement renewal=%+v err=%v", renewed, err)
	}
	assertAcknowledgementEvidence(t, store.Snapshot(), 2, "1", 1, 1, renewed.OfflineSalesValidUntil)

	store.SeedContext(testScope, readmodel.WorkingContext{
		CompanyName: "Company", BranchName: "Branch", WarehouseName: "Warehouse",
		Currency: "TZS", Locale: "en-TZ", TimeZone: "Africa/Dar_es_Salaam",
		MasterDataVersion: 2, PriceVersion: 2, CatalogSnapshotToken: catalogSnapshotTokenV2,
	})
	changedAt := renewedAt.Add(time.Minute)
	salesService, err = sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: changedAt})
	if err != nil {
		t.Fatal(err)
	}
	changedService, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: changedAt})
	if err != nil {
		t.Fatal(err)
	}
	two := int64(2)
	changed, err := changedService.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS Upgraded", AppVersion: "2",
		InstalledMasterDataVersion: &two, InstalledPriceVersion: &two,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV2,
	})
	if err != nil || changed.AppVersion != "2" || changed.MasterDataVersion != 2 || changed.PriceVersion != 2 {
		t.Fatalf("changed installation acknowledgement=%+v err=%v", changed, err)
	}
	assertAcknowledgementEvidence(t, store.Snapshot(), 3, "2", 2, 2, changed.OfflineSalesValidUntil)
	if _, err := changedService.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS Duplicate", AppVersion: "2",
		InstalledMasterDataVersion: &two, InstalledPriceVersion: &two,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV2,
	}); err != nil {
		t.Fatal(err)
	}
	assertAcknowledgementEvidence(t, store.Snapshot(), 3, "2", 2, 2, changed.OfflineSalesValidUntil)
}

func assertAcknowledgementEvidence(t *testing.T, snapshot memory.Snapshot, expectedCount int, appVersion string, masterVersion, priceVersion int64, validUntil time.Time) {
	t.Helper()
	if len(snapshot.Audits) != expectedCount || len(snapshot.Outbox) != expectedCount || len(snapshot.OfflineLeases) != expectedCount {
		t.Fatalf("acknowledgement evidence counts audits=%d outbox=%d leases=%d expected=%d", len(snapshot.Audits), len(snapshot.Outbox), len(snapshot.OfflineLeases), expectedCount)
	}
	auditEvent, outboxEvent := snapshot.Audits[len(snapshot.Audits)-1], snapshot.Outbox[len(snapshot.Outbox)-1]
	if auditEvent.Action != "mobile.device.installation_acknowledged" || outboxEvent.EventType != "mobile.device.installation_acknowledged" {
		t.Fatalf("acknowledgement evidence types audit=%q outbox=%q", auditEvent.Action, outboxEvent.EventType)
	}
	var audited, published devices.Device
	if err := json.Unmarshal(auditEvent.Data, &audited); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(outboxEvent.Payload, &published); err != nil {
		t.Fatal(err)
	}
	for label, value := range map[string]devices.Device{"audit": audited, "outbox": published} {
		if value.AppVersion != appVersion || value.MasterDataVersion != masterVersion || value.PriceVersion != priceVersion || !value.OfflineSalesValidUntil.Equal(validUntil) {
			t.Fatalf("%s did not contain final acknowledged state: %+v", label, value)
		}
	}
}

func TestOfflineDailyLimitUsesDarEsSalaamBusinessDay(t *testing.T) {
	firstTime := time.Date(2026, 8, 4, 20, 30, 0, 0, time.UTC) // 23:30 EAT
	store, firstService := fixture(t, firstTime)
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: firstTime.Add(-time.Minute)})
	device := offlineDevice(firstTime)
	device.OfflineTransactionLimitMinor, device.OfflineDailyLimitMinor = 15_000, 15_000
	store.SeedDevice(device)
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: "CASH",
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000012", ClientTimestamp: firstTime,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true,
	}
	if _, err := firstService.SyncSale(context.Background(), testScope, actorID, "", command); err != nil {
		t.Fatalf("first local day: %v", err)
	}
	secondTime := firstTime.Add(time.Hour) // 00:30 EAT next day, still same UTC day
	salesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: secondTime})
	if err != nil {
		t.Fatal(err)
	}
	secondService, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: secondTime})
	if err != nil {
		t.Fatal(err)
	}
	command.ClientTransactionID = "20000000-0000-4000-8000-000000000013"
	command.ClientTimestamp = firstTime.Add(15 * time.Minute) // queued before midnight, synced after midnight
	if _, err := secondService.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, devices.ErrOfflineLimit) {
		t.Fatalf("queued pre-midnight sale escaped its original business-day limit: %v", err)
	}
	command.ClientTransactionID = "20000000-0000-4000-8000-000000000015"
	command.ClientTimestamp = secondTime
	if _, err := secondService.SyncSale(context.Background(), testScope, actorID, "", command); err != nil {
		t.Fatalf("next local business day should have a fresh limit: %v", err)
	}
}

func TestOfflineRejectsNonCashSettlementWithoutEffects(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: at.Add(-time.Minute)})
	store.SeedDevice(offlineDevice(at))
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	command := mobile.SyncCommand{CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentBankCard, Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID, ClientTransactionID: "20000000-0000-4000-8000-000000000020", ClientTimestamp: at, AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrOfflinePaymentMethod) {
		t.Fatalf("offline non-cash error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
}

func TestOnlineMobileCreditIsRejectedWithoutEffects(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	creditCustomerID := "20000000-0000-4000-8000-000000000030"
	store.SeedCustomer(customers.Account{
		ID: creditCustomerID, Code: "CREDIT-READY", TenantID: tenantID, CompanyID: companyID,
		Name: "Credit Ready Customer", Active: true, CreditEnabled: true, CreditLimitMinor: 10_000_000,
	})
	store.SeedDevice(devices.Device{
		ID: deviceID, Status: devices.StatusActive, ActorID: actorID, Scope: testScope,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1,
		AvailableMasterDataVersion: 1, AvailablePriceVersion: 1,
		AvailableCatalogSnapshotToken: catalogSnapshotTokenV1,
		TimeZone:                      "Africa/Dar_es_Salaam", EnrolledAt: at, LastSeenAt: at,
	})
	command := mobile.SyncCommand{
		CustomerID: creditCustomerID, Kind: sales.KindCredit,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000031", ClientTimestamp: at,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: false,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, devices.ErrMobileCreditUnsupported) {
		t.Fatalf("online mobile credit error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
	snapshot := store.Snapshot()
	if len(snapshot.Ledger) != 0 || len(snapshot.Audits) != 0 {
		t.Fatalf("rejected online mobile credit left ledger or audit effects: %+v", snapshot)
	}
}

func TestOfflineRejectsNonzeroEffectiveTaxWithoutEffects(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	store.SeedDevice(offlineDevice(at))
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	command := mobile.SyncCommand{CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash, Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID, ClientTransactionID: "20000000-0000-4000-8000-000000000021", ClientTimestamp: at, AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrOfflineTaxUnsupported) {
		t.Fatalf("offline nonzero-tax error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
}

func TestOfflineLeaseIsBoundedByTaxTransitionAndExclusiveAtDeadline(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	transition := at.Add(90 * time.Minute)
	store, service := fixture(t, at)
	store.SeedTaxRate(memory.TaxRate{
		TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0,
		EffectiveFrom: at.Add(-time.Minute), EffectiveTo: &transition,
	})
	store.SeedTaxRate(memory.TaxRate{
		TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 1800,
		EffectiveFrom: transition,
	})
	device := offlineDevice(at)
	device.OfflineSalesValidFrom, device.OfflineSalesValidUntil = time.Time{}, time.Time{}
	store.SeedDevice(device)
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	one := int64(1)
	enrolled, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	})
	if err != nil || !enrolled.OfflineSalesValidUntil.Equal(transition) {
		t.Fatalf("tax-bounded lease=%+v err=%v", enrolled, err)
	}
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000022", ClientTimestamp: transition,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, devices.ErrOfflineLeaseExpired) {
		t.Fatalf("deadline error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
}

func TestOfflineLeaseNeverExceedsFourHours(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	device := offlineDevice(at)
	device.OfflineSalesValidFrom, device.OfflineSalesValidUntil = time.Time{}, time.Time{}
	store.SeedDevice(device)
	one := int64(1)
	enrolled, err := service.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	})
	if err != nil || !enrolled.OfflineSalesValidUntil.Equal(at.Add(devices.OfflineSalesLeaseDuration)) {
		t.Fatalf("bounded lease=%+v err=%v", enrolled, err)
	}
}

func TestOfflineSaleRecordsDocumentReceiptAndAccountingTimes(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	documentAt := at.Add(-30 * time.Minute)
	store, service := fixture(t, at)
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: at.Add(-time.Hour)})
	store.SeedDevice(offlineDevice(documentAt))
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000040", ClientTimestamp: documentAt,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1,
		CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true,
	}
	result, err := service.SyncSale(context.Background(), testScope, actorID, "", command)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Sale.DocumentAt.Equal(documentAt) || !result.Sale.ReceivedAt.Equal(at) ||
		!result.Sale.AccountingAt.Equal(at) || result.Sale.AccountingTimeBasis != sales.AccountingTimeServerReceipt {
		t.Fatalf("governed sale times=%+v", result.Sale)
	}
	snapshot := store.Snapshot()
	if len(snapshot.Journals) != 1 || !snapshot.Journals[0].OccurredAt.Equal(at) ||
		len(snapshot.Payments) != 1 || !snapshot.Payments[0].OccurredAt.Equal(at) {
		t.Fatalf("posting effects did not use accounting time: %+v", snapshot)
	}
	store.ReplaceFiscalPeriods(memory.FiscalPeriod{
		ID: "later-period", TenantID: tenantID, CompanyID: companyID,
		StartsAt: at.Add(time.Minute), EndsAt: at.AddDate(0, 1, 0), Open: true,
	})
	command.SyncAttempt = 2
	replay, err := service.SyncSale(context.Background(), testScope, actorID, "", command)
	if err != nil || !replay.IdempotentReplay || replay.Sale.ID != result.Sale.ID {
		t.Fatalf("committed result must replay after period changes: %+v err=%v", replay, err)
	}
}

func TestOfflineClockSkewRoutesExactCommandToReconciliation(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	store.ReplaceOfflinePostingPolicies(sales.OfflinePostingPolicy{
		TenantID: tenantID, CompanyID: companyID, AccountingTimeBasis: sales.AccountingTimeServerReceipt,
		MaximumFutureSkewSeconds: 60, RequireSameFiscalPeriod: true, EffectiveFrom: at.Add(-time.Hour),
	})
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: at.Add(-time.Hour)})
	store.SeedDevice(offlineDevice(at))
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000041", ClientTimestamp: at.Add(2 * time.Minute),
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1,
		CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrOfflineClockReconciliation) {
		t.Fatalf("clock skew error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
	cases := store.Snapshot().ReconciliationCases
	if len(cases) != 1 || cases[0].FailureCode != "offline_clock_reconciliation_required" ||
		cases[0].ClientTransactionID != command.ClientTransactionID {
		t.Fatalf("clock reconciliation evidence=%+v", cases)
	}
}

func TestOfflineCrossPeriodDocumentRoutesToReconciliation(t *testing.T) {
	documentAt := time.Date(2026, 8, 31, 23, 30, 0, 0, time.UTC)
	receivedAt := documentAt.Add(time.Hour)
	store, _ := fixture(t, receivedAt)
	store.ReplaceFiscalPeriods(
		memory.FiscalPeriod{ID: "august", TenantID: tenantID, CompanyID: companyID, StartsAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Open: true},
		memory.FiscalPeriod{ID: "september", TenantID: tenantID, CompanyID: companyID, StartsAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Open: true},
	)
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: documentAt.Add(-time.Hour)})
	store.SeedDevice(offlineDevice(documentAt))
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	salesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: receivedAt})
	if err != nil {
		t.Fatal(err)
	}
	service, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: receivedAt})
	if err != nil {
		t.Fatal(err)
	}
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000042", ClientTimestamp: documentAt,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1,
		CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrOfflinePeriodReconciliation) {
		t.Fatalf("cross-period error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
	cases := store.Snapshot().ReconciliationCases
	if len(cases) != 1 || cases[0].FailureCode != "offline_fiscal_period_reconciliation_required" {
		t.Fatalf("period reconciliation evidence=%+v", cases)
	}
}

func TestHistoricalLeasePostsImmutableProductFactsAfterRenewalAndPriceChange(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, firstService := fixture(t, at)
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: at.Add(-time.Minute)})
	device := offlineDevice(at)
	device.OfflineSalesValidFrom, device.OfflineSalesValidUntil = time.Time{}, time.Time{}
	store.SeedDevice(device)
	store.SeedOfflineAllocation(testScope, deviceID, productID, 10)
	one := int64(1)
	if _, err := firstService.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "1",
		InstalledMasterDataVersion: &one, InstalledPriceVersion: &one,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV1,
	}); err != nil {
		t.Fatal(err)
	}
	store.SeedContext(testScope, readmodel.WorkingContext{
		CompanyName: "Company", BranchName: "Branch", WarehouseName: "Warehouse",
		Currency: "TZS", Locale: "en-TZ", TimeZone: "Africa/Dar_es_Salaam",
		MasterDataVersion: 2, PriceVersion: 2, CatalogSnapshotToken: catalogSnapshotTokenV2,
	})
	renewedAt := at.Add(time.Hour)
	salesService, err := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: renewedAt})
	if err != nil {
		t.Fatal(err)
	}
	renewalService, err := mobile.NewService(store, salesService, identity.UUIDGenerator{}, clock.Fixed{Time: renewedAt})
	if err != nil {
		t.Fatal(err)
	}
	two := int64(2)
	if _, err := renewalService.Enroll(context.Background(), mobile.EnrollCommand{
		Scope: testScope, ActorID: actorID, DeviceID: deviceID, DeviceName: "POS 1", AppVersion: "2",
		InstalledMasterDataVersion: &two, InstalledPriceVersion: &two,
		InstalledCatalogSnapshotToken: &catalogSnapshotTokenV2,
	}); err != nil {
		t.Fatal(err)
	}
	queued := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: productID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000023", ClientTimestamp: at.Add(30 * time.Minute),
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, SyncAttempt: 1, Offline: true,
	}
	if _, err := renewalService.SyncSale(context.Background(), testScope, actorID, "", queued); err != nil {
		t.Fatalf("queued sale under historical lease: %v", err)
	}
	committed := queued
	before := store.Snapshot()
	store.SeedProduct(catalog.Product{
		ID: productID, TenantID: tenantID, CompanyID: companyID, SKU: "SKU", Name: "Product",
		BaseUnitCode: "EA", Active: true, Currency: "TZS", ListPriceMinor: 12_000,
		StandardCostMinor: 6_000, TaxCode: "VAT", RevenueAccountID: "revenue",
		COGSAccountID: "cogs", InventoryAccountID: "inventory", PriceVersion: 2, MasterDataVersion: 2,
	})
	committed.SyncAttempt = 2
	replayed, err := renewalService.SyncSale(context.Background(), testScope, actorID, "", committed)
	if err != nil || !replayed.IdempotentReplay {
		t.Fatalf("committed sale replay after governed drift: %+v err=%v", replayed, err)
	}
	queued.ClientTransactionID = "20000000-0000-4000-8000-000000000024"
	queued.ClientTimestamp = at.Add(45 * time.Minute)
	historical, err := renewalService.SyncSale(context.Background(), testScope, actorID, "", queued)
	if err != nil {
		t.Fatalf("historical publication post: %v", err)
	}
	if historical.Sale.TotalMinor != 10_000 || historical.Sale.Lines[0].UnitPriceMinor != 10_000 || historical.Sale.Lines[0].UnitCostMinor != 6_000 {
		t.Fatalf("historical facts were not preserved: %+v", historical.Sale)
	}
	after := store.Snapshot()
	if len(after.Sales) != len(before.Sales)+1 || len(after.Payments) != len(before.Payments)+1 || len(after.Journals) != len(before.Journals)+1 || len(after.Outbox) != len(before.Outbox)+1 || len(after.Movements) != len(before.Movements)+1 {
		t.Fatalf("historical sale did not commit exactly once: before=%+v after=%+v", before, after)
	}
}

func TestMissingHistoricalPublicationFactRequiresReconciliationWithoutEffects(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	store.SeedTaxRate(memory.TaxRate{TenantID: tenantID, CompanyID: companyID, Code: "VAT", BasisPoints: 0, EffectiveFrom: at.Add(-time.Minute)})
	store.SeedDevice(offlineDevice(at))
	const unpublishedProductID = "20000000-0000-4000-8000-000000000099"
	store.SeedProduct(catalog.Product{
		ID: unpublishedProductID, TenantID: tenantID, CompanyID: companyID, SKU: "LATE", Name: "Not published",
		BaseUnitCode: "EA", Active: true, Currency: "TZS", ListPriceMinor: 10_000,
		StandardCostMinor: 6_000, TaxCode: "VAT", RevenueAccountID: "revenue",
		COGSAccountID: "cogs", InventoryAccountID: "inventory", PriceVersion: 1, MasterDataVersion: 1,
	})
	command := mobile.SyncCommand{
		CustomerID: customerID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash,
		Lines: []sales.CommandLine{{ProductID: unpublishedProductID, Quantity: 1}}, DeviceID: deviceID,
		ClientTransactionID: "20000000-0000-4000-8000-000000000098", ClientTimestamp: at,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1,
		SyncAttempt: 1, Offline: true,
	}
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrOfflineReconciliation) {
		t.Fatalf("missing publication fact error=%v", err)
	}
	assertNoMobileSaleEffects(t, store)
	opened := store.Snapshot()
	if len(opened.ReconciliationCases) != 1 || opened.ReconciliationCases[0].Status != mobile.ReconciliationOpen ||
		len(opened.Audits) != 1 || opened.Audits[0].Action != "mobile.reconciliation.opened" ||
		len(opened.Outbox) != 1 || opened.Outbox[0].EventType != "mobile.reconciliation.opened" {
		t.Fatalf("reconciliation evidence=%+v", opened)
	}
	command.SyncAttempt = 2
	if _, err := service.SyncSale(context.Background(), testScope, actorID, "", command); !errors.Is(err, sales.ErrOfflineReconciliation) {
		t.Fatalf("reconciliation replay error=%v", err)
	}
	replayed := store.Snapshot()
	if len(replayed.ReconciliationCases) != 1 || len(replayed.Audits) != 1 || len(replayed.Outbox) != 1 {
		t.Fatalf("reconciliation replay duplicated evidence: %+v", replayed)
	}
	page, err := service.ReconciliationCases(context.Background(), testScope, actorID, "OPEN", "", 50)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != opened.ReconciliationCases[0].ID {
		t.Fatalf("open reconciliation page=%+v err=%v", page, err)
	}
	resolved, err := service.ResolveReconciliation(context.Background(), mobile.ResolveReconciliationCommand{
		Scope: testScope, ActorID: actorID, CaseID: page.Items[0].ID,
		Action: mobile.ResolutionCashRefunded, Reason: "Cash returned to customer",
		IdempotencyKey: "refund-reconciliation-0001",
	})
	if err != nil || resolved.Status != mobile.ReconciliationResolved || resolved.Resolution == nil {
		t.Fatalf("resolved case=%+v err=%v", resolved, err)
	}
	resolvedReplay, err := service.ResolveReconciliation(context.Background(), mobile.ResolveReconciliationCommand{
		Scope: testScope, ActorID: actorID, CaseID: page.Items[0].ID,
		Action: mobile.ResolutionCashRefunded, Reason: "Cash returned to customer",
		IdempotencyKey: "refund-reconciliation-0001",
	})
	if err != nil || resolvedReplay.Resolution == nil || resolvedReplay.Resolution.ID != resolved.Resolution.ID {
		t.Fatalf("resolution replay=%+v err=%v", resolvedReplay, err)
	}
	if _, err := service.ResolveReconciliation(context.Background(), mobile.ResolveReconciliationCommand{
		Scope: testScope, ActorID: actorID, CaseID: page.Items[0].ID,
		Action: mobile.ResolutionDuplicateConfirmed, Reason: "Verified as an existing sale",
		IdempotencyKey: "refund-reconciliation-0001",
	}); !errors.Is(err, sales.ErrIdempotencyConflict) {
		t.Fatalf("resolution mismatch error=%v", err)
	}
	final := store.Snapshot()
	if len(final.Sales) != 0 || len(final.Payments) != 0 || len(final.Journals) != 0 || len(final.Movements) != 1 ||
		len(final.ReconciliationCases) != 1 || len(final.Audits) != 2 || len(final.Outbox) != 2 {
		t.Fatalf("resolution created accounting effects or duplicate evidence: %+v", final)
	}
}

func TestReconciliationReadIsPermissionAndExactScopeBound(t *testing.T) {
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	store, service := fixture(t, at)
	otherScope := testScope
	otherScope.BranchID = "20000000-0000-4000-8000-000000000088"
	otherScope.WarehouseID = "20000000-0000-4000-8000-000000000089"
	store.SeedPermission(otherScope, actorID, "mobile.reconciliation.read")
	if _, err := service.ReconciliationCases(context.Background(), otherScope, actorID, "", "", 50); err != nil {
		t.Fatalf("empty exact-scope list: %v", err)
	}
	const unprivileged = "20000000-0000-4000-8000-000000000087"
	if _, err := service.ReconciliationCases(context.Background(), testScope, unprivileged, "", "", 50); !errors.Is(err, sales.ErrForbidden) {
		t.Fatalf("permission error=%v", err)
	}
}

func assertNoMobileSaleEffects(t *testing.T, store *memory.Store) {
	t.Helper()
	snapshot := store.Snapshot()
	if len(snapshot.Sales) != 0 || len(snapshot.Payments) != 0 || len(snapshot.Journals) != 0 || len(snapshot.Movements) != 1 {
		t.Fatalf("rejected mobile sale left effects: %+v", snapshot)
	}
	for _, event := range snapshot.Outbox {
		if event.AggregateType == "sale" {
			t.Fatalf("rejected mobile sale left an outbox event: %+v", event)
		}
	}
}

func offlineDevice(at time.Time) devices.Device {
	return devices.Device{
		ID: deviceID, Status: devices.StatusActive, ActorID: actorID, Scope: testScope,
		AppVersion: "1", MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: catalogSnapshotTokenV1, TimeZone: "Africa/Dar_es_Salaam",
		OfflineEnabled: true, OfflineTransactionLimitMinor: 1_000_000, OfflineDailyLimitMinor: 1_000_000,
		OfflineSalesValidFrom: at.Add(-time.Minute), OfflineSalesValidUntil: at.Add(devices.OfflineSalesLeaseDuration),
		EnrolledAt: at, LastSeenAt: at,
	}
}
