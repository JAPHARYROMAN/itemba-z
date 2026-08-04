package httpapi

import (
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

// Public sale DTOs deliberately exclude internal cost and COGS values. Cost
// facts remain in the domain object, journals, audit record, and outbox event.
type saleResponse struct {
	ID                   string             `json:"id"`
	Scope                tenancy.Scope      `json:"scope"`
	RecordType           sales.RecordType   `json:"record_type"`
	Kind                 sales.Kind         `json:"kind"`
	Status               sales.Status       `json:"status"`
	CustomerID           string             `json:"customer_id"`
	Currency             string             `json:"currency"`
	SubtotalMinor        int64              `json:"subtotal_minor"`
	TaxMinor             int64              `json:"tax_minor"`
	TotalMinor           int64              `json:"total_minor"`
	PaymentMethod        string             `json:"payment_method,omitempty"`
	DeviceID             string             `json:"device_id,omitempty"`
	ClientTransactionID  string             `json:"client_transaction_id,omitempty"`
	ClientTimestamp      *time.Time         `json:"client_timestamp,omitempty"`
	AppVersion           string             `json:"app_version,omitempty"`
	MasterDataVersion    int64              `json:"master_data_version,omitempty"`
	PriceVersion         int64              `json:"price_version,omitempty"`
	CatalogSnapshotToken string             `json:"catalog_snapshot_token,omitempty"`
	Offline              bool               `json:"offline,omitempty"`
	ReceiptReference     string             `json:"receipt_reference"`
	FiscalStatus         sales.FiscalStatus `json:"fiscal_status"`
	ReversalOf           string             `json:"reversal_of,omitempty"`
	ReversalReason       string             `json:"reversal_reason,omitempty"`
	CreatedBy            string             `json:"created_by"`
	CorrelationID        string             `json:"correlation_id"`
	CreatedAt            time.Time          `json:"created_at"`
	ReversedAt           *time.Time         `json:"reversed_at,omitempty"`
	Lines                []saleLineResponse `json:"lines"`
}

type saleLineResponse struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	SubtotalMinor  int64  `json:"subtotal_minor"`
	TaxMinor       int64  `json:"tax_minor"`
	TotalMinor     int64  `json:"total_minor"`
}

type salePageResponse struct {
	Items      []saleResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

type mobileSyncResponse struct {
	ClientTransactionID string             `json:"client_transaction_id"`
	State               string             `json:"state"`
	ReceiptReference    string             `json:"receipt_reference"`
	FiscalStatus        sales.FiscalStatus `json:"fiscal_status"`
	IdempotentReplay    bool               `json:"idempotent_replay"`
	Sale                saleResponse       `json:"sale"`
}

func presentSale(value sales.Sale) saleResponse {
	lines := make([]saleLineResponse, 0, len(value.Lines))
	for _, line := range value.Lines {
		lines = append(lines, saleLineResponse{
			ID: line.ID, ProductID: line.ProductID, Quantity: line.Quantity,
			UnitPriceMinor: line.UnitPriceMinor, SubtotalMinor: line.SubtotalMinor,
			TaxMinor: line.TaxMinor, TotalMinor: line.TotalMinor,
		})
	}
	return saleResponse{
		ID: value.ID, Scope: value.Scope, RecordType: value.RecordType, Kind: value.Kind, Status: value.Status,
		CustomerID: value.CustomerID, Currency: value.Currency, SubtotalMinor: value.SubtotalMinor,
		TaxMinor: value.TaxMinor, TotalMinor: value.TotalMinor, PaymentMethod: value.PaymentMethod,
		DeviceID: value.DeviceID, ClientTransactionID: value.ClientTransactionID,
		ClientTimestamp: value.ClientTimestamp, AppVersion: value.AppVersion,
		MasterDataVersion: value.MasterDataVersion, PriceVersion: value.PriceVersion,
		CatalogSnapshotToken: value.CatalogSnapshotToken, Offline: value.Offline,
		ReceiptReference: value.ReceiptReference, FiscalStatus: value.FiscalStatus,
		ReversalOf: value.ReversalOf, ReversalReason: value.ReversalReason,
		CreatedBy: value.CreatedBy, CorrelationID: value.CorrelationID, CreatedAt: value.CreatedAt,
		ReversedAt: value.ReversedAt, Lines: lines,
	}
}

func presentSalePage(value readmodel.SalePage) salePageResponse {
	items := make([]saleResponse, 0, len(value.Items))
	for _, sale := range value.Items {
		items = append(items, presentSale(sale))
	}
	return salePageResponse{Items: items, NextCursor: value.NextCursor}
}

func presentMobileSync(value mobile.SyncResult) mobileSyncResponse {
	return mobileSyncResponse{
		ClientTransactionID: value.ClientTransactionID, State: value.State,
		ReceiptReference: value.ReceiptReference, FiscalStatus: value.FiscalStatus,
		IdempotentReplay: value.IdempotentReplay, Sale: presentSale(value.Sale),
	}
}
