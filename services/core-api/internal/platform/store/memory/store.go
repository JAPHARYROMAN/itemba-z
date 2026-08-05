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

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
)

const testCatalogSnapshotToken = "00000000-0000-4000-8000-000000000001"

type TaxRate struct {
	TenantID      string
	CompanyID     string
	Code          string
	BasisPoints   int64
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
}

type FiscalPeriod struct {
	ID        string
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

type catalogPublication struct {
	customers map[string]customers.Account
	products  map[string]catalog.Product
	taxRates  []TaxRate
}

type state struct {
	permissions               map[string]bool
	customers                 map[string]customers.Account
	products                  map[string]catalog.Product
	taxRates                  []TaxRate
	periods                   []FiscalPeriod
	offlinePostingPolicies    []sales.OfflinePostingPolicy
	posting                   map[string]finance.SalesPostingConfig
	sales                     map[string]sales.Sale
	movements                 []inventory.Movement
	ledger                    []customers.LedgerEntry
	creditPolicies            []customers.CreditPolicy
	receivableItems           map[string]customers.ReceivableItem
	receivableAllocations     []customers.ReceivableAllocation
	payments                  []sales.Payment
	journals                  []finance.Journal
	audits                    []audit.Event
	outbox                    []outbox.Event
	idempotencies             map[string]idempotency
	contexts                  map[string]readmodel.WorkingContext
	devices                   map[string]devices.Device
	offlineLeases             []devices.OfflineLease
	allocations               map[string]int64
	publications              map[string]catalogPublication
	reconciliationCases       map[string]mobile.ReconciliationCase
	reconciliationByCommand   map[string]string
	reconciliationIdempotency map[string]string
	operationDocuments        map[string]operations.Document
	suppliers                 map[string]operations.Supplier
	collections               map[string]receivables.Collection
	reservations              []inventory.Movement
	bankAccounts              map[string]banking.Account
	bankStatements            map[string]banking.Statement
	financialDocuments        map[string]financialops.Document
	periodActions             map[string]financialops.PeriodActionRequest
	glAccounts                map[string]financialops.GLAccount
	postingMappings           map[string]financialops.PostingMapping
	reportExports             map[string]reporting.ExportArtifact
	budgets                   map[string]advancedfinance.Budget
	assets                    map[string]advancedfinance.Asset
	depreciation              map[string]advancedfinance.Depreciation
	facilities                map[string]treasury.Facility
	intercompany              map[string]groupfinance.Transaction
	employees                 map[string]people.Employee
	attendance                map[string]people.Attendance
	leaveTypes                map[string]people.LeaveType
	leaveRequests             map[string]people.LeaveRequest
	employeeLoans             map[string]people.Loan
	payrollRuns               map[string]people.PayrollRun
}

func newState() *state {
	return &state{
		permissions: make(map[string]bool),
		customers:   make(map[string]customers.Account), products: make(map[string]catalog.Product),
		posting: make(map[string]finance.SalesPostingConfig), sales: make(map[string]sales.Sale),
		idempotencies: make(map[string]idempotency),
		contexts:      make(map[string]readmodel.WorkingContext), devices: make(map[string]devices.Device),
		allocations:               make(map[string]int64),
		receivableItems:           make(map[string]customers.ReceivableItem),
		publications:              make(map[string]catalogPublication),
		reconciliationCases:       make(map[string]mobile.ReconciliationCase),
		reconciliationByCommand:   make(map[string]string),
		reconciliationIdempotency: make(map[string]string),
		operationDocuments:        make(map[string]operations.Document),
		suppliers:                 make(map[string]operations.Supplier),
		collections:               make(map[string]receivables.Collection),
		bankAccounts:              make(map[string]banking.Account),
		bankStatements:            make(map[string]banking.Statement),
		financialDocuments:        make(map[string]financialops.Document),
		periodActions:             make(map[string]financialops.PeriodActionRequest),
		glAccounts:                make(map[string]financialops.GLAccount),
		postingMappings:           make(map[string]financialops.PostingMapping),
		reportExports:             make(map[string]reporting.ExportArtifact),
		budgets:                   make(map[string]advancedfinance.Budget),
		assets:                    make(map[string]advancedfinance.Asset),
		depreciation:              make(map[string]advancedfinance.Depreciation),
		facilities:                make(map[string]treasury.Facility),
		intercompany:              make(map[string]groupfinance.Transaction),
		employees:                 make(map[string]people.Employee),
		attendance:                make(map[string]people.Attendance),
		leaveTypes:                make(map[string]people.LeaveType),
		leaveRequests:             make(map[string]people.LeaveRequest),
		employeeLoans:             make(map[string]people.Loan),
		payrollRuns:               make(map[string]people.PayrollRun),
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
	found := false
	for _, policy := range s.state.creditPolicies {
		if policy.Scope.TenantID == value.TenantID && policy.Scope.CompanyID == value.CompanyID && policy.CustomerID == value.ID {
			found = true
			break
		}
	}
	if !found {
		s.state.creditPolicies = append(s.state.creditPolicies, customers.CreditPolicy{
			ID:    "seed-policy:" + value.ID,
			Scope: tenancy.Scope{TenantID: value.TenantID, CompanyID: value.CompanyID}, CustomerID: value.ID,
			CreditEnabled: value.CreditEnabled, CreditLimitMinor: value.CreditLimitMinor,
			PaymentTermsDays: 30, MaxOverdueDays: 0, RiskStatus: customers.CreditRiskStandard,
			Reason: "Seeded customer account policy", EffectiveFrom: time.Unix(0, 0).UTC(), CreatedAt: time.Unix(0, 0).UTC(),
			ApprovedBy: "SYSTEM",
		})
	}
}

func (s *Store) SeedCreditPolicy(value customers.CreditPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.creditPolicies = append(s.state.creditPolicies, value)
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
	if value.ID == "" {
		value.ID = fmt.Sprintf("%s:%s:%s", value.TenantID, value.CompanyID, value.StartsAt.UTC().Format(time.RFC3339Nano))
	}
	s.state.periods = append(s.state.periods, value)
}

func (s *Store) SeedOfflinePostingPolicy(value sales.OfflinePostingPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.offlinePostingPolicies = append(s.state.offlinePostingPolicies, value)
}

func (s *Store) ReplaceOfflinePostingPolicies(values ...sales.OfflinePostingPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.offlinePostingPolicies = append([]sales.OfflinePostingPolicy(nil), values...)
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

func (s *Store) SeedCustomerLedger(value customers.LedgerEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.ledger = append(s.state.ledger, value)
}
func (s *Store) SeedReceivableItem(value customers.ReceivableItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.receivableItems[companyEntityKey(value.TenantID, value.CompanyID, value.ID)] = value
}

func (s *Store) SeedContext(scope tenancy.Scope, value readmodel.WorkingContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value.TenantID, value.CompanyID = scope.TenantID, scope.CompanyID
	value.BranchID, value.WarehouseID = scope.BranchID, scope.WarehouseID
	if value.CatalogSnapshotToken == "" {
		value.CatalogSnapshotToken = testCatalogSnapshotToken
	}
	s.state.contexts[scopeKey(scope)] = value
}

func (s *Store) SeedDevice(value devices.Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value.CatalogSnapshotToken == "" {
		value.CatalogSnapshotToken = testCatalogSnapshotToken
	}
	if value.AvailableCatalogSnapshotToken == "" {
		value.AvailableCatalogSnapshotToken = value.CatalogSnapshotToken
	}
	if value.AuthorizationEpoch == "" {
		value.AuthorizationEpoch = "seed:" + value.ID
	}
	if value.CatalogSnapshotToken != devices.UnacknowledgedCatalogSnapshotToken {
		captureCatalogPublication(s.state, value.Scope, value.CatalogSnapshotToken)
	}
	s.state.devices[deviceKey(value.Scope.TenantID, value.ID)] = value
	if value.OfflineSalesValidUntil.After(value.OfflineSalesValidFrom) {
		appendOfflineLease(s.state, devices.OfflineLease{
			Scope: value.Scope, DeviceID: value.ID, AppVersion: value.AppVersion,
			MasterDataVersion: value.MasterDataVersion, PriceVersion: value.PriceVersion,
			CatalogSnapshotToken: value.CatalogSnapshotToken,
			AuthorizationEpoch:   value.AuthorizationEpoch,
			ValidFrom:            value.OfflineSalesValidFrom, ValidUntil: value.OfflineSalesValidUntil,
		})
	}
}

func (s *Store) SeedOfflineAllocation(scope tenancy.Scope, deviceID, productID string, quantity int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.allocations[allocationKey(scope, deviceID, productID)] = quantity
}

type Snapshot struct {
	Sales               []sales.Sale
	Movements           []inventory.Movement
	Ledger              []customers.LedgerEntry
	Payments            []sales.Payment
	Journals            []finance.Journal
	Audits              []audit.Event
	Outbox              []outbox.Event
	OfflineLeases       []devices.OfflineLease
	ReconciliationCases []mobile.ReconciliationCase
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
	for _, value := range s.state.reconciliationCases {
		result.ReconciliationCases = append(result.ReconciliationCases, cloneReconciliationCase(value))
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

func (t *transaction) OfflineCatalogCustomer(_ context.Context, scope tenancy.Scope, catalogSnapshotToken, customerID string) (customers.Account, error) {
	publication, ok := t.state.publications[publicationKey(scope, catalogSnapshotToken)]
	if !ok {
		return customers.Account{}, sales.ErrOfflineReconciliation
	}
	value, ok := publication.customers[customerID]
	if !ok {
		return customers.Account{}, sales.ErrOfflineReconciliation
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

func (t *transaction) OfflineCatalogProduct(_ context.Context, scope tenancy.Scope, catalogSnapshotToken, productID string, at time.Time) (catalog.Product, int64, error) {
	publication, ok := t.state.publications[publicationKey(scope, catalogSnapshotToken)]
	if !ok {
		return catalog.Product{}, 0, sales.ErrOfflineReconciliation
	}
	product, ok := publication.products[productID]
	if !ok {
		return catalog.Product{}, 0, sales.ErrOfflineReconciliation
	}
	found := false
	var selected TaxRate
	for _, value := range publication.taxRates {
		if value.Code != product.TaxCode || at.Before(value.EffectiveFrom) || (value.EffectiveTo != nil && !at.Before(*value.EffectiveTo)) {
			continue
		}
		if !found || value.EffectiveFrom.After(selected.EffectiveFrom) {
			selected, found = value, true
		}
	}
	if !found {
		return catalog.Product{}, 0, sales.ErrOfflineReconciliation
	}
	return product, selected.BasisPoints, nil
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
	for _, value := range t.state.reservations {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && value.BranchID == scope.BranchID && value.WarehouseID == scope.WarehouseID && value.ProductID == productID {
			quantity -= value.Quantity
		}
	}
	return quantity, nil
}

func (t *transaction) FulfillSalesOrder(_ context.Context, scope tenancy.Scope, orderID, customerID, _ string, lines []sales.CommandLine, releaseIDs []string, _ string, _ string, _ string, at time.Time) error {
	key := operationKey(scope, orderID)
	document, ok := t.state.operationDocuments[key]
	if !ok {
		return sales.ErrNotFound
	}
	if document.Type != operations.SalesOrder || document.Status != operations.Approved || document.PartyType != operations.CustomerParty || document.PartyID != customerID || len(document.Lines) != len(lines) || len(lines) != len(releaseIDs) {
		return operations.ErrSourceMismatch
	}
	quantities := make(map[string]int64, len(document.Lines))
	for _, line := range document.Lines {
		product, ok := t.state.products[companyEntityKey(scope.TenantID, scope.CompanyID, line.ProductID)]
		if !ok || product.ListPriceMinor != line.UnitPriceMinor {
			return operations.ErrSourceMismatch
		}
		quantities[line.ProductID] = line.Quantity
	}
	for _, line := range lines {
		if quantities[line.ProductID] != line.Quantity {
			return operations.ErrSourceMismatch
		}
	}
	for index, line := range lines {
		t.state.reservations = append(t.state.reservations, inventory.Movement{ID: releaseIDs[index], TenantID: scope.TenantID, CompanyID: scope.CompanyID, BranchID: scope.BranchID, WarehouseID: scope.WarehouseID, ProductID: line.ProductID, SourceType: "SALES_ORDER_FULFILMENT", SourceID: orderID, Quantity: -line.Quantity, OccurredAt: at})
	}
	document.Status, document.PostedAt = operations.Closed, &at
	t.state.operationDocuments[key] = document
	return nil
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

func (t *transaction) FiscalPeriod(_ context.Context, scope tenancy.Scope, at time.Time) (sales.FiscalPeriod, error) {
	for _, value := range t.state.periods {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID && !at.Before(value.StartsAt) && at.Before(value.EndsAt) {
			return sales.FiscalPeriod{ID: value.ID, StartsAt: value.StartsAt, EndsAt: value.EndsAt, Open: value.Open}, nil
		}
	}
	return sales.FiscalPeriod{}, nil
}

func (t *transaction) OfflinePostingPolicy(_ context.Context, scope tenancy.Scope, at time.Time) (sales.OfflinePostingPolicy, error) {
	var selected sales.OfflinePostingPolicy
	found := false
	for _, value := range t.state.offlinePostingPolicies {
		if value.TenantID != scope.TenantID || value.CompanyID != scope.CompanyID || at.Before(value.EffectiveFrom) || (value.EffectiveTo != nil && !at.Before(*value.EffectiveTo)) {
			continue
		}
		if !found || value.EffectiveFrom.After(selected.EffectiveFrom) {
			selected, found = value, true
		}
	}
	if !found {
		return sales.OfflinePostingPolicy{}, sales.ErrPostingConfig
	}
	return selected, nil
}

func (t *transaction) SalesPostingConfig(_ context.Context, scope tenancy.Scope) (finance.SalesPostingConfig, error) {
	value, ok := t.state.posting[companyKey(scope.TenantID, scope.CompanyID)]
	if !ok {
		return finance.SalesPostingConfig{}, sales.ErrPostingConfig
	}
	value.CashAccounts = cloneStringMap(value.CashAccounts)
	active := make(map[financialops.MappingKey]financialops.PostingMapping)
	now := time.Now().UTC()
	for _, mapping := range t.state.postingMappings {
		if mapping.TenantID != scope.TenantID || mapping.CompanyID != scope.CompanyID || mapping.Status != financialops.GovernanceActive || mapping.EffectiveFrom.After(now) {
			continue
		}
		if current, ok := active[mapping.Key]; !ok || mapping.EffectiveFrom.After(current.EffectiveFrom) {
			active[mapping.Key] = mapping
		}
	}
	if mapping, ok := active[financialops.MapSalesReceivable]; ok {
		value.ReceivableAccountID = mapping.AccountID
	}
	if mapping, ok := active[financialops.MapSalesTaxPayable]; ok {
		value.TaxPayableAccountID = mapping.AccountID
	}
	for key, method := range map[financialops.MappingKey]string{
		financialops.MapPaymentCash: sales.PaymentCash, financialops.MapPaymentMobileMoney: sales.PaymentMobileMoney,
		financialops.MapPaymentBankCard: sales.PaymentBankCard, financialops.MapPaymentBankTransfer: sales.PaymentBankTransfer,
	} {
		if mapping, ok := active[key]; ok {
			value.CashAccounts[method] = mapping.AccountID
		}
	}
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
func publicationKey(scope tenancy.Scope, token string) string {
	return companyKey(scope.TenantID, scope.CompanyID) + "\x00" + token
}

func captureCatalogPublication(current *state, scope tenancy.Scope, token string) {
	key := publicationKey(scope, token)
	if _, exists := current.publications[key]; exists {
		return
	}
	publication := catalogPublication{
		customers: make(map[string]customers.Account),
		products:  make(map[string]catalog.Product),
	}
	for _, value := range current.customers {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID {
			publication.customers[value.ID] = value
		}
	}
	for _, value := range current.products {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID {
			publication.products[value.ID] = value
		}
	}
	for _, value := range current.taxRates {
		if value.TenantID == scope.TenantID && value.CompanyID == scope.CompanyID {
			publication.taxRates = append(publication.taxRates, value)
		}
	}
	current.publications[key] = publication
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
	result.offlinePostingPolicies = append([]sales.OfflinePostingPolicy(nil), source.offlinePostingPolicies...)
	for key, value := range source.posting {
		value.CashAccounts = cloneStringMap(value.CashAccounts)
		result.posting[key] = value
	}
	for key, value := range source.sales {
		result.sales[key] = cloneSale(value)
	}
	result.movements = append([]inventory.Movement(nil), source.movements...)
	result.reservations = append([]inventory.Movement(nil), source.reservations...)
	result.ledger = append([]customers.LedgerEntry(nil), source.ledger...)
	result.creditPolicies = append([]customers.CreditPolicy(nil), source.creditPolicies...)
	for key, value := range source.receivableItems {
		result.receivableItems[key] = value
	}
	result.receivableAllocations = append([]customers.ReceivableAllocation(nil), source.receivableAllocations...)
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
	for key, value := range source.publications {
		copyValue := catalogPublication{
			customers: make(map[string]customers.Account, len(value.customers)),
			products:  make(map[string]catalog.Product, len(value.products)),
			taxRates:  append([]TaxRate(nil), value.taxRates...),
		}
		for id, account := range value.customers {
			copyValue.customers[id] = account
		}
		for id, product := range value.products {
			copyValue.products[id] = product
		}
		result.publications[key] = copyValue
	}
	for key, value := range source.reconciliationCases {
		result.reconciliationCases[key] = cloneReconciliationCase(value)
	}
	for key, value := range source.reconciliationByCommand {
		result.reconciliationByCommand[key] = value
	}
	for key, value := range source.reconciliationIdempotency {
		result.reconciliationIdempotency[key] = value
	}
	for key, value := range source.operationDocuments {
		value.Lines = append([]operations.DocumentLine(nil), value.Lines...)
		result.operationDocuments[key] = value
	}
	for key, value := range source.suppliers {
		result.suppliers[key] = value
	}
	for key, value := range source.collections {
		result.collections[key] = value
	}
	for key, value := range source.bankAccounts {
		result.bankAccounts[key] = value
	}
	for key, value := range source.bankStatements {
		result.bankStatements[key] = cloneBankStatement(value)
	}
	for key, value := range source.financialDocuments {
		value.Lines = append([]finance.JournalEntry(nil), value.Lines...)
		result.financialDocuments[key] = value
	}
	for key, value := range source.periodActions {
		result.periodActions[key] = value
	}
	for key, value := range source.glAccounts {
		result.glAccounts[key] = value
	}
	for key, value := range source.postingMappings {
		result.postingMappings[key] = value
	}
	for key, value := range source.reportExports {
		result.reportExports[key] = value
	}
	for key, value := range source.budgets {
		value.Lines = append([]advancedfinance.BudgetLine(nil), value.Lines...)
		result.budgets[key] = value
	}
	for key, value := range source.assets {
		result.assets[key] = value
	}
	for key, value := range source.depreciation {
		result.depreciation[key] = value
	}
	for key, value := range source.facilities {
		value.Transactions = append([]treasury.Transaction(nil), value.Transactions...)
		result.facilities[key] = value
	}
	for key, value := range source.intercompany {
		result.intercompany[key] = value
	}
	for key, value := range source.employees {
		result.employees[key] = value
	}
	for key, value := range source.attendance {
		result.attendance[key] = value
	}
	for key, value := range source.leaveTypes {
		result.leaveTypes[key] = value
	}
	for key, value := range source.leaveRequests {
		result.leaveRequests[key] = value
	}
	for key, value := range source.employeeLoans {
		result.employeeLoans[key] = value
	}
	for key, value := range source.payrollRuns {
		value.Lines = append([]people.PayrollLine(nil), value.Lines...)
		result.payrollRuns[key] = value
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

func cloneReconciliationCase(value mobile.ReconciliationCase) mobile.ReconciliationCase {
	value.Command = append([]byte(nil), value.Command...)
	if value.Resolution != nil {
		resolution := *value.Resolution
		value.Resolution = &resolution
	}
	return value
}

func cloneBankStatement(value banking.Statement) banking.Statement {
	value.Lines = append([]banking.StatementLine(nil), value.Lines...)
	for index := range value.Lines {
		value.Lines[index].Candidates = append([]banking.Candidate(nil), value.Lines[index].Candidates...)
		if value.Lines[index].Match != nil {
			match := *value.Lines[index].Match
			value.Lines[index].Match = &match
		}
	}
	return value
}
