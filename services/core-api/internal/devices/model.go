// Package devices owns registered mobile devices and their server-side policy.
package devices

import (
	"errors"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusSuspended Status = "SUSPENDED"
	StatusRevoked   Status = "REVOKED"
	// OfflineSalesLeaseDuration bounds exposure even when no tax transition is
	// currently scheduled. It is an operational safety lease, not tax policy.
	OfflineSalesLeaseDuration = 4 * time.Hour
)

type Device struct {
	ID         string        `json:"device_id"`
	Status     Status        `json:"status"`
	ActorID    string        `json:"actor_id"`
	Scope      tenancy.Scope `json:"scope"`
	Name       string        `json:"device_name"`
	AppVersion string        `json:"app_version"`
	// MasterDataVersion and PriceVersion are the versions the device has
	// explicitly acknowledged as durably installed.
	MasterDataVersion            int64             `json:"master_data_version"`
	PriceVersion                 int64             `json:"price_version"`
	AvailableMasterDataVersion   int64             `json:"available_master_data_version"`
	AvailablePriceVersion        int64             `json:"available_price_version"`
	OfflineSalesValidFrom        time.Time         `json:"-"`
	OfflineSalesValidUntil       time.Time         `json:"offline_sales_valid_until"`
	TimeZone                     string            `json:"timezone"`
	OfflineEnabled               bool              `json:"offline_enabled"`
	OfflineTransactionLimitMinor int64             `json:"transaction_value_limit_minor"`
	OfflineDailyLimitMinor       int64             `json:"daily_value_limit_minor"`
	OfflineRemainingDailyMinor   int64             `json:"remaining_daily_value_minor"`
	EnrolledAt                   time.Time         `json:"enrolled_at"`
	LastSeenAt                   time.Time         `json:"last_seen_at"`
	StockAllocations             []StockAllocation `json:"stock_allocations"`
}

type InstallAcknowledgement struct {
	MasterDataVersion int64
	PriceVersion      int64
}

// OfflineLease records the exact governed cache and application build that a
// device was allowed to use while disconnected. Leases are append-only so a
// later cache acknowledgement cannot strand transactions created under an
// earlier, still-valid lease.
type OfflineLease struct {
	Scope             tenancy.Scope
	DeviceID          string
	AppVersion        string
	MasterDataVersion int64
	PriceVersion      int64
	ValidFrom         time.Time
	ValidUntil        time.Time
}

func ValidateWireSafe(value Device) error {
	numbers := []int64{
		value.MasterDataVersion, value.PriceVersion,
		value.AvailableMasterDataVersion, value.AvailablePriceVersion,
		value.OfflineTransactionLimitMinor, value.OfflineDailyLimitMinor,
		value.OfflineRemainingDailyMinor,
	}
	for _, allocation := range value.StockAllocations {
		numbers = append(numbers, allocation.AllocatedQuantity, allocation.RemainingQuantity)
	}
	for _, number := range numbers {
		if !wire.IsSafeInteger(number) {
			return wire.ErrUnsafeInteger
		}
	}
	return nil
}

type StockAllocation struct {
	ProductID         string `json:"product_id"`
	AllocatedQuantity int64  `json:"allocated_quantity"`
	RemainingQuantity int64  `json:"remaining_quantity"`
}

var (
	ErrNotEnrolled             = errors.New("mobile device is not enrolled")
	ErrScopeMismatch           = errors.New("mobile device is bound to a different actor or organizational scope")
	ErrNotActive               = errors.New("mobile device is not active")
	ErrOfflineDisabled         = errors.New("offline sales are disabled for this device")
	ErrMobileCreditUnsupported = errors.New("mobile credit sales are unavailable until authoritative credit approval is implemented")
	ErrAllocationExceeded      = errors.New("offline stock allocation would be exceeded")
	ErrOfflineLimit            = errors.New("offline transaction or daily value limit would be exceeded")
	ErrStaleMasterData         = errors.New("mobile sale uses stale app, master data, or price versions")
	ErrInvalidTimeZone         = errors.New("legal company business timezone is invalid")
	ErrOfflineLeaseExpired     = errors.New("offline sale was created outside the server-issued cache validity lease")
)
