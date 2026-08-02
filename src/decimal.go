// Package decimal is a Go port of decimal.js, an arbitrary-precision Decimal
// type for JavaScript.
//
// The port reproduces decimal.js's observable behaviour — results, rounding,
// and edge cases — rather than its API shape. Values are immutable: every
// operation returns a new Decimal and never writes through the receiver or the
// argument.
package decimal

// A Decimal is an arbitrary-precision decimal number, or one of the three
// non-finite values NaN, +Infinity and -Infinity.
//
// The representation mirrors decimal.js so that results match digit for digit:
// a coefficient held as base-1e7 limbs, a base-10 exponent, and a sign.
//
//	value = sign * 0.d[0]d[1]... * 10^(exp+1)
//
// The zero value of Decimal is not a usable number; construct values with New
// or FromString. Non-finite values carry a nil coefficient, distinguished by
// sign: signNaN for NaN, and 1 or -1 for ±Infinity. decimal.js encodes the same
// distinction as `d === null` with `s` either ±1 or NaN.
type Decimal struct {
	// coefficient holds the significant digits, most significant limb first,
	// each limb in [0, base). It is nil if and only if the value is
	// non-finite. A zero value has a single zero limb.
	coefficient []int

	// exponent is the base-10 exponent of the most significant digit, i.e.
	// the same value as decimal.js's `.e`. It is meaningless when the value
	// is non-finite.
	exponent int

	// sign is 1 or -1 for finite values and infinities, and signNaN for NaN.
	// decimal.js keeps a signed zero the same way: -0 is sign -1 with a zero
	// coefficient.
	sign int
}

const (
	// base is the limb radix of the coefficient. decimal.js uses 1e7 because
	// it is the largest power of ten whose square is exactly representable in
	// a float64, and the port keeps it so that intermediate rounding lands in
	// the same places.
	base = 1e7

	// logBase is log10(base): the number of decimal digits per limb.
	logBase = 7

	// maxSafeInteger is JavaScript's Number.MAX_SAFE_INTEGER, 2^53-1. It
	// bounds the integer arguments decimal.js accepts without losing
	// precision, and the port enforces the same bound so the same inputs are
	// rejected.
	maxSafeInteger = 9007199254740991

	// expLimit bounds the configurable exponent range, as in decimal.js.
	expLimit = 9e15

	// maxDigits bounds `precision` and the digit-count arguments to the
	// formatting methods.
	maxDigits = 1e9

	// signNaN is the sign stored for NaN. decimal.js stores the JavaScript
	// NaN itself in `.s`; Go has no such value for an int, so NaN gets its
	// own sentinel and every sign comparison must account for it.
	signNaN = 0
)

// Sign reports the sign of d: 1 for positive values including +0, -1 for
// negative values including -0, and 0 for NaN. It is decimal.js's `.s`.
func (d Decimal) Sign() int { return d.sign }

// Exponent reports the base-10 exponent of d's most significant digit, i.e.
// decimal.js's `.e`. It is meaningless for non-finite values.
func (d Decimal) Exponent() int { return d.exponent }

// Coefficient returns a copy of d's base-1e7 limbs, most significant first, or
// nil if d is not finite. It is decimal.js's `.d`, exposed because the original
// test suite asserts against the internal representation directly.
func (d Decimal) Coefficient() []int {
	if d.coefficient == nil {
		return nil
	}
	limbs := make([]int, len(d.coefficient))
	copy(limbs, d.coefficient)
	return limbs
}
