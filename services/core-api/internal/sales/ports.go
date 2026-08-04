package sales

import (
	"context"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/audit"
	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/devices"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/outbox"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

// Repository provides a real atomic boundary around all modules participating
// in a sale. Implementations must roll back every write when fn returns an error.
type Repository interface {
	WithTransaction(ctx context.Context, fn func(Transaction) error) error
}

type Transaction interface {
	Authorize(ctx context.Context, scope tenancy.Scope, actorID, permission string) (bool, error)
	// ClaimIdempotency returns acquired=false and the original result ID when
	// this exact request has already committed. A reused key with a different
	// request hash returns ErrIdempotencyConflict.
	ClaimIdempotency(ctx context.Context, scope tenancy.Scope, operation, key, requestHash string) (acquired bool, resultID string, err error)
	CompleteIdempotency(ctx context.Context, scope tenancy.Scope, operation, key, resultID string) error

	Customer(ctx context.Context, scope tenancy.Scope, customerID string) (customers.Account, error)
	OfflineCatalogCustomer(ctx context.Context, scope tenancy.Scope, catalogSnapshotToken, customerID string) (customers.Account, error)
	LockCustomerCredit(ctx context.Context, scope tenancy.Scope, customerID string) error
	Product(ctx context.Context, scope tenancy.Scope, productID string) (catalog.Product, error)
	OfflineCatalogProduct(ctx context.Context, scope tenancy.Scope, catalogSnapshotToken, productID string, at time.Time) (catalog.Product, int64, error)
	TaxRateBasisPoints(ctx context.Context, scope tenancy.Scope, taxCode string, at time.Time) (int64, error)
	AvailableStock(ctx context.Context, scope tenancy.Scope, productID string) (int64, error)
	CreditExposure(ctx context.Context, scope tenancy.Scope, customerID string) (int64, error)
	FiscalPeriodOpen(ctx context.Context, scope tenancy.Scope, at time.Time) (bool, error)
	FiscalPeriod(ctx context.Context, scope tenancy.Scope, at time.Time) (FiscalPeriod, error)
	OfflinePostingPolicy(ctx context.Context, scope tenancy.Scope, at time.Time) (OfflinePostingPolicy, error)
	SalesPostingConfig(ctx context.Context, scope tenancy.Scope) (finance.SalesPostingConfig, error)
	MobileDevice(ctx context.Context, scope tenancy.Scope, actorID, deviceID string) (devices.Device, error)
	OfflineLeaseValid(ctx context.Context, lease devices.OfflineLease, clientTimestamp time.Time) (bool, error)
	OfflineAllocation(ctx context.Context, scope tenancy.Scope, deviceID, productID string) (int64, error)
	OfflineSalesTotal(ctx context.Context, scope tenancy.Scope, deviceID string, startsAt, endsAt time.Time) (int64, error)

	Sale(ctx context.Context, scope tenancy.Scope, saleID string) (Sale, error)
	CreateSale(ctx context.Context, sale Sale) error
	MarkSaleReversed(ctx context.Context, scope tenancy.Scope, saleID string, reversedAt time.Time) error
	AppendStockMovement(ctx context.Context, movement inventory.Movement) error
	StockMovementsBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]inventory.Movement, error)
	AppendCustomerLedgerEntry(ctx context.Context, entry customers.LedgerEntry) error
	CustomerLedgerBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) ([]customers.LedgerEntry, error)
	CreatePayment(ctx context.Context, payment Payment) error
	PaymentsBySale(ctx context.Context, scope tenancy.Scope, saleID string) ([]Payment, error)
	CreateJournal(ctx context.Context, journal finance.Journal) error
	JournalBySource(ctx context.Context, scope tenancy.Scope, sourceType, sourceID string) (finance.Journal, error)
	AppendAuditEvent(ctx context.Context, event audit.Event) error
	AppendOutboxEvent(ctx context.Context, event outbox.Event) error
}
