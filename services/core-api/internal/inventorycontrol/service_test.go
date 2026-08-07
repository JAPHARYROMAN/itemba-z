package inventorycontrol_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/catalog"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventorycontrol"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestGovernedPolicyAndReplenishmentRecommendation(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	store := memory.New()
	svc, err := inventorycontrol.NewService(store, identity.UUIDGenerator{}, clock.Fixed{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	productID := "10000000-0000-4000-8000-000000000001"
	store.SeedProduct(catalog.Product{ID: productID, TenantID: scope.TenantID, CompanyID: scope.CompanyID, SKU: "LOT-100", Name: "Governed item", BaseUnitCode: "EA", Active: true, Currency: "TZS", StandardCostMinor: 1000})
	for _, actor := range []string{"maker", "checker"} {
		for _, permission := range []string{"inventory.planning.read", "inventory.planning.manage", "inventory.planning.approve", "inventory.lots.manage"} {
			store.SeedPermission(scope, actor, permission)
		}
	}
	policy, err := svc.CreatePolicy(ctx, inventorycontrol.PolicyCommand{Scope: scope, ProductID: productID, CostMethod: inventorycontrol.MovingAverage, LotControlled: true, ReorderPoint: 20, ReorderQuantity: 30, MaximumStock: 50, SafetyStock: 10, LeadTimeDays: 7, Reason: "Govern expiry and replenishment", ActorID: "maker", IdempotencyKey: "create-inventory-policy-0001"})
	if err != nil {
		t.Fatal(err)
	}
	policy, err = svc.TransitionPolicy(ctx, inventorycontrol.PolicyTransitionCommand{Scope: scope, PolicyID: policy.ID, Status: inventorycontrol.PolicySubmitted, Reason: "Submit policy for independent review", ActorID: "maker", IdempotencyKey: "submit-inventory-policy-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.TransitionPolicy(ctx, inventorycontrol.PolicyTransitionCommand{Scope: scope, PolicyID: policy.ID, Status: inventorycontrol.PolicyActive, Reason: "Maker attempts own policy approval", ActorID: "maker", IdempotencyKey: "self-approve-policy-000001"}); !errors.Is(err, inventorycontrol.ErrSeparationOfDuties) {
		t.Fatalf("expected separation of duties, got %v", err)
	}
	if _, err = svc.TransitionPolicy(ctx, inventorycontrol.PolicyTransitionCommand{Scope: scope, PolicyID: policy.ID, Status: inventorycontrol.PolicyActive, Reason: "Independent inventory review complete", ActorID: "checker", IdempotencyKey: "activate-inventory-policy-01"}); err != nil {
		t.Fatal(err)
	}
	workspace, err := svc.Workspace(ctx, scope, "checker")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Replenishments) != 1 || !workspace.Replenishments[0].ActionRequired || workspace.Replenishments[0].RecommendedQuantity != 50 {
		t.Fatalf("unexpected replenishment recommendation: %+v", workspace.Replenishments)
	}
}
