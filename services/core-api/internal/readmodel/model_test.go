package readmodel

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

func TestMalformedButDecodableCursorIsRejected(t *testing.T) {
	cursor := base64.RawURLEncoding.EncodeToString([]byte("not-a-uuid"))
	if _, err := listOptions("", cursor, "", nil, 50); !errors.Is(err, sales.ErrInvalidCommand) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

type unsafeReadRepository struct{}

func (unsafeReadRepository) WorkingContext(context.Context, tenancy.Scope, string) (WorkingContext, error) {
	return WorkingContext{MasterDataVersion: 1, PriceVersion: 1, CatalogSnapshotToken: "00000000-0000-4000-8000-000000000001"}, nil
}
func (unsafeReadRepository) ListCustomers(context.Context, tenancy.Scope, string, ListOptions) (CatalogSnapshot, []CustomerSummary, error) {
	return CatalogSnapshot{Token: "00000000-0000-4000-8000-000000000001", MasterDataVersion: 1, PriceVersion: 1}, nil, nil
}
func (unsafeReadRepository) ListProducts(context.Context, tenancy.Scope, string, ListOptions) (CatalogSnapshot, []ProductSummary, error) {
	return CatalogSnapshot{Token: "00000000-0000-4000-8000-000000000001", MasterDataVersion: 1, PriceVersion: 1}, []ProductSummary{{ID: "product", UnitPriceMinor: sales.MaxWireSafeInteger + 1, PriceVersion: 1, MasterDataVersion: 1}}, nil
}
func (unsafeReadRepository) ListSales(context.Context, tenancy.Scope, string, ListOptions) ([]sales.Sale, error) {
	return []sales.Sale{{ID: "sale", TotalMinor: sales.MaxWireSafeInteger + 1}}, nil
}
func (unsafeReadRepository) ListAuditEvents(context.Context, tenancy.Scope, string, string, string) ([]AuditRecord, error) {
	return nil, nil
}

func TestPublicReadModelsRejectUnsafeExistingDatabaseIntegers(t *testing.T) {
	service, err := NewService(unsafeReadRepository{})
	if err != nil {
		t.Fatal(err)
	}
	scope := tenancy.Scope{TenantID: "tenant", CompanyID: "company", BranchID: "branch", WarehouseID: "warehouse"}
	if _, err := service.Products(context.Background(), scope, "actor", "", "", "", 50); !errors.Is(err, sales.ErrUnsafeWireInteger) {
		t.Fatalf("unsafe product integer error=%v", err)
	}
	if _, err := service.Sales(context.Background(), scope, "actor", "", 50); !errors.Is(err, sales.ErrUnsafeWireInteger) {
		t.Fatalf("unsafe sale integer error=%v", err)
	}
}
