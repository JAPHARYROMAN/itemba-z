package sales

import "github.com/itemba-z/itemba-z/services/core-api/internal/platform/wire"

// MaxWireSafeInteger is the largest integer JSON clients implemented with a
// JavaScript Number can represent exactly. PostgreSQL remains bigint-backed,
// but no public API value may cross this boundary in this milestone.
const MaxWireSafeInteger int64 = wire.MaxSafeInteger

func IsWireSafeInteger(value int64) bool {
	return wire.IsSafeInteger(value)
}

func ValidateWireSafeSale(value Sale) error {
	numbers := []int64{
		value.SubtotalMinor, value.TaxMinor, value.TotalMinor,
		value.MasterDataVersion, value.PriceVersion,
	}
	for _, line := range value.Lines {
		numbers = append(numbers, line.Quantity, line.UnitPriceMinor, line.SubtotalMinor, line.TaxMinor, line.TotalMinor)
	}
	for _, number := range numbers {
		if !IsWireSafeInteger(number) {
			return ErrUnsafeWireInteger
		}
	}
	return nil
}
