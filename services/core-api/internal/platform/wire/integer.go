package wire

import "errors"

// MaxSafeInteger is the largest integer JSON clients implemented with a
// JavaScript Number can represent exactly.
const MaxSafeInteger int64 = 9_007_199_254_740_991

var ErrUnsafeInteger = errors.New("a money, quantity, count, or version value exceeds the exact JSON integer range")

func IsSafeInteger(value int64) bool {
	return value >= -MaxSafeInteger && value <= MaxSafeInteger
}
