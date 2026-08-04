// Package memory implements the sales repository contract with copy-on-write
// transactions. It is deterministic and intended for domain/integration tests
// and explicitly configured local development, never as a production database.
package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type TaxRate struct {
	TenantID      string
	CompanyID     string
	Code          string
	BasisPoints   int64
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
}

type FiscalPeriod struct {
	TenantID  string
	CompanyID string
	StartsAt  time.Time
	EndsAt    time.Time
	Open      bool
}

type idempotency struct {
	RequestHash string
	ResultID    string
}

type state struct {
	permissions   map[string]bool
	customers     map[string]customers.Account
	products      map[string]catalog.Product
	taxRates      []TaxRate
	periods       []FiscalPeriod
	posting       map[string]finance.SalesPostingConfig
	sales         map[string]sales.Sale
	movements     []inventory.Movement
	ledger        []customers.LedgerEntry
	payments      []sales.Payment
	journals      []finance.Journal
	audits        []audit.Event
	outbox        []outbox.Event
	idempotencies map[string]idempotency
	contexts      map[string]readmodel.WorkingContext
	devices       map[string]devices.Device
	offlineLeases []devices.OfflineLease
	allocations   map[string]int64
}

func newState() *state {
	return &state{
		permissions: make(map[string]bool),
		customers:   make(map[string]customers.Account), products: make(map[string]catalog.Product),
		posting: make(map[string]finance.SalesPostingConfig), sales: make(map[string]sales.Sale),
		idempotencies: make(map[string]idempotency),
		contexts:      make(map[string]readmodel.WorkingContext), devices: make(map[string]devices.Device),
		allocations: make(map[string]int64),
	}
}

type Store struct {
	mu    sync.Mutex
	state *state
}

func New() *Store { return &Store{state: newState()} }

func (s *Store) WithTransaction(ctx context.Context, fn func(sales.Transaction) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	if err := fn(&transaction{state: next}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) SeedCustomer(value customers.Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.customers[companyEntityKey(value.TenantID, value.CompanyID, value.ID)] = value
}

func (s *Store) SeedPermission(scope tenancy.Scope, actorID, permission string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.permissions[permissionKey(scope, actorID, permission)] = true
}

func (s *Store) SeedProduct(value catalog.Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.products[companyEntityKey(value.TenantID, value.CompanyID, value.ID)] = value
}

func (s *Store) SeedTaxRate(value TaxRate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.taxRates = append(s.state.taxRates, value)
}

func (s *Store) SeedFiscalPeriod(value FiscalPeriod) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.periods = append(s.state.periods, value)
}

func (s *Store) ReplaceFiscalPeriods(values ...FiscalPeriod) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.periods = append([]FiscalPeriod(nil), values...)
}

func (s *Store) SeedPostingConfig(tenantID, companyID string, value finance.SalesPostingConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyValue := value
	copyValue.CashAccounts = cloneStringMap(value.CashAccounts)
	s.state.posting[companyKey(tenantID, companyID)] = copyValue
}

func (s *Store) SeedStock(value inventory.Movement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.movements = append(s.state.movements, value)
}

func (s *Store) SeedContext(scope tenancy.Scope, value readmodel.WorkingContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value.TenantID, value.CompanyID = scope.TenantID, scope.CompanyID
	value.BranchID, value.WarehouseID = scope.BranchID, scope.WarehouseID
	s.state.contexts[scopeKey(scope)] = value
}

func (s *Store) SeedDevice(value devices.Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.devices[deviceKey(value.Scope.TenantID, value.ID)] = value
	if value.OfflineSalesValidUntil.After(value.OfflineSalesValidFrom) {
		appendOfflineLease(s.state, devices.OfflineLease{
			Scope: value.Scope, DeviceID: value.ID, AppVersion: value.AppVersion,
			MasterDataVersion: value.MasterDataVersion, PriceVersion: value.PriceVersion,
			ValidFrom: value.OfflineSalesValidFrom, ValidUntil: value.OfflineSalesValidUntil,
		})
	}
}

func (s *Store) SeedOfflineAllocation(scope tenancy.Scope, deviceID, productID string, quantity int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.allocations[allocationKey(scope, deviceID, productID)] = quantity
}

