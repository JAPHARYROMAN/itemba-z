// Package catalog owns sellable product definitions. Prices and costs use the
// currency's smallest unit; floating point values are never used for money.
package catalog

type Product struct {
	ID                 string
	TenantID           string
	CompanyID          string
	SKU                string
	Name               string
	Active             bool
	Currency           string
	ListPriceMinor     int64
	StandardCostMinor  int64
	TaxCode            string
	RevenueAccountID   string
	COGSAccountID      string
	InventoryAccountID string
}
