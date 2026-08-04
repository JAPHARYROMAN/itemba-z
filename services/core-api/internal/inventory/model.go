// Package inventory owns the append-only stock ledger.
package inventory

import "time"

// Movement.Quantity is signed: receipts are positive and issues are negative.
type Movement struct {
	ID          string
	TenantID    string
	CompanyID   string
	BranchID    string
	WarehouseID string
	ProductID   string
	SourceType  string
	SourceID    string
	Quantity    int64
	OccurredAt  time.Time
}