type Snapshot struct {
	Sales         []sales.Sale
	Movements     []inventory.Movement
	Ledger        []customers.LedgerEntry
	Payments      []sales.Payment
	Journals      []finance.Journal
	Audits        []audit.Event
	Outbox        []outbox.Event
	OfflineLeases []devices.OfflineLease
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := Snapshot{
		Movements:     append([]inventory.Movement(nil), s.state.movements...),
		Ledger:        append([]customers.LedgerEntry(nil), s.state.ledger...),
		Payments:      append([]sales.Payment(nil), s.state.payments...),
		Audits:        append([]audit.Event(nil), s.state.audits...),
		Outbox:        append([]outbox.Event(nil), s.state.outbox...),
		OfflineLeases: append([]devices.OfflineLease(nil), s.state.offlineLeases...),
	}
	for _, sale := range s.state.sales {
		result.Sales = append(result.Sales, cloneSale(sale))
	}
	for _, journal := range s.state.journals {
		result.Journals = append(result.Journals, cloneJournal(journal))
	}
	return result
}

type transaction struct{ state *state }

func (t *transaction) Authorize(_ context.Context, scope tenancy.Scope, actorID, permission string) (bool, error) {
	return t.state.permissions[permissionKey(scope, actorID, permission)], nil
}

func (t *transaction) ClaimIdempotency(_ context.Context, scope tenancy.Scope, operation, key, requestHash string) (bool, string, error) {
	storageKey := idempotencyKey(scope, operation, key)
	if found, ok := t.state.idempotencies[storageKey]; ok {
		if found.RequestHash != requestHash {
			return false, "", sales.ErrIdempotencyConflict
		}
		if found.ResultID == "" {
			return false, "", errors.New("idempotent operation has no committed result")
		}
		return false, found.ResultID, nil
	}
	t.state.idempotencies[storageKey] = idempotency{RequestHash: requestHash}
	return true, "", nil
}

func (t *transaction) CompleteIdempotency(_ context.Context, scope tenancy.Scope, operation, key, resultID string) error {
	storageKey := idempotencyKey(scope, operation, key)
	value, ok := t.state.idempotencies[storageKey]
	if !ok || value.ResultID != "" || resultID == "" {
		return errors.New("invalid idempotency completion")
	}
	value.ResultID = resultID
	t.state.idempotencies[storageKey] = value
	return nil
}

func (t *transaction) Customer(_ context.Context, scope tenancy.Scope, customerID string) (customers.Account, error) {
	value, ok := t.state.customers[companyEntityKey(scope.TenantID, scope.CompanyID, customerID)]
	if !ok {
		return customers.Account{}, sales.ErrNotFound
	}
	return value, nil
}

func (t *transaction) LockCustomerCredit(_ context.Context, scope tenancy.Scope, customerID string) error {
	value, ok := t.state.customers[companyEntityKey(scope.TenantID, scope.CompanyID, customerID)]
	if !ok || value.TenantID != scope.TenantID || value.CompanyID != scope.CompanyID {
		return sales.ErrNotFound
	}
	return nil
}

func (t *transaction) Product(_ context.Context, scope tenancy.Scope, productID string) (catalog.Product, error) {
	value, ok := t.state.products[companyEntityKey(scope.TenantID, scope.CompanyID, productID)]
	if !ok {
		return catalog.Product{}, sales.ErrNotFound
	}
	return value, nil
}

func (t *transaction) TaxRateBasisPoints(_ context.Context, scope tenancy.Scope, code string, at time.Time) (int64, error) {
	found := false
	var selected TaxRate
	for _, value := range t.state.taxRates {
		if value.TenantID != scope.TenantID || value.CompanyID != scope.CompanyID || value.Code != code || at.Before(value.EffectiveFrom) || (value.EffectiveTo != nil && !at.Before(*value.EffectiveTo)) {
			continue
		}
		if !found || value.EffectiveFrom.After(selected.EffectiveFrom) {
			selected, found = value, true
		}
	}
	if !found {
		return 0, sales.ErrNotFound
	}
	return selected.BasisPoints, nil
}

func (t *transaction) AvailableStock(_ context.Context, scope tenancy.Scope, productID string) (int64, error) {
	var quantity int64
	for _, value := range t.state.movements {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.BranchID == scope.BranchID && value.WarehouseID == scope.WarehouseID && value.ProductID == productID {
			quantity += value.Quantity
		}
	}
	return quantity, nil
}

func (t *transaction) CreditExposure(_ context.Context, scope tenancy.Scope, customerID string) (int64, error) {
	var amount int64
	for _, value := range t.state.ledger {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.CustomerID == customerID {
			amount += value.AmountMinor
		}
	}
	return amount, nil
}

