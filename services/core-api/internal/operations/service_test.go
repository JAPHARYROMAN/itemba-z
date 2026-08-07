package operations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/customers"
	"github.com/itemba-z/itemba-z/services/core-api/internal/finance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestSalesOrderLifecycleEnforcesSeparationOfDuties(t *testing.T) {
	store := memory.New()
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	maker := "00000000-0000-4000-8000-000000000005"
	approver := "00000000-0000-4000-8000-000000000006"
	for _, actor := range []string{maker, approver} {
		store.SeedPermission(scope, actor, "sales.orders.manage")
		store.SeedPermission(scope, actor, "operations.read")
	}
	store.SeedStock(inventory.Movement{ID: "opening", TenantID: scope.TenantID, CompanyID: scope.CompanyID, BranchID: scope.BranchID, WarehouseID: scope.WarehouseID, ProductID: "00000000-0000-4000-8000-000000000008", Quantity: 10})
	service, err := operations.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	document, err := service.Create(context.Background(), operations.CreateCommand{Scope: scope, Type: operations.SalesOrder, PartyType: operations.CustomerParty, PartyID: "00000000-0000-4000-8000-000000000007", Currency: "TZS", Reason: "Approved customer order", Lines: []operations.CommandLine{{ProductID: "00000000-0000-4000-8000-000000000008", Quantity: 2, UnitPriceMinor: 1000}}, ActorID: maker, IdempotencyKey: "create-sales-order-0001"})
	if err != nil {
		t.Fatal(err)
	}
	document, err = service.Transition(context.Background(), operations.TransitionCommand{Scope: scope, DocumentID: document.ID, ToStatus: operations.Submitted, Reason: "Submitted for approval", ActorID: maker, IdempotencyKey: "submit-sales-order-001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Transition(context.Background(), operations.TransitionCommand{Scope: scope, DocumentID: document.ID, ToStatus: operations.Approved, Reason: "Attempt self approval", ActorID: maker, IdempotencyKey: "approve-sales-order-01"})
	if !errors.Is(err, operations.ErrSeparationOfDuties) {
		t.Fatalf("expected separation error, got %v", err)
	}
	document, err = service.Transition(context.Background(), operations.TransitionCommand{Scope: scope, DocumentID: document.ID, ToStatus: operations.Approved, Reason: "Independent approval", ActorID: approver, IdempotencyKey: "approve-sales-order-02"})
	if err != nil {
		t.Fatal(err)
	}
	if document.Status != operations.Approved {
		t.Fatalf("status=%s", document.Status)
	}
}

func TestApprovedOrderFulfilmentClosesOrderAndPostsSale(t *testing.T) {
	store := memory.New()
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	maker := "00000000-0000-4000-8000-000000000005"
	approver := "00000000-0000-4000-8000-000000000006"
	customerID := "00000000-0000-4000-8000-000000000007"
	productID := "00000000-0000-4000-8000-000000000008"
	at := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	for _, permission := range []string{"sales.orders.manage", "operations.read", "sales.complete"} {
		store.SeedPermission(scope, maker, permission)
		store.SeedPermission(scope, approver, permission)
	}
	store.SeedCustomer(customers.Account{ID: customerID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, Name: "Amani", Active: true})
	store.SeedProduct(catalog.Product{ID: productID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SKU: "SKU", Name: "Rice", Active: true, Currency: "TZS", ListPriceMinor: 1000, StandardCostMinor: 600, TaxCode: "ZERO", RevenueAccountID: "revenue", COGSAccountID: "cogs", InventoryAccountID: "inventory"})
	store.SeedTaxRate(memory.TaxRate{TenantID: scope.TenantID, CompanyID: scope.CompanyID, Code: "ZERO", BasisPoints: 0, EffectiveFrom: at.AddDate(-1, 0, 0)})
	store.SeedFiscalPeriod(memory.FiscalPeriod{TenantID: scope.TenantID, CompanyID: scope.CompanyID, StartsAt: at.AddDate(0, -1, 0), EndsAt: at.AddDate(0, 1, 0), Open: true})
	store.SeedPostingConfig(scope.TenantID, scope.CompanyID, finance.SalesPostingConfig{ReceivableAccountID: "receivable", TaxPayableAccountID: "tax", CashAccounts: map[string]string{sales.PaymentCash: "cash"}})
	store.SeedStock(inventory.Movement{ID: "opening", TenantID: scope.TenantID, CompanyID: scope.CompanyID, BranchID: scope.BranchID, WarehouseID: scope.WarehouseID, ProductID: productID, Quantity: 10})
	operationsService, _ := operations.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	order, err := operationsService.Create(context.Background(), operations.CreateCommand{Scope: scope, Type: operations.SalesOrder, PartyType: operations.CustomerParty, PartyID: customerID, Currency: "TZS", Reason: "Customer confirmed order", Lines: []operations.CommandLine{{ProductID: productID, Quantity: 2, UnitPriceMinor: 1000}}, ActorID: maker, IdempotencyKey: "order-fulfil-create-01"})
	if err != nil {
		t.Fatal(err)
	}
	order, err = operationsService.Transition(context.Background(), operations.TransitionCommand{Scope: scope, DocumentID: order.ID, ToStatus: operations.Submitted, Reason: "Submitted for approval", ActorID: maker, IdempotencyKey: "order-fulfil-submit-01"})
	if err != nil {
		t.Fatal(err)
	}
	order, err = operationsService.Transition(context.Background(), operations.TransitionCommand{Scope: scope, DocumentID: order.ID, ToStatus: operations.Approved, Reason: "Approved independently", ActorID: approver, IdempotencyKey: "order-fulfil-approve-1"})
	if err != nil {
		t.Fatal(err)
	}
	salesService, _ := sales.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	posted, err := salesService.Complete(context.Background(), sales.CompleteCommand{Scope: scope, CustomerID: customerID, SourceDocumentID: order.ID, Kind: sales.KindCash, PaymentMethod: sales.PaymentCash, Lines: []sales.CommandLine{{ProductID: productID, Quantity: 2}}, ActorID: maker, IdempotencyKey: "order-fulfil-sale-001"})
	if err != nil {
		t.Fatal(err)
	}
	if posted.SourceDocumentID != order.ID {
		t.Fatalf("source=%s", posted.SourceDocumentID)
	}
	closed, err := operationsService.Get(context.Background(), scope, maker, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != operations.Closed {
		t.Fatalf("status=%s", closed.Status)
	}
	var stock int64
	for _, movement := range store.Snapshot().Movements {
		if movement.ProductID == productID {
			stock += movement.Quantity
		}
	}
	if stock != 8 {
		t.Fatalf("stock=%d", stock)
	}
}

func TestStockCountAllowsZeroAndRejectsClientValuation(t *testing.T) {
	store := memory.New()
	scope := tenancy.Scope{TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002", BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004"}
	actor := "00000000-0000-4000-8000-000000000005"
	store.SeedPermission(scope, actor, "inventory.counts.manage")
	service, _ := operations.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: time.Now()})
	base := operations.CreateCommand{Scope: scope, Type: operations.StockCount, PartyType: operations.NoParty, Currency: "TZS", Reason: "Full physical inventory count", Lines: []operations.CommandLine{{ProductID: "00000000-0000-4000-8000-000000000008", Quantity: 0}}, ActorID: actor, IdempotencyKey: "stock-count-zero-0001"}
	if _, err := service.Create(context.Background(), base); err != nil {
		t.Fatalf("zero count rejected: %v", err)
	}
	base.IdempotencyKey = "stock-count-price-001"
	base.Lines[0].UnitPriceMinor = 1
	if _, err := service.Create(context.Background(), base); !errors.Is(err, operations.ErrInvalidCommand) {
		t.Fatalf("expected valuation rejection, got %v", err)
	}
}
