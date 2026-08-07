package commercial_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/commercial"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestGovernedMastersAndCompetitiveSourcingAward(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	store := memory.New()
	svc, e := commercial.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if e != nil {
		t.Fatal(e)
	}
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	maker, checker := "maker", "checker"
	for _, actor := range []string{maker, checker} {
		for _, p := range []string{"masterdata.read", "masterdata.suppliers.manage", "masterdata.products.manage", "masterdata.approve", "purchases.sourcing.read", "purchases.sourcing.manage", "purchases.sourcing.approve", "operations.read"} {
			store.SeedPermission(scope, actor, p)
		}
	}
	activate := func(v commercial.Revision) commercial.Revision {
		var err error
		v, err = svc.TransitionRevision(ctx, commercial.RevisionTransitionCommand{Scope: scope, RevisionID: v.ID, Status: commercial.Submitted, Reason: "Submitted for independent review", ActorID: maker, IdempotencyKey: "submit-revision-" + v.ID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = svc.TransitionRevision(ctx, commercial.RevisionTransitionCommand{Scope: scope, RevisionID: v.ID, Status: commercial.Active, Reason: "Maker cannot approve this record", ActorID: maker, IdempotencyKey: "self-approval---" + v.ID}); !errors.Is(err, commercial.ErrSeparationOfDuties) {
			t.Fatalf("expected separation of duties, got %v", err)
		}
		v, err = svc.TransitionRevision(ctx, commercial.RevisionTransitionCommand{Scope: scope, RevisionID: v.ID, Status: commercial.Active, Reason: "Independently reviewed and activated", ActorID: checker, IdempotencyKey: "activate-revision" + v.ID})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	supplier, err := svc.CreateRevision(ctx, commercial.RevisionCommand{Scope: scope, EntityType: commercial.SupplierEntity, Supplier: &commercial.SupplierData{Code: "sup-100", Name: "Mwanza Wholesale Limited", TaxID: "tin-100", Email: "BUY@EXAMPLE.COM", Phone: "+255700000000", PaymentTermsDays: 30, Active: true}, Reason: "Create approved sourcing supplier", ActorID: maker, IdempotencyKey: "supplier-revision-0001"})
	if err != nil {
		t.Fatal(err)
	}
	supplier = activate(supplier)
	product, err := svc.CreateRevision(ctx, commercial.RevisionCommand{Scope: scope, EntityType: commercial.ProductEntity, Product: &commercial.ProductData{SKU: "item-100", Name: "Core retail item", BaseUnitCode: "EA", Currency: "TZS", ListPriceMinor: 25000, StandardCostMinor: 12000, TaxCode: "VAT18", RevenueAccountID: "4000", COGSAccountID: "5000", InventoryAccountID: "1200", Active: true}, Reason: "Create governed purchasing product", ActorID: maker, IdempotencyKey: "product-revision-00001"})
	if err != nil {
		t.Fatal(err)
	}
	product = activate(product)
	rfq, err := svc.CreateRFQ(ctx, commercial.RFQCommand{Scope: scope, Currency: "TZS", ResponseDueAt: at.Add(72 * time.Hour), Reason: "Source replenishment through comparison", ActorID: maker, IdempotencyKey: "create-rfq-000000001", Lines: []commercial.RFQCommandLine{{ProductID: product.EntityID, Quantity: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	rfq, err = svc.TransitionRFQ(ctx, commercial.RFQTransitionCommand{Scope: scope, RFQID: rfq.ID, Status: commercial.RFQSubmitted, Reason: "Submit request for sourcing approval", ActorID: maker, IdempotencyKey: "submit-rfq-00000001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.TransitionRFQ(ctx, commercial.RFQTransitionCommand{Scope: scope, RFQID: rfq.ID, Status: commercial.RFQApproved, Reason: "Independent sourcing approval complete", ActorID: checker, IdempotencyKey: "approve-rfq-0000001"}); err != nil {
		t.Fatal(err)
	}
	quote, err := svc.CreateQuote(ctx, commercial.QuoteCommand{Scope: scope, RFQID: rfq.ID, SupplierID: supplier.EntityID, Reference: "q-100", Currency: "TZS", DeliveryDays: 3, PaymentTermsDays: 30, ValidUntil: at.Add(10 * 24 * time.Hour), Reason: "Supplier response received and recorded", ActorID: maker, IdempotencyKey: "create-quote-0000001", Lines: []commercial.QuoteCommandLine{{ProductID: product.EntityID, Quantity: 10, UnitPriceMinor: 11500}}})
	if err != nil {
		t.Fatal(err)
	}
	quote, err = svc.SubmitQuote(ctx, commercial.QuoteSubmitCommand{Scope: scope, QuoteID: quote.ID, Reason: "Submit supplier response for comparison", ActorID: maker, IdempotencyKey: "submit-quote-000001"})
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := svc.Comparison(ctx, scope, checker, rfq.ID)
	if err != nil || len(comparison.Quotes) != 1 || !comparison.Quotes[0].Comparable {
		t.Fatalf("comparison %+v %v", comparison, err)
	}
	award, err := svc.Award(ctx, commercial.AwardCommand{Scope: scope, RFQID: rfq.ID, QuoteID: quote.ID, Reason: "Best evaluated compliant supplier selected", ActorID: checker, IdempotencyKey: "award-rfq-000000001"})
	if err != nil {
		t.Fatal(err)
	}
	po, err := store.OperationDocument(ctx, scope, checker, award.PurchaseOrderID)
	if err != nil || po.Type != operations.PurchaseOrder || po.PartyID != supplier.EntityID || po.TotalMinor != 115000 {
		t.Fatalf("purchase order %+v %v", po, err)
	}
	if _, err = svc.Award(ctx, commercial.AwardCommand{Scope: scope, RFQID: rfq.ID, QuoteID: quote.ID, Reason: "Attempt a second sourcing award record", ActorID: checker, IdempotencyKey: "award-rfq-000000002"}); !errors.Is(err, commercial.ErrSourceMismatch) && !errors.Is(err, commercial.ErrAwardExists) {
		t.Fatalf("expected duplicate award rejection, got %v", err)
	}
}