func (t *transaction) FiscalPeriodOpen(_ context.Context, scope tenancy.Scope, at time.Time) (bool, error) {
	for _, value := range t.state.periods {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && !at.Before(value.StartsAt) && at.Before(value.EndsAt) {
			return value.Open, nil
		}
	}
	return false, nil
}

func (t *transaction) SalesPostingConfig(_ context.Context, scope tenancy.Scope) (finance.SalesPostingConfig, error) {
	value, ok := t.state.posting[companyKey(scope.TenantID, scope.CompanyID)]
	if !ok {
		return finance.SalesPostingConfig{}, sales.ErrPostingConfig
	}
	value.CashAccounts = cloneStringMap(value.CashAccounts)
	return value, nil
}

func (t *transaction) Sale(_ context.Context, scope tenancy.Scope, saleID string) (sales.Sale, error) {
	value, ok := t.state.sales[companyEntityKey(scope.TenantID, scope.CompanyID, saleID)]
	if !ok || value.Scope != scope {
		return sales.Sale{}, sales.ErrNotFound
	}
	return cloneSale(value), nil
}

func (t *transaction) CreateSale(_ context.Context, value sales.Sale) error {
	key := companyEntityKey(value.Scope.TenantID, value.Scope.CompanyID, value.ID)
	if _, exists := t.state.sales[key]; exists {
		return fmt.Errorf("duplicate sale id")
	}
	t.state.sales[key] = cloneSale(value)
	return nil
}

func (t *transaction) MarkSaleReversed(_ context.Context, scope tenancy.Scope, saleID string, at time.Time) error {
	key := companyEntityKey(scope.TenantID, scope.CompanyID, saleID)
	value, ok := t.state.sales[key]
	if !ok || value.Scope != scope {
		return sales.ErrNotFound
	}
	if value.Status == sales.StatusReversed {
		return sales.ErrAlreadyReversed
	}
	value.Status, value.ReversedAt = sales.StatusReversed, &at
	t.state.sales[key] = value
	return nil
}

func (t *transaction) AppendStockMovement(_ context.Context, value inventory.Movement) error {
	for _, found := range t.state.movements {
		if found.ID == value.ID {
			return errors.New("duplicate stock movement")
		}
	}
	t.state.movements = append(t.state.movements, value)
	return nil
}

func (t *transaction) StockMovementsBySource(_ context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]inventory.Movement, error) {
	if !t.saleSourceInScope(scope, sourceType, sourceID) {
		return nil, sales.ErrNotFound
	}
	var result []inventory.Movement
	for _, value := range t.state.movements {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.BranchID == scope.BranchID && value.WarehouseID == scope.WarehouseID && value.SourceType == sourceType && value.SourceID == sourceID {
			result = append(result, value)
		}
	}
	return result, nil
}

func (t *transaction) AppendCustomerLedgerEntry(_ context.Context, value customers.LedgerEntry) error {
	for _, found := range t.state.ledger {
		if found.ID == value.ID {
			return errors.New("duplicate customer ledger entry")
		}
	}
	t.state.ledger = append(t.state.ledger, value)
	return nil
}

func (t *transaction) CustomerLedgerBySource(_ context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]customers.LedgerEntry, error) {
	if !t.saleSourceInScope(scope, sourceType, sourceID) {
		return nil, sales.ErrNotFound
	}
	var result []customers.LedgerEntry
	for _, value := range t.state.ledger {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.SourceType == sourceType && value.SourceID == sourceID {
			result = append(result, value)
		}
	}
	return result, nil
}

func (t *transaction) CreatePayment(_ context.Context, value sales.Payment) error {
	for _, found := range t.state.payments {
		if found.ID == value.ID {
			return errors.New("duplicate payment")
		}
	}
	t.state.payments = append(t.state.payments, value)
	return nil
}

func (t *transaction) PaymentsBySale(_ context.Context, scope tenancy.Scope, saleID string) ([]sales.Payment, error) {
	if !t.saleSourceInScope(scope, string(sales.RecordSale), saleID) {
		return nil, sales.ErrNotFound
	}
	var result []sales.Payment
	for _, value := range t.state.payments {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.SaleID == saleID {
			result = append(result, value)
		}
	}
	return result, nil
}

func (t *transaction) CreateJournal(_ context.Context, value finance.Journal) error {
	if err := value.Validate(); err != nil {
		return err
	}
	for _, found := range t.state.journals {
		if found.ID == value.ID || (found.TenantID == value.TenantID && found.CompanyID == value.CompanyID && found.SourceType == value.SourceType && found.SourceID == value.SourceID) {
			return errors.New("duplicate journal")
		}
	}
	t.state.journals = append(t.state.journals, cloneJournal(value))
	return nil
}

func (t *transaction) JournalBySource(_ context.Context, scope tenancy.Scope, sourceType, sourceID string) (finance.Journal, error) {
	if !t.saleSourceInScope(scope, sourceType, sourceID) {
		return finance.Journal{}, sales.ErrNotFound
	}
	for _, value := range t.state.journals {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.SourceType == sourceType && value.SourceID == sourceID {
			return cloneJournal(value), nil
		}
	}
	return finance.Journal{}, sales.ErrNotFound
}

func (t *transaction) saleSourceInScope(scope tenancy.Scope, sourceType, sourceID string) bool {
	if sourceType != string(sales.RecordSale) {
		return false
	}
	value, ok := t.state.sales[companyEntityKey(scope.TenantID, scope.CompanyID, sourceID)]
	return ok && value.RecordType == sales.RecordSale && value.Scope == scope
}

func (t *transaction) AppendAuditEvent(_ context.Context, value audit.Event) error {
	for _, found := range t.state.audits {
		if found.ID == value.ID {
			return errors.New("duplicate audit event")
		}
	}
	t.state.audits = append(t.state.audits, value)
	return nil
}

func (t *transaction) AppendOutboxEvent(_ context.Context, value outbox.Event) error {
	for _, found := range t.state.outbox {
		if found.ID == value.ID {
			return errors.New("duplicate outbox event")
		}
	}
	t.state.outbox = append(t.state.outbox, value)
	return nil
}

func companyKey(tenantID, companyID string) string { return tenantID + "\x00" + companyID }
func companyEntityKey(tenantID, companyID, entityID string) string {
	return companyKey(tenantID, companyID) + "\x00" + entityID
}
func idempotencyKey(scope tenancy.Scope, operation, key string) string {
	return companyEntityKey(scope.TenantID, scope.CompanyID, operation) + "\x00" + key
}
func permissionKey(scope tenancy.Scope, actorID, permission string) string {
	return companyEntityKey(scope.TenantID, scope.CompanyID, actorID) + "\x00" + scope.BranchID + "\x00" + scope.WarehouseID + "\x00" + permission
}
func scopeKey(scope tenancy.Scope) string {
	return companyEntityKey(scope.TenantID, scope.CompanyID, scope.BranchID) + "\x00" + scope.WarehouseID
}
func deviceKey(tenantID, deviceID string) string { return tenantID + "\x00" + deviceID }
func allocationKey(scope tenancy.Scope, deviceID, productID string) string {
	return scopeKey(scope) + "\x00" + deviceID + "\x00" + productID
}

func cloneState(source *state) *state {
	result := newState()
	for key, value := range source.permissions {
		result.permissions[key] = value
	}
	for key, value := range source.customers {
		result.customers[key] = value
	}
	for key, value := range source.products {
		result.products[key] = value
	}
	result.taxRates = append([]TaxRate(nil), source.taxRates...)
	result.periods = append([]FiscalPeriod(nil), source.periods...)
	for key, value := range source.posting {
		value.CashAccounts = cloneStringMap(value.CashAccounts)
		result.posting[key] = value
	}
	for key, value := range source.sales {
		result.sales[key] = cloneSale(value)
	}
	result.movements = append([]inventory.Movement(nil), source.movements...)
	result.ledger = append([]customers.LedgerEntry(nil), source.ledger...)
	result.payments = append([]sales.Payment(nil), source.payments...)
	for _, value := range source.journals {
		result.journals = append(result.journals, cloneJournal(value))
	}
	result.audits = append([]audit.Event(nil), source.audits...)
	result.outbox = append([]outbox.Event(nil), source.outbox...)
	for key, value := range source.idempotencies {
		result.idempotencies[key] = value
	}
	for key, value := range source.contexts {
		value.Permissions = append([]string(nil), value.Permissions...)
		result.contexts[key] = value
	}
	for key, value := range source.devices {
		result.devices[key] = value
	}
	result.offlineLeases = append([]devices.OfflineLease(nil), source.offlineLeases...)
	for key, value := range source.allocations {
		result.allocations[key] = value
	}
	return result
}

func cloneSale(value sales.Sale) sales.Sale {
	value.Lines = append([]sales.Line(nil), value.Lines...)
	return value
}
func cloneJournal(value finance.Journal) finance.Journal {
	value.Entries = append([]finance.JournalEntry(nil), value.Entries...)
	return value
}
func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
